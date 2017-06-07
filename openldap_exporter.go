package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/mjtrangoni/openldap_exporter/sources"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

var (
	scrapeDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: sources.Namespace,
			Subsystem: "exporter",
			Name:      "scrape_duration_seconds",
			Help:      "openldap_exporter: Duration of a scrape job.",
		},
		[]string{"source", "result"},
	)
)

//OpenLDAPSource is a list of all sources that the user would like to collect.
type OpenLDAPSource struct {
	sourceList map[string]sources.OpenLDAPSource
}

//Describe implements the prometheus.Describe interface
func (l OpenLDAPSource) Describe(ch chan<- *prometheus.Desc) {
	scrapeDurations.Describe(ch)
}

//Collect implements the prometheus.Collect interface
func (l OpenLDAPSource) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(l.sourceList))
	for name, c := range l.sourceList {
		go func(name string, c sources.OpenLDAPSource) {
			collectFromSource(name, c, ch)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
	scrapeDurations.Collect(ch)
}

func collectFromSource(name string, s sources.OpenLDAPSource, ch chan<- prometheus.Metric) {
	result := "success"
	begin := time.Now()
	err := s.Update(ch)
	duration := time.Since(begin)
	if err != nil {
		log.Errorf("ERROR: %q source failed after %f seconds: %s", name, duration.Seconds(), err)
		result = "error"
	} else {
		log.Debugf("OK: %q source succeeded after %f seconds: %s", name, duration.Seconds(), err)
	}
	scrapeDurations.WithLabelValues(name, result).Observe(duration.Seconds())
}

func loadSources(list []string) (map[string]sources.OpenLDAPSource, error) {
	sourceList := map[string]sources.OpenLDAPSource{}
	for _, name := range list {
		fn, ok := sources.Factories[name]
		if !ok {
			return nil, fmt.Errorf("source %q not available", name)
		}
		c, err := fn()
		if err != nil {
			return nil, err
		}
		sourceList[name] = c
	}
	return sourceList, nil
}

func init() {
	prometheus.MustRegister(version.NewCollector("openldap_exporter"))
}

func main() {
	var (
		showVersion   = flag.Bool("version", false, "Print version information.")
		listenAddress = flag.String("web.listen-address", ":9999", "Address to use to expose OpenLDAP metrics.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path to use to expose OpenLDAP metrics.")
	)
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("openldap_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting openldap_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	//expand to include more sources eventually (CLI, other?)
	enabledSources := []string{"procfs"}

	sourceList, err := loadSources(enabledSources)
	if err != nil {
		log.Fatalf("Couldn't load sources: %q", err)
	}

	log.Infof("Enabled sources:")
	for s := range sourceList {
		log.Infof(" - %s", s)
	}

	prometheus.MustRegister(OpenLDAPSource{sourceList: sourceList})
	handler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{ErrorLog: log.NewErrorLogger()})

	http.Handle(*metricsPath, prometheus.InstrumentHandler("prometheus", handler))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>OpenLDAP Exporter</title></head>
			<body>
			<h1>OpenLDAP Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	log.Infoln("Listening on", *listenAddress)
	err = http.ListenAndServe(*listenAddress, nil)
	if err != nil {
		log.Fatal(err)
	}
}
