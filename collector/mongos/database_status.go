package mongos

import (
	"context"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	indexSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "index_size_bytes",
		Help:      "The total size in bytes of all indexes created on this database",
	}, []string{"db", "shard"})
	dataSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "data_size_bytes",
		Help:      "The total size in bytes of the uncompressed data held in this database",
	}, []string{"db", "shard"})
	collectionsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "collections_total",
		Help:      "Contains a count of the number of collections in that database",
	}, []string{"db", "shard"})
	indexesTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "indexes_total",
		Help:      "Contains a count of the total number of indexes across all collections in the database",
	}, []string{"db", "shard"})
	objectsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "objects_total",
		Help:      "Contains a count of the number of objects (i.e. documents) in the database across all collections",
	}, []string{"db", "shard"})
)

// DatabaseStatList contains stats from all databases
type DatabaseStatList struct {
	Members []DatabaseStatus
}

// DatabaseStatus represents stats about a database (mongod and raw from mongos)
type DatabaseStatus struct {
	RawStatus                       // embed to collect top-level attributes
	Shards    map[string]*RawStatus `bson:"raw,omitempty"`
}

// RawStatus represents stats about a database from Mongos side
type RawStatus struct {
	Name        string `bson:"db,omitempty"`
	IndexSize   int    `bson:"indexSize,omitempty"`
	DataSize    int    `bson:"dataSize,omitempty"`
	Collections int    `bson:"collections,omitempty"`
	Objects     int    `bson:"objects,omitempty"`
	Indexes     int    `bson:"indexes,omitempty"`
}

// Export exports database stats to prometheus
func (dbStatList *DatabaseStatList) Export(ch chan<- prometheus.Metric) {
	for _, member := range dbStatList.Members {
		if len(member.Shards) > 0 {
			for shard, stats := range member.Shards {
				ls := prometheus.Labels{
					"db":    stats.Name,
					"shard": strings.Split(shard, "/")[0],
				}
				indexSize.With(ls).Set(float64(stats.IndexSize))
				dataSize.With(ls).Set(float64(stats.DataSize))
				collectionsTotal.With(ls).Set(float64(stats.Collections))
				indexesTotal.With(ls).Set(float64(stats.Indexes))
				objectsTotal.With(ls).Set(float64(stats.Objects))
			}
		}
	}

	indexSize.Collect(ch)
	dataSize.Collect(ch)
	collectionsTotal.Collect(ch)
	indexesTotal.Collect(ch)
	objectsTotal.Collect(ch)

	indexSize.Reset()
	dataSize.Reset()
	collectionsTotal.Reset()
	indexesTotal.Reset()
	objectsTotal.Reset()
}

// Describe describes database stats for prometheus
func (dbStatList *DatabaseStatList) Describe(ch chan<- *prometheus.Desc) {
	indexSize.Describe(ch)
	dataSize.Describe(ch)
	collectionsTotal.Describe(ch)
	indexesTotal.Describe(ch)
	objectsTotal.Describe(ch)
}

// GetDatabaseStatList returns stats for all databases
func GetDatabaseStatList(client *mongo.Client) *DatabaseStatList {
	dbStatList := &DatabaseStatList{}
	dbNames, err := client.ListDatabaseNames(context.TODO(), bson.M{})
	if err != nil {
		log.Errorf("Failed to get database names: %s.", err)
		return nil
	}
	for _, db := range dbNames {
		dbStatus := DatabaseStatus{}
		r := client.Database(db).RunCommand(context.TODO(), bson.D{{"dbStats", 1}, {"scale", 1}})
		err := r.Decode(&dbStatus)
		if err != nil {
			log.Errorf("Failed to get database status: %s.", err)
			return nil
		}
		dbStatList.Members = append(dbStatList.Members, dbStatus)
	}

	return dbStatList
}
