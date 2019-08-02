// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongod

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	subsystem = "replset"
	myName    = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "my_name",
		Help:      "The replica state name of the current member",
	}, []string{"set", "name"})
	myState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "my_state",
		Help:      "An integer between 0 and 10 that represents the replica state of the current member",
	}, []string{"set"})
	date = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "date",
		Help:      "The value of the date field is an ISODate of the current time, according to the current server.",
	}, []string{"set"})
	term = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "term",
		Help:      "The election count for the replica set, as known to this replica set member",
	}, []string{"set"})
	numberOfMembers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "number_of_members",
		Help:      "The number of replica set mebers",
	}, []string{"set"})
	heartbeatIntervalMillis = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "heatbeat_interval_millis",
		Help:      "The frequency in milliseconds of the heartbeats",
	}, []string{"set"})
	memberHealth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_health",
		Help:      "This field conveys if the member is up (1) or down (0).",
	}, []string{"set", "name", "state"})
	memberState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_state",
		Help:      "The value of state is an integer between 0 and 10 that represents the replica state of the member.",
	}, []string{"set", "name", "state"})
	memberUptimeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(Namespace, subsystem, "member_uptime"),
		"The uptime field holds a value that reflects the number of seconds that this member has been online.",
		[]string{"set", "name", "state"},
		nil,
	)
	memberOptimeDate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_optime_date",
		Help:      "The timestamp of the last oplog entry that this member applied.",
	}, []string{"set", "name", "state"})
	memberRepLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_replication_lag",
		Help:      "The replication lag that this member has with the primary.",
	}, []string{"set", "name", "state"})
	memberOperationalLag = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_operational_lag",
		Help:      "The operationl lag - or staleness of the oplog timestamp - for this member.",
	}, []string{"set", "name", "state"})
	memberElectionDate = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_election_date",
		Help:      "The timestamp the node was elected as replica leader",
	}, []string{"set", "name", "state"})
	memberLastHeartbeat = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_last_heartbeat",
		Help:      "The lastHeartbeat value provides an ISODate formatted date and time of the transmission time of last heartbeat received from this member",
	}, []string{"set", "name", "state"})
	memberLastHeartbeatRecv = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_last_heartbeat_recv",
		Help:      "The lastHeartbeatRecv value provides an ISODate formatted date and time that the last heartbeat was received from this member",
	}, []string{"set", "name", "state"})
	memberPingMs = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_ping_ms",
		Help:      "The pingMs represents the number of milliseconds (ms) that a round-trip packet takes to travel between the remote member and the local instance.",
	}, []string{"set", "name", "state"})
	memberConfigVersion = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_config_version",
		Help:      "The configVersion value is the replica set configuration version.",
	}, []string{"set", "name", "state"})
	primaryOptimeDate        float64
	primaryLastHeartbeatRecv float64
)

// ReplSetStatus keeps the data returned by the GetReplSetStatus method
type ReplSetStatus struct {
	Set                     string    `bson:"set"`
	Date                    time.Time `bson:"date"`
	MyState                 int32     `bson:"myState"`
	Term                    *int32    `bson:"term,omitempty"`
	HeartbeatIntervalMillis *float64  `bson:"heartbeatIntervalMillis,omitempty"`
	Members                 []Member  `bson:"members"`

	Ok float64 `bson:"ok"`
}

// Member represents an array element of ReplSetStatus.Members
type Member struct {
	Name                 string              `bson:"name"`
	Self                 *bool               `bson:"self,omitempty"`
	Health               *int32              `bson:"health,omitempty"`
	State                int32               `bson:"state"`
	StateStr             string              `bson:"stateStr"`
	Uptime               float64             `bson:"uptime"`
	Optime               interface{}         `bson:"optime"`
	OptimeDate           time.Time           `bson:"optimeDate"`
	ElectionTime         primitive.Timestamp `bson:"electionTime,omitempty"`
	ElectionDate         *time.Time          `bson:"electionDate,omitempty"`
	LastHeartbeat        *time.Time          `bson:"lastHeartbeat,omitempty"`
	LastHeartbeatRecv    *time.Time          `bson:"lastHeartbeatRecv,omitempty"`
	LastHeartbeatMessage *string             `bson:"lastHeartbeatMessage,omitempty"`
	PingMs               *float64            `bson:"pingMs,omitempty"`
	SyncingTo            *string             `bson:"syncingTo,omitempty"`
	ConfigVersion        *int32              `bson:"configVersion,omitempty"`
}

