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
	collectionIndexSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "index_size",
		Help:      "The individual index size",
	}, []string{"db", "coll", "index"})
	collectionWTReusableFileBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "wiredtiger_file_reusable_bytes",
		Help:      "Bytes Available for Reuse",
	}, []string{"db", "coll"})
	collectionWTFileSizeInBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "db_coll",
		Name:      "wiredtiger_file_allocated_bytes",
		Help:      "Allocated file bytes (file size on disk)",
	}, []string{"db", "coll"})
)

// CollectionStatList contains stats from all collections.
type CollectionStatList struct {
	Members []CollectionStatus
}

// Wired Tiger Block Manager Stats
type BlockManagerStats struct {
	AllocationsRequiringFileExtension int `bson:"allocations requiring file extension,omitempty"`
	BlocksAllocated                   int `bson:"blocks allocated,omitempty"`
	BlocksFreed                       int `bson:"blocks freed,omitempty"`
	CheckpointSize                    int `bson:"checkpoint size,omitempty"`
	FileAllocationUnitSize            int `bson:"file allocation unit size,omitempty"`
	FileBytesAvailableForReuse        int `bson:"file bytes available for reuse,omitempty"`
	FileMagicNumber                   int `bson:"file magic number,omitempty"`
	FileMajorVersionNumber            int `bson:"file major version number,omitempty"`
	FileSizeInBytes                   int `bson:"file size in bytes,omitempty"`
	MinorVersionNumber                int `bson:"minor version number,omitempty"`
}

// Collection Wired Tiger Stats
type CollectionWiredTigerStats struct {
	BlockManager BlockManagerStats `bson:"block-manager,omitempty"`
}

// CollectionStatus represents stats about a collection in database (mongod and raw from mongos).
type CollectionStatus struct {
	Database    string
	Name        string
	Size        int                       `bson:"size,omitempty"`
	Count       int                       `bson:"count,omitempty"`
	AvgObjSize  int                       `bson:"avgObjSize,omitempty"`
	StorageSize int                       `bson:"storageSize,omitempty"`
	IndexesSize int                       `bson:"totalIndexSize,omitempty"`
	IndexSizes  map[string]float64        `bson:"indexSizes,omitempty"`
	WiredTiger  CollectionWiredTigerStats `bson:"wiredTiger,omitempty"`
}

// Export exports database stats to prometheus.
func (collStatList *CollectionStatList) Export(ch chan<- prometheus.Metric) {
	// reset previously collected values
	collectionSize.Reset()
	collectionObjectCount.Reset()
	collectionAvgObjSize.Reset()
	collectionStorageSize.Reset()
	collectionIndexes.Reset()
	collectionIndexesSize.Reset()
	collectionIndexSize.Reset()
	collectionWTReusableFileBytes.Reset()
	collectionWTFileSizeInBytes.Reset()

	for _, member := range collStatList.Members {
		ls := prometheus.Labels{
			"db":   member.Database,
			"coll": member.Name,
		}
		collectionSize.With(ls).Set(float64(member.Size))
		collectionObjectCount.With(ls).Set(float64(member.Count))
		collectionAvgObjSize.With(ls).Set(float64(member.AvgObjSize))
		collectionStorageSize.With(ls).Set(float64(member.StorageSize))
		collectionIndexes.With(ls).Set(float64(len(member.IndexSizes)))
		collectionIndexesSize.With(ls).Set(float64(member.IndexesSize))
		collectionWTReusableFileBytes.With(ls).Set(float64(member.WiredTiger.BlockManager.FileBytesAvailableForReuse))
		collectionWTFileSizeInBytes.With(ls).Set(float64(member.WiredTiger.BlockManager.FileSizeInBytes))

		for indexName, size := range member.IndexSizes {
			ls = prometheus.Labels{
				"db":    member.Database,
				"coll":  member.Name,
				"index": indexName,
			}
			collectionIndexSize.With(ls).Set(size)
		}
	}
	collectionSize.Collect(ch)
	collectionObjectCount.Collect(ch)
	collectionAvgObjSize.Collect(ch)
	collectionStorageSize.Collect(ch)
	collectionIndexes.Collect(ch)
	collectionIndexesSize.Collect(ch)
	collectionIndexSize.Collect(ch)
	collectionWTReusableFileBytes.Collect(ch)
	collectionWTFileSizeInBytes.Collect(ch)
}

// Describe describes database stats for prometheus.
func (collStatList *CollectionStatList) Describe(ch chan<- *prometheus.Desc) {
	collectionSize.Describe(ch)
	collectionObjectCount.Describe(ch)
	collectionAvgObjSize.Describe(ch)
	collectionStorageSize.Describe(ch)
	collectionIndexes.Describe(ch)
	collectionIndexesSize.Describe(ch)
	collectionWTReusableFileBytes.Describe(ch)
	collectionWTFileSizeInBytes.Describe(ch)
}

var logSuppressCS = shared.NewSyncStringSet()

const keyCS = ""

// GetCollectionStatList returns stats for all non-system collections.
func GetCollectionStatList(client *mongo.Client) *CollectionStatList {
	collectionStatList := &CollectionStatList{}
	dbNames, err := client.ListDatabaseNames(context.TODO(), bson.M{})
	if err != nil {
		if !logSuppressCS.Contains(keyCS) {
			log.Warnf("%s. Collection stats will not be collected. This log message will be suppressed from now.", err)
			logSuppressCS.Add(keyCS)
		}
		return nil
	}

	logSuppressCS.Delete(keyCS)
	for _, dbName := range dbNames {
		if common.IsSystemDB(dbName) {
			continue
		}

		collNames, err := client.Database(dbName).ListCollectionNames(context.TODO(), bson.M{})
		if err != nil {
			if !logSuppressCS.Contains(dbName) {
				log.Warnf("%s. Collection stats will not be collected for this db. This log message will be suppressed from now.", err)
				logSuppressCS.Add(dbName)
			}
			continue
		}

		logSuppressCS.Delete(dbName)
		for _, collName := range collNames {
			if common.IsSystemCollection(collName) {
				continue
			}

			fullCollName := common.CollFullName(dbName, collName)
			collStatus := CollectionStatus{}
			err = client.Database(dbName).RunCommand(context.TODO(), bson.D{{"collStats", collName}, {"scale", 1}}).Decode(&collStatus)
			if err != nil {
				if !logSuppressCS.Contains(fullCollName) {
					log.Warnf("%s. Collection stats will not be collected for this collection. This log message will be suppressed from now.", err)
					logSuppressCS.Add(fullCollName)
				}
				continue
			}

			logSuppressCS.Delete(fullCollName)
			collStatus.Database = dbName
			collStatus.Name = collName
			collectionStatList.Members = append(collectionStatList.Members, collStatus)
		}
	}

	return collectionStatList
}
