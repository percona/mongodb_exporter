package mongod

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	memberHidden = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_hidden",
		Help:      "This field conveys if the member is hidden (1) or not-hidden (0).",
	}, []string{"id", "host"})
	memberArbiter = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_arbiter",
		Help:      "This field conveys if the member is an arbiter (1) or not (0).",
	}, []string{"id", "host"})
	memberBuildIndexes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_build_indexes",
		Help:      "This field conveys if the member is  builds indexes (1) or not (0).",
	}, []string{"id", "host"})
	memberPriority = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_priority",
		Help:      "This field conveys the priority of a given member",
	}, []string{"id", "host"})
	memberVotes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: subsystem,
		Name:      "member_votes",
		Help:      "This field conveys the number of votes of a given member",
	}, []string{"id", "host"})
)

// OuterReplSetConf Although the docs say that it returns a map with id etc. it *actually* returns that wrapped in a map.
type OuterReplSetConf struct {
	Config ReplSetConf `bson:"config"`
}

// ReplSetConf keeps the data returned by the GetReplSetConf method
type ReplSetConf struct {
	ID      string       `bson:"_id"`
	Version int          `bson:"version"`
	Members []MemberConf `bson:"members"`
}

// MemberConf represents an array element of ReplSetConf.Members
type MemberConf struct {
	ID           int32  `bson:"_id"`
	Host         string `bson:"host"`
	ArbiterOnly  bool   `bson:"arbiterOnly"`
	BuildIndexes bool   `bson:"buildIndexes"`
	Hidden       bool   `bson:"hidden"`
	Priority     int32  `bson:"priority"`

	Tags       map[string]string `bson:"tags"`
	SlaveDelay float64           `bson:"saveDelay"`
	Votes      int32             `bson:"votes"`
}

// Export exports the replSetGetStatus stati to be consumed by prometheus
func (replConf *ReplSetConf) Export(ch chan<- prometheus.Metric) {
	for _, member := range replConf.Members {
		ls := prometheus.Labels{
			"id":   replConf.ID,
			"host": member.Host,
		}
		if member.Hidden {
			memberHidden.With(ls).Set(1)
		} else {
			memberHidden.With(ls).Set(0)
		}

		if member.ArbiterOnly {
			memberArbiter.With(ls).Set(1)
		} else {
			memberArbiter.With(ls).Set(0)
		}

		if member.BuildIndexes {
			memberBuildIndexes.With(ls).Set(1)
		} else {
			memberBuildIndexes.With(ls).Set(0)
		}

		memberPriority.With(ls).Set(float64(member.Priority))
		memberVotes.With(ls).Set(float64(member.Votes))
	}
	// collect metrics
	memberHidden.Collect(ch)
	memberArbiter.Collect(ch)
	memberBuildIndexes.Collect(ch)
	memberPriority.Collect(ch)
	memberVotes.Collect(ch)
}

// Describe describes the replSetGetStatus metrics for prometheus
func (replConf *ReplSetConf) Describe(ch chan<- *prometheus.Desc) {
	memberHidden.Describe(ch)
	memberArbiter.Describe(ch)
	memberBuildIndexes.Describe(ch)
	memberPriority.Describe(ch)
	memberVotes.Describe(ch)
}

// GetReplSetConf returns the replica status info
func GetReplSetConf(client *mongo.Client) *ReplSetConf {
	result := &OuterReplSetConf{}
	err := client.Database("admin").RunCommand(context.TODO(), bson.D{{"replSetGetConfig", 1}}).Decode(result)
	if err != nil {
		log.Error(err)
		return nil
	}
	return &result.Config
}
