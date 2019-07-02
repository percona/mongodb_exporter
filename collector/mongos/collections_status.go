package mongos

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	collectionSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "size",
		Help:      "The total size in memory of all records in a collection",
	}, []string{"db", "coll"})
	collectionObjectCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "count",
		Help:      "The number of objects or documents in this collection",
	}, []string{"db", "coll"})
	collectionAvgObjSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "avgobjsize",
		Help:      "The average size of an object in the collection (plus any padding)",
	}, []string{"db", "coll"})
	collectionStorageSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "storage_size",
		Help:      "The total amount of storage allocated to this collection for document storage",
	}, []string{"db", "coll"})
	collectionIndexes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "indexes",
		Help:      "The number of indexes on the collection",
	}, []string{"db", "coll"})
	collectionIndexesSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "indexes_size",
		Help:      "The total size of all indexes",
	}, []string{"db", "coll"})
)

// CollectionStatList contains stats from all collections
type CollectionStatList struct {
	Members []CollectionStatus
}

// CollectionStatus represents stats about a collection in database (mongod and raw from mongos)
type CollectionStatus struct {
	Database    string
	Name        string
	Size        int `bson:"size,omitempty"`
	Count       int `bson:"count,omitempty"`
	AvgObjSize  int `bson:"avgObjSize,omitempty"`
	StorageSize int `bson:"storageSize,omitempty"`
	Indexes     int `bson:"indexSizes,omitempty"`
	IndexesSize int `bson:"totalIndexSize,omitempty"`
}

// Export exports database stats to prometheus
func (collStatList *CollectionStatList) Export(ch chan<- prometheus.Metric) {
	for _, member := range collStatList.Members {
		ls := prometheus.Labels{
			"db":   member.Database,
			"coll": member.Name,
		}
		collectionSize.With(ls).Set(float64(member.Size))
		collectionObjectCount.With(ls).Set(float64(member.Count))
		collectionAvgObjSize.With(ls).Set(float64(member.AvgObjSize))
		collectionStorageSize.With(ls).Set(float64(member.StorageSize))
		collectionIndexes.With(ls).Set(float64(member.Indexes))
		collectionIndexesSize.With(ls).Set(float64(member.IndexesSize))
	}
	collectionSize.Collect(ch)
	collectionObjectCount.Collect(ch)
	collectionAvgObjSize.Collect(ch)
	collectionStorageSize.Collect(ch)
	collectionIndexes.Collect(ch)
	collectionIndexesSize.Collect(ch)
}

// Describe describes database stats for prometheus
func (collStatList *CollectionStatList) Describe(ch chan<- *prometheus.Desc) {
	collectionSize.Describe(ch)
	collectionObjectCount.Describe(ch)
	collectionAvgObjSize.Describe(ch)
	collectionStorageSize.Describe(ch)
	collectionIndexes.Describe(ch)
	collectionIndexesSize.Describe(ch)
}

var (
	logSuppressCS = make(map[string]bool)
)

// GetCollectionStatList returns stats for a given database
func GetCollectionStatList(client *mongo.Client) *CollectionStatList {
	collectionStatList := &CollectionStatList{}
	dbNames, err := client.ListDatabaseNames(context.TODO(), bson.M{})
	if err != nil {
		_, logSFound := logSuppressCS[""]
		if !logSFound {
			log.Errorf("%s. Collection stats will not be collected. This log message will be suppressed from now.", err)
			logSuppressCS[""] = true
		}
		return nil
	}
	delete(logSuppressCS, "")
	for _, dbName := range dbNames {
		c, err := client.Database(dbName).ListCollections(context.TODO(), bson.M{}, options.ListCollections().SetNameOnly(true))
		if err != nil {
			_, logSFound := logSuppressCS[dbName]
			if !logSFound {
				log.Errorf("%s. Collection stats will not be collected for this db. This log message will be suppressed from now.", err)
				logSuppressCS[dbName] = true
			}
		} else {

			type collListItem struct {
				Name string `bson:"name,omitempty"`
				Type string `bson:"type,omitempty"`
			}

			delete(logSuppressCS, dbName)
			for c.Next(context.TODO()) {
				coll := &collListItem{}
				err := c.Decode(&coll)
				if err != nil {
					log.Error(err)
					continue
				}
				collStatus := CollectionStatus{}
				res := client.Database(dbName).RunCommand(context.TODO(), bson.D{{"collStats", coll.Name}, {"scale", 1}})
				err = res.Decode(&collStatus)
				if err != nil {
					_, logSFound := logSuppressCS[dbName+"."+coll.Name]
					if !logSFound {
						log.Errorf("%s. Collection stats will not be collected for this collection. This log message will be suppressed from now.", err)
						logSuppressCS[dbName+"."+coll.Name] = true
					}
				} else {
					delete(logSuppressCS, dbName+"."+coll.Name)
					collStatus.Database = dbName
					collStatus.Name = coll.Name
					collectionStatList.Members = append(collectionStatList.Members, collStatus)
				}
			}

			if err := c.Err(); err != nil {
				log.Error(err)
			}

			if err := c.Close(context.TODO()); err != nil {
				log.Errorf("Could not close ListCollections() cursor, reason: %v", err)
			}
		}
	}

	return collectionStatList
}
