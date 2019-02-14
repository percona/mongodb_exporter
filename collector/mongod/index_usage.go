package mongod

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	indexUsage = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "index_usage_count",
		Help:      "Contains a usage count of each index",
	}, []string{"collection", "db", "index"})
)

// IndexStatsList represents index usage information
type IndexStatsList struct {
	Items []IndexUsageStats
}

// IndexUsageStats represents stats about an Index
type IndexUsageStats struct {
	Name       string         `bson:"name"`
	Accesses   IndexUsageInfo `bson:"accesses"`
	Database   string
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
		ls := prometheus.Labels{
			"db":         indexStat.Database,
			"collection": indexStat.Collection,
			"index":      indexStat.Name,
		}
		indexUsage.With(ls).Add(indexStat.Accesses.Ops)
	}
	indexUsage.Collect(ch)
}

// Describe describes database stats for prometheus
func (indexStats *IndexStatsList) Describe(ch chan<- *prometheus.Desc) {
	indexUsage.Describe(ch)
}

var (
	logSuppressIS = make(map[string]bool)
)

// GetIndexUsageStatList returns stats for a given collection in a database
func GetIndexUsageStatList(session *mgo.Session) *IndexStatsList {
	indexUsageStatsList := &IndexStatsList{}
	databaseNames, err := session.DatabaseNames()
	if err != nil {
		_, logSFound := logSuppressIS[""]
		if !logSFound {
			log.Errorf("%s. Index usage stats will not be collected. This log message will be suppressed from now.", err)
			logSuppressIS[""] = true
		}
		return nil
	}
	delete(logSuppressIS, "")
	for _, dbName := range databaseNames {
		collNames, err := session.DB(dbName).CollectionNames()
		if err != nil {
			_, logSFound := logSuppressIS[dbName]
			if !logSFound {
				log.Errorf("%s. Index usage stats will not be collected for this db. This log message will be suppressed from now.", err)
				logSuppressIS[dbName] = true
			}
		} else {
			delete(logSuppressIS, dbName)
			for _, collName := range collNames {

				collIndexUsageStats := IndexStatsList{}
				err := session.DB(dbName).C(collName).Pipe([]bson.M{{"$indexStats": bson.M{}}}).All(&collIndexUsageStats.Items)
				if err != nil {
					_, logSFound := logSuppressIS[dbName+"."+collName]
					if !logSFound {
						log.Errorf("%s. Index usage stats will not be collected for this collection. This log message will be suppressed from now.", err)
						logSuppressIS[dbName+"."+collName] = true
					}
				} else {
					delete(logSuppressIS, dbName+"."+collName)
					// Label index stats with corresponding db.collection
					for i := 0; i < len(collIndexUsageStats.Items); i++ {
						collIndexUsageStats.Items[i].Database = dbName
						collIndexUsageStats.Items[i].Collection = collName
					}
					indexUsageStatsList.Items = append(indexUsageStatsList.Items, collIndexUsageStats.Items...)
				}
			}
		}
	}

	return indexUsageStatsList
}
