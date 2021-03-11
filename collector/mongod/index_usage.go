package mongod

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/percona/mongodb_exporter/collector/common"
	"github.com/percona/mongodb_exporter/shared"
)

var indexUsage = prometheus.NewCounterVec(prometheus.CounterOpts{
	Namespace: Namespace,
	Name:      "index_usage_count",
	Help:      "Contains a usage count of each index"},
	[]string{"collection", "db", "index"})

// IndexStatsList represents index usage information.
type IndexStatsList struct {
	Items []IndexUsageStats
}

// IndexUsageStats represents stats about an Index.
type IndexUsageStats struct {
	Name       string         `bson:"name"`
	Accesses   IndexUsageInfo `bson:"accesses"`
	Database   string
	Collection string
}

// IndexUsageInfo represents a single index stats of an Index.
type IndexUsageInfo struct {
	Ops float64 `bson:"ops"`
}

// Export exports database stats to prometheus.
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

// Describe describes database stats for prometheus.
func (indexStats *IndexStatsList) Describe(ch chan<- *prometheus.Desc) {
	indexUsage.Describe(ch)
}

var logSuppressIS = shared.NewSyncStringSet()

const keyIS = ""

// GetIndexUsageStatList returns stats for all non-system collections.
func GetIndexUsageStatList(client *mongo.Client) *IndexStatsList {
	indexUsageStatsList := &IndexStatsList{}
	databaseNames, err := client.ListDatabaseNames(context.TODO(), bson.M{})
	if err != nil {
		if !logSuppressIS.Contains(keyIS) {
			log.Warnf("%s. Index usage stats will not be collected. This log message will be suppressed from now.", err)
			logSuppressIS.Add(keyIS)
		}
		return nil
	}

	logSuppressIS.Delete(keyIS)
	for _, dbName := range databaseNames {
		if common.IsSystemDB(dbName) {
			continue
		}

		collNames, err := client.Database(dbName).ListCollectionNames(context.TODO(), bson.M{})
		if err != nil {
			if !logSuppressIS.Contains(dbName) {
				log.Warnf("%s. Index usage stats will not be collected for this db. This log message will be suppressed from now.", err)
				logSuppressIS.Add(dbName)
			}
			continue
		}

		logSuppressIS.Delete(dbName)
		for _, collName := range collNames {
			if common.IsSystemCollection(collName) {
				continue
			}

			fullCollName := common.CollFullName(dbName, collName)
			collIndexUsageStats := IndexStatsList{}
			c, err := client.Database(dbName).Collection(collName).Aggregate(context.TODO(), []bson.M{{"$indexStats": bson.M{}}})
			if err != nil {
				if !logSuppressIS.Contains(fullCollName) {
					log.Warnf("%s. Index usage stats will not be collected for this collection. This log message will be suppressed from now.", err)
					logSuppressIS.Add(fullCollName)
				}
				continue
			}

			for c.Next(context.TODO()) {
				s := &IndexUsageStats{}
				if err := c.Decode(s); err != nil {
					log.Error(err)
					continue
				}
				collIndexUsageStats.Items = append(collIndexUsageStats.Items, *s)
			}

			if err := c.Err(); err != nil {
				log.Error(err)
			}

			if err := c.Close(context.TODO()); err != nil {
				log.Errorf("Could not close Aggregate() cursor, reason: %v", err)
			}

			logSuppressIS.Delete(fullCollName)
			// Label index stats with corresponding db.collection.
			for i := 0; i < len(collIndexUsageStats.Items); i++ {
				collIndexUsageStats.Items[i].Database = dbName
				collIndexUsageStats.Items[i].Collection = collName
			}
			indexUsageStatsList.Items = append(indexUsageStatsList.Items, collIndexUsageStats.Items...)

		}
	}

	return indexUsageStatsList
}
