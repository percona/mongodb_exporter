// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mongod

import (
	"github.com/buger/jsonparser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type TopScrape struct {
	Bytes []byte
	// Maps db to (slice) of collections in db
	DbCollMap map[string][]string
}

type CollCounters struct {
	Total     float64
	ReadLock  float64
	WriteLock float64
	Queries   float64
	GetMore   float64
	Insert    float64
	Update    float64
	Delete    float64
	Commands  float64
}

var (
	collCountersTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "coll_counters_total",
		Help:      "coll_counters_total provide number of queries|getmore|insert|update|remove|commands for each collection in every database",
	}, []string{"type", "db", "coll"})
)

// Export exports the collection metrics to be consumed by prometheus.
func (top *TopScrape) Export(ch chan<- prometheus.Metric) {
	for db, collection := range top.DbCollMap {
		for _, coll := range collection {
			counters, err := getCollCounters(top.Bytes, db, coll)
			if err != nil {
				continue
			}
			collCountersTotal.WithLabelValues("total", db, coll).Set(counters.Total)
			collCountersTotal.WithLabelValues("readlock", db, coll).Set(counters.ReadLock)
			collCountersTotal.WithLabelValues("writelock", db, coll).Set(counters.WriteLock)
			collCountersTotal.WithLabelValues("query", db, coll).Set(counters.Queries)
			collCountersTotal.WithLabelValues("getmore", db, coll).Set(counters.GetMore)
			collCountersTotal.WithLabelValues("insert", db, coll).Set(counters.Insert)
			collCountersTotal.WithLabelValues("update", db, coll).Set(counters.Update)
			collCountersTotal.WithLabelValues("delete", db, coll).Set(counters.Delete)
			collCountersTotal.WithLabelValues("command", db, coll).Set(counters.Commands)
		}
	}

	collCountersTotal.Collect(ch)
}

// Describe describes the server status for prometheus.
func (top *TopScrape) Describe(ch chan<- *prometheus.Desc) {
	if top.Bytes != nil {
		collCountersTotal.Describe(ch)
	}
}

func getCollCounters(metricsBytes []byte, dbname string, collection string) (CollCounters, error) {
	var counters CollCounters

	metric := []string{"total", "readLock", "writeLock", "queries", "getmore", "insert", "update", "remove", "commands"}

	for _, m := range metric {
		val, err := jsonparser.GetFloat(metricsBytes, "totals", dbname+"."+collection, m, "count")
		if err != nil {
			if err.Error() == "Key path not found" {
				continue
			} else {
				log.Errorf("Failed to parse top output for db=%s, collection=%s err=%s", dbname, collection, err.Error())
				return counters, err
			}
		}
		switch m {
		case "total":
			counters.Total = val
		case "readLock":
			counters.ReadLock = val
		case "writeLock":
			counters.WriteLock = val
		case "queries":
			counters.Queries = val
		case "getmore":
			counters.GetMore = val
		case "insert":
			counters.Insert = val
		case "update":
			counters.Update = val
		case "remove":
			//Rename 'remove' to Delete
			counters.Delete = val
		case "commands":
			counters.Commands = val
		}
	}
	return counters, nil
}

// GetCollMetrics returns the server status info.
func GetTop(session *mgo.Session) (*TopScrape, error) {
	var (
		results TopScrape
	)

	results.DbCollMap = make(map[string][]string)

	//Get databases & collection on a shard and create a map
	databases, err := session.DatabaseNames()
	if err != nil {
		log.Errorf("Failed to get databases (err=%s)", err)
		return nil, err
	}

	for _, db := range databases {
		collections, err := session.DB(db).CollectionNames()
		if err != nil {
			log.Errorf("Failed to get collections in db=%s (err=%s)", db, err)
			return nil, err
		}
		for _, coll := range collections {
			results.DbCollMap[db] = append(results.DbCollMap[db], coll)
		}
	}

	//Get 'top' output
	result := bson.M{}
	err = session.DB("admin").Run(bson.D{{"top", 1}, {"recordStats", 0}}, result)
	if err != nil {
		log.Error("Failed to get server status.")
		return nil, err
	}
	results.Bytes, err = bson.MarshalJSON(result)
	if err != nil {
		log.Errorf("error in bson.MarshalJSON (err=%s)", err)
		return nil, err
	}
	return &results, nil
}
