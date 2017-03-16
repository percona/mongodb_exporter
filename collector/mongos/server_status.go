package collector_mongos

import (
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	instanceUptimeSeconds = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "instance", "uptime_seconds"),
		"The value of the uptime field corresponds to the number of seconds that the mongos or mongod process has been active.",
		nil, nil)
	instanceUptimeEstimateSeconds = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "instance", "uptime_estimate_seconds"),
		"uptimeEstimate provides the uptime as calculated from MongoDB's internal course-grained time keeping system.",
		nil, nil)
	instanceLocalTime = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, "instance", "local_time"),
		"The localTime value is the current time, according to the server, in UTC specified in an ISODate format.",
	  nil, nil)
)

// ServerStatus keeps the data returned by the serverStatus() method.
type ServerStatus struct {
	Uptime         float64   `bson:"uptime"`
	UptimeEstimate float64   `bson:"uptimeEstimate"`
	LocalTime      time.Time `bson:"localTime"`

	Asserts *AssertsStats `bson:"asserts"`

	Connections *ConnectionStats `bson:"connections"`

	ExtraInfo *ExtraInfo `bson:"extra_info"`

	Network *NetworkStats `bson:"network"`

	Opcounters     *OpcountersStats     `bson:"opcounters"`
	Mem            *MemStats            `bson:"mem"`
	Metrics        *MetricsStats        `bson:"metrics"`

	Cursors *Cursors `bson:"cursors"`
}

// Export exports the server status to be consumed by prometheus.
func (status *ServerStatus) Export(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(instanceUptimeSeconds, prometheus.CounterValue, status.Uptime)
	ch <- prometheus.MustNewConstMetric(instanceUptimeEstimateSeconds, prometheus.CounterValue, status.Uptime)
	ch <- prometheus.MustNewConstMetric(instanceLocalTime, prometheus.CounterValue, float64(status.LocalTime.Unix()))

	if status.Asserts != nil {
		status.Asserts.Export(ch)
	}
	if status.Connections != nil {
		status.Connections.Export(ch)
	}
	if status.ExtraInfo != nil {
		status.ExtraInfo.Export(ch)
	}
	if status.Network != nil {
		status.Network.Export(ch)
	}
	if status.Opcounters != nil {
		status.Opcounters.Export(ch)
	}
	if status.Mem != nil {
		status.Mem.Export(ch)
	}
	if status.Metrics != nil {
		status.Metrics.Export(ch)
	}
	if status.Cursors != nil {
		status.Cursors.Export(ch)
	}
}

// Describe describes the server status for prometheus.
func (status *ServerStatus) Describe(ch chan<- *prometheus.Desc) {
	ch <- instanceUptimeSeconds
	ch <- instanceUptimeEstimateSeconds
	ch <- instanceLocalTime

	if status.Asserts != nil {
		status.Asserts.Describe(ch)
	}
	if status.Connections != nil {
		status.Connections.Describe(ch)
	}
	if status.ExtraInfo != nil {
		status.ExtraInfo.Describe(ch)
	}
	if status.Network != nil {
		status.Network.Describe(ch)
	}
	if status.Opcounters != nil {
		status.Opcounters.Describe(ch)
	}
	if status.Mem != nil {
		status.Mem.Describe(ch)
	}
	if status.Metrics != nil {
		status.Metrics.Describe(ch)
	}
	if status.Cursors != nil {
		status.Cursors.Describe(ch)
	}
}

// GetServerStatus returns the server status info.
func GetServerStatus(session *mgo.Session) *ServerStatus {
	result := &ServerStatus{}
	err := session.DB("admin").Run(bson.D{{"serverStatus", 1}, {"recordStats", 0}}, result)
	if err != nil {
		glog.Error("Failed to get server status.")
		return nil
	}

	return result
}