// Export exports the replSetGetStatus stati to be consumed by prometheus
func (replStatus *ReplSetStatus) Export(ch chan<- prometheus.Metric) {
	myName.Reset()
	myState.Reset()
	term.Reset()
	numberOfMembers.Reset()
	heartbeatIntervalMillis.Reset()
	memberState.Reset()
	memberHealth.Reset()
	memberOptimeDate.Reset()
	memberRepLag.Reset()
	memberOperationalLag.Reset()
	memberElectionDate.Reset()
	memberLastHeartbeat.Reset()
	memberLastHeartbeatRecv.Reset()
	memberPingMs.Reset()
	memberConfigVersion.Reset()

	myState.WithLabelValues(replStatus.Set).Set(float64(replStatus.MyState))
	date.WithLabelValues(replStatus.Set).Set(float64(replStatus.Date.Unix()))

	// new in version 3.2
	if replStatus.Term != nil {
		term.WithLabelValues(replStatus.Set).Set(float64(*replStatus.Term))
	}
	numberOfMembers.WithLabelValues(replStatus.Set).Set(float64(len(replStatus.Members)))

	// new in version 3.2
	if replStatus.HeartbeatIntervalMillis != nil {
		heartbeatIntervalMillis.WithLabelValues(replStatus.Set).Set(*replStatus.HeartbeatIntervalMillis)
	}

	// Find the Optime and the LastHeartbeatRecv for the Primary.
	for _, member := range replStatus.Members {
		if member.StateStr == "PRIMARY" {
			// Needed to calcule the replication lag for secondaries.
			primaryOptimeDate = float64(member.OptimeDate.Unix())
			// Needed to calcule the operationl lag.
			if member.LastHeartbeatRecv != nil {
				primaryLastHeartbeatRecv = float64((*member.LastHeartbeatRecv).Unix())
			} else {
				primaryLastHeartbeatRecv = 0
			}
			break
		}
	}

	for _, member := range replStatus.Members {
		if member.Self != nil {
			labels := prometheus.Labels{
				"set":  replStatus.Set,
				"name": member.Name,
			}
			myName.With(labels).Set(1)
		}
		ls := prometheus.Labels{
			"set":   replStatus.Set,
			"name":  member.Name,
			"state": member.StateStr,
		}

		memberState.With(ls).Set(float64(member.State))

		// ReplSetStatus.Member.Health is not available on the node you're connected to
		if member.Health != nil {
			memberHealth.With(ls).Set(float64(*member.Health))
		}

		ch <- prometheus.MustNewConstMetric(memberUptimeDesc, prometheus.CounterValue, member.Uptime, replStatus.Set, member.Name, member.StateStr)

		memberOptimeDate.With(ls).Set(float64(member.OptimeDate.Unix()))

		if member.StateStr == "SECONDARY" {
			memberRepLag.With(ls).Set(primaryOptimeDate - float64(member.OptimeDate.Unix()))
			memberOperationalLag.With(ls).Set(float64(replStatus.Date.Unix()) - primaryLastHeartbeatRecv)
		}

		// ReplSetGetStatus.Member.ElectionTime is only available on the PRIMARY
		if member.ElectionDate != nil {
			memberElectionDate.With(ls).Set(float64((*member.ElectionDate).Unix()))
		}
		if member.LastHeartbeat != nil {
			memberLastHeartbeat.With(ls).Set(float64((*member.LastHeartbeat).Unix()))
		}
		if member.LastHeartbeatRecv != nil {
			memberLastHeartbeatRecv.With(ls).Set(float64((*member.LastHeartbeatRecv).Unix()))
		}
		if member.PingMs != nil {
			memberPingMs.With(ls).Set(*member.PingMs)
		}
		if member.ConfigVersion != nil {
			memberConfigVersion.With(ls).Set(float64(*member.ConfigVersion))
		}
	}
	// collect metrics
	myName.Collect(ch)
	myState.Collect(ch)
	term.Collect(ch)
	date.Collect(ch)
	numberOfMembers.Collect(ch)
	heartbeatIntervalMillis.Collect(ch)
	memberState.Collect(ch)
	memberHealth.Collect(ch)
	memberOptimeDate.Collect(ch)
	memberRepLag.Collect(ch)
	memberOperationalLag.Collect(ch)
	memberElectionDate.Collect(ch)
	memberLastHeartbeat.Collect(ch)
	memberLastHeartbeatRecv.Collect(ch)
	memberPingMs.Collect(ch)
	memberConfigVersion.Collect(ch)
}

// Describe describes the replSetGetStatus metrics for prometheus
func (replStatus *ReplSetStatus) Describe(ch chan<- *prometheus.Desc) {
	myName.Describe(ch)
	myState.Describe(ch)
	term.Describe(ch)
	date.Describe(ch)
	numberOfMembers.Describe(ch)
	heartbeatIntervalMillis.Describe(ch)
	memberState.Describe(ch)
	memberHealth.Describe(ch)
	memberOptimeDate.Describe(ch)
	memberRepLag.Describe(ch)
	memberOperationalLag.Describe(ch)
	memberElectionDate.Describe(ch)
	memberLastHeartbeatRecv.Describe(ch)
	memberPingMs.Describe(ch)
	memberConfigVersion.Describe(ch)

	ch <- memberUptimeDesc
}

// GetReplSetStatus returns the replica status info
func GetReplSetStatus(client *mongo.Client) *ReplSetStatus {
	result := &ReplSetStatus{}
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"replSetGetStatus", 1}}).Decode(result)
	if err != nil {
		log.Errorf("Failed to get replSet status: %s", err)
		return nil
	}
	return result
}
