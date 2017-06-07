package sources

import (
	"github.com/prometheus/client_golang/prometheus"
)

//Namespace defines the namespace shared by all OpenLDAP metrics.
const Namespace = "openldap"

//Factories contains the list of all sources.
var Factories = make(map[string]func() (LustreSource, error))

//OpenLDAPSource is the interface that each source implements.
type OpenLDAPSource interface {
	Update(ch chan<- prometheus.Metric) (err error)
}
