// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	exporterPrefix         = "mongodb_"
	histogramLowerBoundKey = "lowerBound"
	histogramMicrosKey     = "micros"
	// collisionLabel keeps document fields apart when they sanitize to the same metric name.
	collisionLabel = "metric_idx"
)

type rawMetric struct {
	// Full Qualified Name
	fqName string
	// Help string
	help string
	// Label names
	ln []string
	// Label values
	lv []string
	// Metric value as float64
	val float64
	// Value type
	vt prometheus.ValueType
}

//nolint:gochecknoglobals
var (
	// Rules to shrink metric names
	// Please do not change the definitions order: rules are sorted by precedence.
	prefixes = [][]string{
		{"serverStatus.wiredTiger.transaction", "ss_wt_txn"},
		{"serverStatus.wiredTiger", "ss_wt"},
		{"serverStatus.queues.execution", "ss_wt_concurrentTransactions"},
		{"serverStatus", "ss"},
		{"replSetGetStatus", "rs"},
		{"systemMetrics", "sys"},
		{"local.oplog.rs.stats.wiredTiger", "oplog_stats_wt"},
		{"local.oplog.rs.stats.storageStats.wiredTiger", "oplog_stats_wt"},
		{"local.oplog.rs.stats", "oplog_stats"},
		{"local.oplog.rs.stats.storageStats", "oplog_stats"},
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
	// the value the label will have is the field name in the structure. Example.
	//
	//   mongodb_ss_opcounters{legacy_op_type="insert"} 4
	//   mongodb_ss_opcounters{legacy_op_type="query"} 2118
	//   mongodb_ss_opcounters{legacy_op_type="update"} 14
	//   mongodb_ss_opcounters{legacy_op_type="delete"} 22
	//   mongodb_ss_opcounters{legacy_op_type="getmore"} 9141
	//   mongodb_ss_opcounters{legacy_op_type="command"} 67923
	//
	nodeToPDMetrics = map[string]string{
		"collStats.storageStats.indexDetails.":                   "index_name",
		"globalLock.activeQueue.":                                "count_type",
		"globalLock.locks.":                                      "lock_type",
		"serverStatus.asserts.":                                  "assert_type",
		"serverStatus.connections.":                              "conn_type",
		"serverStatus.globalLock.currentQueue.":                  "count_type",
		"serverStatus.metrics.commands.":                         "cmd_name",
		"serverStatus.metrics.cursor.open.":                      "csr_type",
		"serverStatus.metrics.document.":                         "doc_op_type",
		"serverStatus.opLatencies.":                              "op_type",
		"serverStatus.opReadConcernCounters.":                    "concern_type",
		"serverStatus.opcounters.":                               "legacy_op_type",
		"serverStatus.opcountersRepl.":                           "legacy_op_type",
		"serverStatus.transactions.commitTypes.":                 "commit_type",
		"serverStatus.wiredTiger.concurrentTransactions.":        "txn_rw_type",
		"serverStatus.queues.execution.":                         "txn_rw_type",
		"serverStatus.wiredTiger.perf.":                          "perf_bucket",
		"systemMetrics.disks.":                                   "device_name",
		"collstats.storageStats.indexSizes.":                     "index_name",
		"config.transactions.stats.storageStats.indexSizes.":     "index_name",
		"config.image_collection.stats.storageStats.indexSizes.": "index_name",
	}

	// This map is used to add labels to some specific metrics.
	// The difference from the case above that it works with middle nodes in the structure.
	// For example, the fields under the storageStats.indexDetails. structure have this
	// signature:
	//
	//    "storageStats": primitive.M{
	//        "indexDetails": primitive.M{
	//            "_id_": primitive.M{
	//                "LSM": primitive.M{
	//                    "bloom filter false positives": int32(0),
	//                    "bloom filter hits":            int32(0),
	//                    "bloom filter misses":          int32(0),
	// ...
	//                },
	//				"block-manager": primitive.M{
	//                    "allocations requiring file extension": int32(0),
	// ...
	//                },
	// ...
	//            },
	//            "name_1": primitive.M{
	// ...
	//            },
	// ...
	//        },
	//    },
	//
	// Applying the renaming rules, storageStats will become storageStats but instead of having metrics
	// with the form storageStats.indexDetails.<index_name>.<metric_name> where index_name is each one of
	// the fields inside the structure (_id_, name_1, etc), those keys will become labels for the same
	// metric name. The label name is defined as the value for each metric name in the map and the value
	// the label will have is the field name in the structure. Example.
	//
	//   mongodb_storageStats_indexDetails_index_name_LSM_bloom_filter_false_positives{index_name="_id_"} 0
	keyNodesToLabels = map[string]string{
		"storageStats.indexDetails.":                               "index_name",
		"config.image_collection.stats.storageStats.indexDetails.": "index_name",
		"config.transactions.stats.storageStats.indexDetails.":     "index_name",
		"collstats.storageStats.indexDetails.":                     "index_name",
	}

	// Regular expressions used to make the metric name Prometheus-compatible
	// This variables are global to compile the regexps only once.
	specialCharsRe        = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	repeatedUnderscoresRe = regexp.MustCompile(`__+`)
	dollarRe              = regexp.MustCompile(`\_$`)
)

var prometheusizeCache = sync.Map{}

// prometheusize renames metrics by replacing some prefixes with shorter names
// replace special chars to follow Prometheus metric naming rules and adds the
// exporter name prefix.
func prometheusize(s string) string {
	if renamed, exists := prometheusizeCache.Load(s); exists {
		return renamed.(string)
	}
	backup := strings.Clone(s)

	for _, pair := range prefixes {
		if strings.HasPrefix(s, pair[0]+".") {
			s = pair[1] + strings.TrimPrefix(s, pair[0])
			break
		}
	}

	s = specialCharsRe.ReplaceAllString(s, "_")
	s = dollarRe.ReplaceAllString(s, "")
	s = repeatedUnderscoresRe.ReplaceAllString(s, "_")
	s = strings.TrimPrefix(s, "_")
	s = exporterPrefix + s

	prometheusizeCache.Store(backup, strings.Clone(s))

	return s
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
func makeRawMetric(prefix, name string, value any, labels map[string]string) (*rawMetric, error) {
	f, err := asFloat64(value)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, nil
	}

	help := metricHelp(prefix, name)

	fqName, label := nameAndLabel(prefix, name)

	metricType := prometheus.UntypedValue

	// reservedPrefixes are used for metrics that might include the word ‘count’ in their name but are not actual counters.
	reservedPrefixes := []string{"collstats.storageStats.indexSizes."}
	if !slices.Contains(reservedPrefixes, prefix) && strings.HasSuffix(strings.ToLower(name), "count") {
		metricType = prometheus.CounterValue
	}

	rm := &rawMetric{
		fqName: fqName,
		help:   help,
		val:    *f,
		vt:     metricType,
		ln:     make([]string, 0, len(labels)),
		lv:     make([]string, 0, len(labels)),
	}

	// Add original labels to the metric
	for k, v := range labels {
		rm.ln = append(rm.ln, k)
		rm.lv = append(rm.lv, v)
	}

	// Add predefined label, if any
	if label != "" {
		rm.ln = append(rm.ln, label)
		rm.lv = append(rm.lv, name)
	}

	return rm, nil
}

func asFloat64(value any) (*float64, error) {
	var f float64
	switch v := value.(type) {
	case bool:
		if v {
			f = 1
		}
	case int:
		f = float64(v)
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
	case primitive.Timestamp:
		f = float64(v.T)
	case primitive.A, primitive.ObjectID, primitive.Binary, string, []uint8, time.Time:
		return nil, nil
	default:
		return nil, errors.Wrapf(errCannotHandleType, "%T", v)
	}
	return &f, nil
}

func rawToPrometheusMetric(rm *rawMetric) (prometheus.Metric, error) {
	d := prometheus.NewDesc(rm.fqName, rm.help, rm.ln, nil)
	return prometheus.NewConstMetric(d, rm.vt, rm.val, rm.lv...)
}

// metricHelp builds the metric help.
// It is a very simple function, but the idea is if the future we want
// to improve the help somehow, there is only one place to change it for the real
// functions and for all the tests.
// Use only prefix or name but not both because 2 metrics cannot have same name but different help.
// For metrics where we labelize some keys, if we put the real metric name here it will be rejected
// by prometheus. For first level metrics, there is no prefix so we should use the metric name or
// the help would be empty.
func metricHelp(prefix, name string) string {
	if _, ok := nodeToPDMetrics[prefix]; ok {
		return strings.TrimSuffix(prefix, ".")
	}
	if prefix != "" {
		return normalizeSpaces(prefix + name)
	}

	return normalizeSpaces(name)
}

// normalizeSpaces collapses whitespace runs and trims the edges. prometheusize maps keys
// differing only in whitespace to the same metric name, so the help has to be normalized
// as well or the registry rejects the descriptors as inconsistent.
func normalizeSpaces(s string) string {
	if !hasRedundantSpaces(s) {
		return s
	}

	return strings.Join(strings.Fields(s), " ")
}

func hasRedundantSpaces(s string) bool {
	previousIsSpace := true
	for _, r := range s {
		isSpace := unicode.IsSpace(r)
		if isSpace && previousIsSpace {
			return true
		}
		previousIsSpace = isSpace
	}

	return previousIsSpace && s != ""
}

func makeMetrics(prefix string, m bson.M, labels map[string]string, compatibleMode bool) []prometheus.Metric {
	return makeMetricsWithHistograms(prefix, m, labels, compatibleMode, false)
}

func makeMetricsWithHistograms(prefix string, m bson.M, labels map[string]string, compatibleMode, includeHistograms bool) []prometheus.Metric {
	var res []prometheus.Metric

	if prefix != "" {
		prefix += "."
	}

	collidingKeys := collidingKeyIndexes(m)

	for k, val := range m {
		nextPrefix := prefix + k
		if !includeHistograms && isHistogramPath(nextPrefix) {
			continue
		}

		l := make(map[string]string)
		if label, ok := keyNodesToLabels[prefix]; ok {
			maps.Copy(l, labels)
			l[label] = k
			nextPrefix = prefix + label
		} else {
			l = labels
		}
		if idx, ok := collidingKeys[k]; ok {
			l = withCollisionIndex(l, idx)
		}
		switch v := val.(type) {
		case bson.M:
			res = append(res, makeMetricsWithHistograms(nextPrefix, v, l, compatibleMode, includeHistograms)...)
		case map[string]any:
			res = append(res, makeMetricsWithHistograms(nextPrefix, v, l, compatibleMode, includeHistograms)...)
		case primitive.A:
			res = append(res, processMetricSlice(nextPrefix, v, l, compatibleMode, includeHistograms)...)
		case []any:
			if isHistogramBucketSlice(nextPrefix, v) {
				res = append(res, processHistogramSlice(nextPrefix, v, l, compatibleMode)...)
			}
			continue
		default:
			res = appendMetricValue(res, prefix, k, v, l, compatibleMode)
		}
	}

	return res
}

// collidingKeyIndexes returns an index for every key of a document that is not unique once
// sanitized. MongoDB can report several fields whose names differ only in characters that
// prometheusize collapses, for example the "giant hdr" counters that ethtool exposes per
// queue with a different amount of leading spaces. Without the index they would be exported
// as the very same series and the registry would reject the whole scrape. Keys are sorted
// because the BSON document order is lost when it is decoded into a map, and the index has
// to stay stable between scrapes.
func collidingKeyIndexes(m bson.M) map[string]int {
	if len(m) <= 1 {
		return nil
	}

	firstKeyByMetricName := make(map[string]string, len(m))
	var collidingKeys map[string][]string

	for k := range m {
		metricName := sanitizeKey(k)

		first, seen := firstKeyByMetricName[metricName]
		if !seen {
			firstKeyByMetricName[metricName] = k

			continue
		}

		if collidingKeys == nil {
			collidingKeys = make(map[string][]string, 1)
		}
		if _, started := collidingKeys[metricName]; !started {
			collidingKeys[metricName] = []string{first}
		}
		collidingKeys[metricName] = append(collidingKeys[metricName], k)
	}

	if collidingKeys == nil {
		return nil
	}

	indexes := make(map[string]int)
	for _, keys := range collidingKeys {
		slices.Sort(keys)
		for idx, k := range keys {
			indexes[k] = idx
		}
	}

	return indexes
}

func withCollisionIndex(labels map[string]string, idx int) map[string]string {
	distinct := make(map[string]string, len(labels)+1)
	maps.Copy(distinct, labels)
	distinct[collisionLabel] = strconv.Itoa(idx)

	return distinct
}

var sanitizedKeyCache = sync.Map{}

// sanitizeKey turns a document field into the part of the metric name it ends up in.
// The result is cached because the set of field names MongoDB reports is small and the
// collision check walks all of them on every scrape.
func sanitizeKey(k string) string {
	if sanitized, ok := sanitizedKeyCache.Load(k); ok {
		if sanitizedString, ok := sanitized.(string); ok {
			return sanitizedString
		}
	}

	sanitized := specialCharsRe.ReplaceAllString(k, "_")
	sanitizedKeyCache.Store(strings.Clone(k), sanitized)

	return sanitized
}

// Extract maps from arrays. Only some structures like replicasets have arrays of members
// and each member is represented by a map[string]any.
func processSlice(prefix string, v []any, commonLabels map[string]string, compatibleMode, includeHistograms bool) []prometheus.Metric {
	metrics := make([]prometheus.Metric, 0)
	labels := make(map[string]string)
	maps.Copy(labels, commonLabels)

	for _, item := range v {
		s, ok := asMetricMap(item)
		if !ok {
			continue
		}

		// use the replicaset or server name as a label
		if name, ok := s["name"].(string); ok {
			labels["member_idx"] = name
		}
		if state, ok := s["stateStr"].(string); ok {
			labels["member_state"] = state
		}
		if host, ok := s["host"].(string); ok {
			labels["member_idx"] = host
		}

		metrics = append(metrics, makeMetricsWithHistograms(prefix, s, labels, compatibleMode, includeHistograms)...)
	}

	return metrics
}

func processMetricSlice(prefix string, v []any, commonLabels map[string]string, compatibleMode, includeHistograms bool) []prometheus.Metric {
	if isHistogramBucketSlice(prefix, v) {
		return processHistogramSlice(prefix, v, commonLabels, compatibleMode)
	}

	return processSlice(prefix, v, commonLabels, compatibleMode, includeHistograms)
}

func appendMetricValue(metrics []prometheus.Metric, prefix, name string, value any, labels map[string]string, compatibleMode bool) []prometheus.Metric {
	rm, err := makeRawMetric(prefix, name, value, labels)
	if err != nil {
		invalidMetric := prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
		return append(metrics, invalidMetric)
	}

	// makeRawMetric returns a nil metric for some data types like strings
	// because we cannot extract data from all types
	if rm == nil {
		return metrics
	}

	rawMetrics := []*rawMetric{rm}
	if renamedMetrics := metricRenameAndLabel(rm, specialConversions); renamedMetrics != nil {
		rawMetrics = renamedMetrics
	}

	for _, rawMetric := range rawMetrics {
		metric, err := rawToPrometheusMetric(rawMetric)
		if err != nil {
			invalidMetric := prometheus.NewInvalidMetric(prometheus.NewInvalidDesc(err), err)
			metrics = append(metrics, invalidMetric)
			continue
		}

		metrics = append(metrics, metric)
		if compatibleMode {
			metrics = appendCompatibleMetric(metrics, rawMetric)
		}
	}

	return metrics
}

func processHistogramSlice(prefix string, v []any, commonLabels map[string]string, compatibleMode bool) []prometheus.Metric {
	metrics := make([]prometheus.Metric, 0, len(v))
	for _, item := range v {
		bucket, ok := asMetricMap(item)
		if !ok {
			continue
		}

		boundKey, boundLabel, ok := histogramBound(bucket)
		if !ok {
			continue
		}

		labels := make(map[string]string, len(commonLabels)+1)
		for name, value := range commonLabels {
			labels[name] = value
		}

		labels[boundLabel] = fmt.Sprint(bucket[boundKey])
		metrics = appendMetricValue(metrics, prefix+".", "count", bucket["count"], labels, compatibleMode)
	}

	return metrics
}

// histogramBound returns the field holding the bucket boundary and the label exposing it.
// getDiagnosticData uses two shapes: {lowerBound, count} under "histograms" nodes and
// {micros, count} under the "histogram" arrays of serverStatus.opLatencies.
func histogramBound(bucket map[string]any) (string, string, bool) {
	for _, bound := range []struct{ key, label string }{
		{histogramLowerBoundKey, "lower_bound"},
		{histogramMicrosKey, histogramMicrosKey},
	} {
		if _, ok := bucket[bound.key]; ok {
			return bound.key, bound.label, true
		}
	}

	return "", "", false
}

func isHistogramBucketSlice(prefix string, v []any) bool {
	if len(v) == 0 {
		return false
	}
	if !isHistogramPath(prefix) {
		return false
	}

	for _, item := range v {
		bucket, ok := asMetricMap(item)
		if !ok {
			return false
		}
		if _, _, ok := histogramBound(bucket); !ok {
			return false
		}
		if _, ok := bucket["count"]; !ok {
			return false
		}
	}

	return true
}

func isHistogramPath(prefix string) bool {
	return isPathNode(prefix, "histograms") || isPathNode(prefix, "histogram")
}

func isPathNode(prefix, node string) bool {
	return prefix == node || strings.Contains(prefix, "."+node+".") || strings.HasSuffix(prefix, "."+node)
}

func asMetricMap(item any) (map[string]any, bool) {
	switch value := item.(type) {
	case map[string]any:
		return value, true
	case primitive.M:
		return map[string]any(value), true
	default:
		return nil, false
	}
}

type conversion struct {
	newName               string
	oldName               string
	labelConversions      map[string]string // key: current label, value: old exporter (compatible) label
	labelValueConversions map[string]string // key: current label, value: old exporter (compatible) label
	prefix                string
	suffixLabel           string
	suffixMapping         map[string]string
}

func metricRenameAndLabel(rm *rawMetric, convs []conversion) []*rawMetric {
	// check if the metric exists in the conversions array.
	// if it exists, it should be converted.
	var result []*rawMetric
	for _, cm := range convs {
		switch {
		case cm.newName != "" && rm.fqName == cm.newName: // first renaming case. See (1)
			result = append(result, newToOldMetric(rm, cm))

		case cm.prefix != "" && strings.HasPrefix(rm.fqName, cm.prefix): // second renaming case. See (2)
			conversionSuffix := strings.TrimPrefix(rm.fqName, cm.prefix)
			conversionSuffix = strings.TrimPrefix(conversionSuffix, "_")

			// Check that also the suffix matches.
			// In the conversion array, there are metrics with the same prefix but the 'old' name varies
			// also depending on the metic suffix
			if _, ok := cm.suffixMapping[conversionSuffix]; ok {
				om := createOldMetricFromNew(rm, cm)
				result = append(result, om)
			}
		}
	}

	return result
}

// specialConversions returns a list of special conversions we want to implement.
// See: https://jira.percona.com/browse/PMM-6506
var specialConversions = []conversion{ //nolint:gochecknoglobals
	{
		oldName:     "mongodb_ss_opLatencies_ops",
		prefix:      "mongodb_ss_opLatencies",
		suffixLabel: "op_type",
		suffixMapping: map[string]string{
			"commands_ops":     "commands",
			"reads_ops":        "reads",
			"transactions_ops": "transactions",
			"writes_ops":       "writes",
		},
	},
	{
		oldName:     "mongodb_ss_opLatencies_latency",
		prefix:      "mongodb_ss_opLatencies",
		suffixLabel: "op_type",
		suffixMapping: map[string]string{
			"commands_latency":     "commands",
			"reads_latency":        "reads",
			"transactions_latency": "transactions",
			"writes_latency":       "writes",
		},
	},
	// mongodb_ss_wt_concurrentTransactions_read_out
	// mongodb_ss_wt_concurrentTransactions_write_out
	{
		oldName:     "mongodb_ss_wt_concurrentTransactions_out",
		prefix:      "mongodb_ss_wt_concurrentTransactions",
		suffixLabel: "txn_rw",
		suffixMapping: map[string]string{
			"read_out":  "read",
			"write_out": "write",
		},
	},
	// mongodb_ss_wt_concurrentTransactions_read_available
	// mongodb_ss_wt_concurrentTransactions_write_available
	{
		oldName:     "mongodb_ss_wt_concurrentTransactions_available",
		prefix:      "mongodb_ss_wt_concurrentTransactions",
		suffixLabel: "txn_rw",
		suffixMapping: map[string]string{
			"read_available":  "read",
			"write_available": "write",
		},
	},
	// mongodb_ss_wt_concurrentTransactions_read_totalTickets
	// mongodb_ss_wt_concurrentTransactions_write_totalTickets
	{
		oldName:     "mongodb_ss_wt_concurrentTransactions_totalTickets",
		prefix:      "mongodb_ss_wt_concurrentTransactions",
		suffixLabel: "txn_rw",
		suffixMapping: map[string]string{
			"read_totalTickets":  "read",
			"write_totalTickets": "write",
		},
	},
}
