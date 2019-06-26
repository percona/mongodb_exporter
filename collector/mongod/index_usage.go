package mongod

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
func GetIndexUsageStatList(client *mongo.Client) *IndexStatsList {
	indexUsageStatsList := &IndexStatsList{}
	databaseNames, err := client.ListDatabaseNames(context.TODO(), bson.M{})
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
		c, err := client.Database(dbName).ListCollections(context.TODO(), bson.M{}, options.ListCollections().SetNameOnly(true))
		if err != nil {
			_, logSFound := logSuppressIS[dbName]
			if !logSFound {
				log.Errorf("%s. Index usage stats will not be collected for this db. This log message will be suppressed from now.", err)
				logSuppressIS[dbName] = true
			}
		} else {

			type collListItem struct {
				Name string `bson:"name,omitempty"`
				Type string `bson:"type,omitempty"`
			}

			delete(logSuppressIS, dbName)
			for c.Next(context.TODO()) {
				coll := &collListItem{}
				err := c.Decode(&coll)
				if err != nil {
					log.Error(err)
					continue
				}

				collIndexUsageStats := IndexStatsList{}
				c, err := client.Database(dbName).Collection(coll.Name).Aggregate(context.TODO(), []bson.M{{"$indexStats": bson.M{}}})
				if err != nil {
					_, logSFound := logSuppressIS[dbName+"."+coll.Name]
					if !logSFound {
						log.Errorf("%s. Index usage stats will not be collected for this collection. This log message will be suppressed from now.", err)
						logSuppressIS[dbName+"."+coll.Name] = true
					}
				} else {

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

					delete(logSuppressIS, dbName+"."+coll.Name)
					// Label index stats with corresponding db.collection
					for i := 0; i < len(collIndexUsageStats.Items); i++ {
						collIndexUsageStats.Items[i].Database = dbName
						collIndexUsageStats.Items[i].Collection = coll.Name
					}
					indexUsageStatsList.Items = append(indexUsageStatsList.Items, collIndexUsageStats.Items...)
				}
			}
			if err := c.Close(context.TODO()); err != nil {
				log.Errorf("Could not close ListCollections() cursor, reason: %v", err)
			}
		}
	}

	return indexUsageStatsList
}
