package mongod

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	slowQueries = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "profiler",
		Name:      "slow_query",
		Help:      "Number of slow queries recoded in the system.profile collection",
	}, []string{"db", "collection"})
	slowOps = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: Namespace,
		Subsystem: "profiler",
		Name:      "slow_ops",
		Help:      "Number of slow operations reported by $currentOp",
	}, []string{"db", "collection"})

	// Track collections for wich slow queries have been seen so they can be re-set to
	// zero in case no slow queries are detected during metrics refresh.
	// This is done to avoid gaps in reported metrics that would otherwise confuse prometheus.
	trackedLabelsSet = make(map[string]map[string]bool)
)

// DatabaseProfilerRecord represents records returned by the aggregate query.
type DatabaseProfilerRecord struct {
	Namespace   string `bson:"_id,omitempty"`
	SlowQueries int    `bson:"count,omitempty"`
}

// DatabaseProfilerStatList contains stats from all databases
type DatabaseProfilerStatsList struct {
	Members []DatabaseProfilerStats
}

// Describe describes database stats for prometheus
func (dbStatsList *DatabaseProfilerStatsList) Describe(ch chan<- *prometheus.Desc) {
	slowQueries.Describe(ch)
}

// Export exports database stats to prometheus
func (dbStatsList *DatabaseProfilerStatsList) Export(ch chan<- prometheus.Metric) {
	skipValueReset := make(map[string]map[string]bool)
	for _, member := range dbStatsList.Members {
		ls := prometheus.Labels{"db": member.Database, "collection": member.Collection}
		slowQueries.With(ls).Set(float64(member.SlowQueries))

		// Book-keeping around seen collections for resetting correctly.
		if _, ok := skipValueReset[member.Database]; !ok {
			skipValueReset[member.Database] = make(map[string]bool)
		}
		skipValueReset[member.Database][member.Collection] = true
		if _, ok := trackedLabelsSet[member.Database]; !ok {
			trackedLabelsSet[member.Database] = make(map[string]bool)
		}
		trackedLabelsSet[member.Database][member.Collection] = true
	}

	// Set stale slow queries back to 0
	for db, colls := range trackedLabelsSet {
		for coll := range colls {
			if skipValueReset[db][coll] {
				continue
			}
			ls := prometheus.Labels{"db": db, "collection": coll}
			slowQueries.With(ls).Set(0.0)
		}
	}
	slowQueries.Collect(ch)
}

// DatabaseProfilerStats represents profiler aggregated data grouped by db and collection.
type DatabaseProfilerStats struct {
	Database    string
	Collection  string
	SlowQueries int
}

// GetDatabaseProfilerStats returns profiler stats for all databases
func GetDatabaseProfilerStats(client *mongo.Client, lookback int64, millis int64) *DatabaseProfilerStatsList {
	dbStatsList := &DatabaseProfilerStatsList{}
	dbNames, err := client.ListDatabaseNames(context.TODO(), bson.M{})
	if err != nil {
		log.Errorf("Failed to get database names, %v", err)
		return nil
	}
	dbsToSkip := map[string]bool{
		"admin":  true,
		"config": true,
		"local":  true,
		"test":   true,
	}
	for _, db := range dbNames {
		if dbsToSkip[db] {
			continue
		}
		from := time.Unix(time.Now().UTC().Unix()-lookback, 0)
		match := bson.M{"$match": bson.M{
			"ts":     bson.M{"$gt": from},
			"millis": bson.M{"$gte": millis},
		}}
		group := bson.M{"$group": bson.M{
			"_id":   "$ns",
			"count": bson.M{"$sum": 1},
		}}
		pipeline := []bson.M{match, group}
		cursor, err := client.Database(db).Collection("system.profile").Aggregate(context.TODO(), pipeline)
		if err != nil {
			log.Errorf("Failed to get database profiler stats: %s.", err)
			return nil
		}
		defer cursor.Close(context.TODO())
		for cursor.Next(context.TODO()) {
			record := DatabaseProfilerRecord{}
			err := cursor.Decode(&record)
			if err != nil {
				log.Errorf("Failed to iterate database profiler stats: %s.", err)
				return nil
			}
			ns := strings.SplitN(record.Namespace, ".", 2)
			db := ns[0]
			coll := ns[1]
			stats := DatabaseProfilerStats{
				Database:    db,
				Collection:  coll,
				SlowQueries: record.SlowQueries,
			}
			dbStatsList.Members = append(dbStatsList.Members, stats)
		}
		if err := cursor.Err(); err != nil {
			log.Errorf("Failed to iterate database profiler stats: %s.", err)
			return nil
		}
	}
	return dbStatsList
}

