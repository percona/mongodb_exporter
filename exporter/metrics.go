package exporter

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	exporterPrefix = "mongodb_"
)

//nolint:gochecknoglobals
var (
	// Rules to shrink metric names
	// Please do not change the definitions order: rules are sorted by precedence.
	prefixes = [][]string{
		{"serverStatus.wiredTiger.transaction", "ss_wt_txn"},
		{"serverStatus.wiredTiger", "ss_wt"},
		{"serverStatus", "ss"},
		{"replSetGetStatus", "rs"},
		{"systemMetrics", "sys"},
		{"local.oplog.rs.stats.wiredTiger", "oplog_stats_wt"},
		{"local.oplog.rs.stats", "oplog_stats"},
		{"collstats_storage.wiredTiger", "collstats_storage_wt"},
		{"collstats_storage.indexDetails", "collstats_storage_idx"},
		{"collStats.storageStats", "collstats_storage"},
		{"collStats.latencyStats", "collstats_latency"},
	}

	// This map is used to add labels to some specific metrics.
	// For example, the fields under the serverStatus.opcounters. structure have this
	// signature:
	//
	//    "opcounters": primitive.M{
	//        "insert":  int32(4),
	//        "query":   int32(2118),
	//        "update":  int32(14),
	//        "delete":  int32(22),
	//        "getmore": int32(9141),
	//        "command": int32(67923),
	//    },
	//
	// Applying the renaming rules, serverStatus will become ss but instead of having metrics
	// with the form ss.opcounters.<operation> where operation is each one of the fields inside
	// the structure (insert, query, update, etc), those keys will become labels for the same
	// metric name. The label name is defined as the value for each metric name in the map and
	// the value the labeil will have is the field name in the structure. Example.
	//
	//   mongodb_ss_opcounters{legacy_op_type="insert"} 4
	//   mongodb_ss_opcounters{legacy_op_type="query"} 2118
	//   mongodb_ss_opcounters{legacy_op_type="update"} 14
	//   mongodb_ss_opcounters{legacy_op_type="delete"} 22
	//   mongodb_ss_opcounters{legacy_op_type="getmore"} 9141
	//   mongodb_ss_opcounters{legacy_op_type="command"} 67923
	//
	nodeToPDMetrics = map[string]string{
		"collStats.storageStats.indexDetails.":            "index_name",
		"globalLock.activeQueue.":                         "count_type",
		"globalLock.locks.":                               "lock_type",
		"serverStatus.asserts.":                           "assert_type",
		"serverStatus.connections.":                       "conn_type",
		"serverStatus.globalLock.currentQueue.":           "count_type",
		"serverStatus.metrics.commands.":                  "cmd_name",
		"serverStatus.metrics.cursor.open.":               "csr_type",
		"serverStatus.metrics.document.":                  "doc_op_type",
		"serverStatus.opLatencies.":                       "op_type",
		"serverStatus.opReadConcernCounters.":             "concern_type",
		"serverStatus.opcounters.":                        "legacy_op_type",
		"serverStatus.opcountersRepl.":                    "legacy_op_type",
		"serverStatus.transactions.commitTypes.":          "commit_type",
		"serverStatus.wiredTiger.concurrentTransactions.": "txn_rw_type",
		"serverStatus.wiredTiger.perf.":                   "perf_bucket",
		"systemMetrics.disks.":                            "device_name",
	}

	// Regular expressions used to make the metric name Prometheus-compatible
	// This variables are global to compile the regexps only once.
	specialCharsRe        = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	repeatedUnderscoresRe = regexp.MustCompile(`__+`)
	dollarRe              = regexp.MustCompile(`\_$`)
)

// prometheusize renames metrics by replacing some prefixes with shorter names
// replace special chars to follow Prometheus metric naming rules and adds the
// exporter name prefix.
func prometheusize(s string) string {
	for _, pair := range prefixes {
		if strings.HasPrefix(s, pair[0]+".") {
			s = pair[1] + strings.TrimPrefix(s, pair[0])
			break
		}
	}

	s = specialCharsRe.ReplaceAllString(s, "_")
	s = dollarRe.ReplaceAllString(s, "")
	s = repeatedUnderscoresRe.ReplaceAllString(s, "_")

	return exporterPrefix + s
}

// nameAndLabel checks if there are predefined metric name and label for that metric or
// the standard metrics name should be used in place.
func nameAndLabel(prefix, name string) (string, string) {
	if label, ok := nodeToPDMetrics[prefix]; ok {
		return prometheusize(prefix), label
	}

	return prometheusize(prefix + name), ""
}

// makeRawMetric creates a Prometheus metric based on the parameters we collected by
// traversing the MongoDB structures returned by the collector functions.
func makeRawMetric(prefix, name string, value interface{}, labels map[string]string) (prometheus.Metric, error) {
	var f float64

	switch v := value.(type) {
	case bool:
		if v {
			f = 1
		}
	case int32:
		f = float64(v)
	case int64:
		f = float64(v)
	case float32:
		f = float64(v)
	case float64:
		f = v
	case primitive.DateTime:
		f = float64(v)
	case primitive.A, primitive.ObjectID, primitive.Timestamp, string:
		return nil, nil
	default:
		return nil, fmt.Errorf("makeRawMetric: unhandled type %T", v)
	}

	if labels == nil {
		labels = map[string]string{}
	}

	help := metricHelp(prefix)
	typ := prometheus.UntypedValue

	fqName, label := nameAndLabel(prefix, name)
	if label != "" {
		labels[label] = name
	}

	ln := make([]string, 0, len(labels))
	lv := make([]string, 0, len(labels))

	for k, v := range labels {
		ln = append(ln, k)
		lv = append(lv, v)
	}

	d := prometheus.NewDesc(fqName, help, ln, nil)

	m, err := prometheus.NewConstMetric(d, typ, f, lv...)

	return m, err
}

// metricHelp builds the metric help.
// It is a very very very simple function, but the idea is if the future we want
// to improve the help somehow, there is only one place to change it for the real
// functions and for all the tests.
// Use only prefix because 2 metrics cannot have same name but different help. For metrics
// where we labelize some keys, if we put the real metric name here it will be rejected
// by prometheus
func metricHelp(prefix string) string {
	return prefix
}

func makeMetrics(prefix string, m bson.M, labels map[string]string) []prometheus.Metric {
	var res []prometheus.Metric

	if prefix != "" {
		prefix += "."
	}

	for k, val := range m {
		switch v := val.(type) {
		case bson.M:
			res = append(res, makeMetrics(prefix+k, v, labels)...)
		case map[string]interface{}:
			res = append(res, makeMetrics(prefix+k, v, labels)...)
		case primitive.A:
			v = []interface{}(v)
			res = append(res, processSlice(prefix, k, v)...)
		case []interface{}:
			continue

		default:
			metric, err := makeRawMetric(prefix, k, v, labels)
			if err != nil {
				// TODO
				panic(err)
			}

			if metric != nil {
				res = append(res, metric)
			}
		}
	}

	return res
}

func processSlice(prefix, k string, v []interface{}) []prometheus.Metric {
	metrics := make([]prometheus.Metric, 0)
	labels := make(map[string]string)

	for _, item := range v {
		var s map[string]interface{}

		switch i := item.(type) {
		case map[string]interface{}:
			s = i
		case primitive.M:
			s = map[string]interface{}(i)
		}

		if name, ok := s["name"].(string); ok {
			labels["member_idx"] = name
		}

		metrics = append(metrics, makeMetrics(prefix+k, s, labels)...)
	}

	return metrics
}
