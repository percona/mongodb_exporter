package mongod

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"strings"
)

var (
	indexUsage = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "index_usage_count",
		Help:      "Contains a usage count of each index",
	}, []string{"collection", "index"})
)

// IndexStatsList represents index usage information
type IndexStatsList struct {
	Items []IndexUsageStats
}

// IndexUsageStats represents stats about an Index
type IndexUsageStats struct {
	Name       string         `bson:"name"`
	Accesses   IndexUsageInfo `bson:"accesses"`
	Collection string
}

// IndexUsageInfo represents a single index stats of an Index
type IndexUsageInfo struct {
	Ops float64 `bson:"ops"`
}

// Export exports database stats to prometheus
func (indexStats *IndexStatsList) Export(ch chan<- prometheus.Metric) {
	indexUsage.Reset()
	for _, indexStat := range indexStats.Items {
		indexUsage.WithLabelValues(indexStat.Collection, indexStat.Name).Add(indexStat.Accesses.Ops)
	}
	indexUsage.Collect(ch)
}

// Describe describes database stats for prometheus
func (indexStats *IndexStatsList) Describe(ch chan<- *prometheus.Desc) {
	indexUsage.Describe(ch)
}

// GetIndexUsageStatList returns stats for all non-system collections
func GetIndexUsageStatList(session *mgo.Session) *IndexStatsList {
	indexUsageStatsList := &IndexStatsList{}
	log.Info("collecting index stats")
	databaseNames, err := session.DatabaseNames()
	if err != nil {
		log.Error("Failed to get database names")
		return nil
	}
	for _, db := range databaseNames {
		if db == "admin" || db == "config" || "db" == "local" {
			continue
		}
		collectionNames, err := session.DB(db).CollectionNames()
		if err != nil {
			log.Error("Failed to get collection names for db=" + db)
			return nil
		}
		for _, collectionName := range collectionNames {
			if strings.HasPrefix(collectionName, "system.") {
				continue
			}
			collIndexUsageStats := IndexStatsList{}
			err := session.DB(db).C(collectionName).Pipe([]bson.M{{"$indexStats": bson.M{}}}).All(&collIndexUsageStats.Items)
			if err != nil {
				log.Error("Failed to collect index stats for " + db + "." + collectionName)
				return nil
			}
			// Label index stats with corresponding db.collection
			for _, stat := range collIndexUsageStats.Items {
				stat.Collection = db + "." + collectionName
			}
			indexUsageStatsList.Items = append(indexUsageStatsList.Items, collIndexUsageStats.Items...)
		}
	}

	return indexUsageStatsList
}
