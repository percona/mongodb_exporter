package mongod

import (
	"context"

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
	}, []string{"db"})
	dataSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "data_size_bytes",
		Help:      "The total size in bytes of the uncompressed data held in this database",
	}, []string{"db"})
	collectionsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "collections_total",
		Help:      "Contains a count of the number of collections in that database",
	}, []string{"db"})
	indexesTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "indexes_total",
		Help:      "Contains a count of the total number of indexes across all collections in the database",
	}, []string{"db"})
	objectsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db",
		Name:      "objects_total",
		Help:      "Contains a count of the number of objects (i.e. documents) in the database across all collections",
	}, []string{"db"})
)

// DatabaseStatList contains stats from all databases
type DatabaseStatList struct {
	Members []DatabaseStatus
}

// DatabaseStatus represents stats about a database (mongod and raw from mongos)
type DatabaseStatus struct {
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
		ls := prometheus.Labels{"db": member.Name}
		indexSize.With(ls).Set(float64(member.IndexSize))
		dataSize.With(ls).Set(float64(member.DataSize))
		collectionsTotal.With(ls).Set(float64(member.Collections))
		indexesTotal.With(ls).Set(float64(member.Indexes))
		objectsTotal.With(ls).Set(float64(member.Objects))
	}
	indexSize.Collect(ch)
	dataSize.Collect(ch)
	collectionsTotal.Collect(ch)
	indexesTotal.Collect(ch)
	objectsTotal.Collect(ch)

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
		log.Errorf("Failed to get database names, %v", err)
		return nil
	}
	for _, db := range dbNames {
		dbStatus := DatabaseStatus{}
		err := client.Database(db).RunCommand(context.TODO(), bson.D{{"dbStats", 1}, {"scale", 1}}).Decode(&dbStatus)
		if err != nil {
			log.Errorf("Failed to get database status: %s.", err)
			return nil
		}
		dbStatList.Members = append(dbStatList.Members, dbStatus)
	}

	return dbStatList
}