// DatabaseCurrentOpStatsList contains stats from all databases
type DatabaseCurrentOpStatsList struct {
	Members []DatabaseProfilerStats
}

// Describe describes $currentOp stats for prometheus
func (dbStatsList *DatabaseCurrentOpStatsList) Describe(ch chan<- *prometheus.Desc) {
	slowOps.Describe(ch)
}

// Export exports database stats to prometheus
func (dbStatsList *DatabaseCurrentOpStatsList) Export(ch chan<- prometheus.Metric) {
	skipValueReset := make(map[string]map[string]bool)
	for _, member := range dbStatsList.Members {
		ls := prometheus.Labels{"db": member.Database, "collection": member.Collection}
		slowOps.With(ls).Set(float64(member.SlowQueries))

		// Book-keeping around seen collections for resetting correctly.
		if _, ok := skipValueReset[member.Database]; !ok {
			skipValueReset[member.Database] = make(map[string]bool)
		}
		skipValueReset[member.Database][member.Collection] = true
		if _, ok := trackedLabelsSet[member.Database]; !ok {
			trackedLabelsSet[member.Database] = make(map[string]bool)
		}
		trackedLabelsSet[member.Database][member.Collection] = true
	}

	// Set stale slow ops back to 0
	for db, colls := range trackedLabelsSet {
		for coll := range colls {
			if skipValueReset[db][coll] {
				continue
			}
			ls := prometheus.Labels{"db": db, "collection": coll}
			slowOps.With(ls).Set(0.0)
		}
	}
	slowOps.Collect(ch)
}

// GetDatabaseCurrentOpStats returns $currentOp stats for all databases
func GetDatabaseCurrentOpStats(client *mongo.Client, millis int64) *DatabaseCurrentOpStatsList {
	dbStatsList := &DatabaseCurrentOpStatsList{}
	currentOp := bson.M{"$currentOp": bson.M{
		"allUsers": true,
	}}
	match := bson.M{"$match": bson.M{
		"microsecs_running": bson.M{"$gte": millis * 1000},
	}}
	group := bson.M{"$group": bson.M{
		"_id":   "$ns",
		"count": bson.M{"$sum": 1},
	}}
	pipeline := []bson.M{currentOp, match, group}
	// Need the command version of aggregate to use $currentOp.
	//   https://docs.mongodb.com/manual/reference/command/aggregate/#dbcmd.aggregate
	aggregate := bson.D{
		{"aggregate", 1},
		{"pipeline", pipeline},
		{"cursor", bson.M{}},
	}
	cursor, err := client.Database("admin").RunCommandCursor(context.TODO(), aggregate)
	if err != nil {
		log.Errorf("Failed to get $currentOp stats: %v", err)
		return nil
	}
	defer cursor.Close(context.TODO())
	dbsToSkip := map[string]bool{
		"admin":  true,
		"config": true,
		"local":  true,
		"test":   true,
	}
	for cursor.Next(context.TODO()) {
		record := DatabaseProfilerRecord{}
		err := cursor.Decode(&record)
		if err != nil {
			log.Errorf("Failed to iterate $currentOp stats: %s.", err)
			return nil
		}
		ns := strings.SplitN(record.Namespace, ".", 2)
		db := ns[0]
		if dbsToSkip[db] {
			continue
		}
		coll := ns[1]
		stats := DatabaseProfilerStats{
			Database:    db,
			Collection:  coll,
			SlowQueries: record.SlowQueries,
		}
		dbStatsList.Members = append(dbStatsList.Members, stats)
	}
	if err := cursor.Err(); err != nil {
		log.Errorf("Failed to iterate $currentOp stats: %s.", err)
		return nil
	}
	return dbStatsList
}
