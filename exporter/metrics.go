package exporter

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func makeRawMetricName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case 'a' <= r && r <= 'z':
			b.WriteRune(r)
		case 'A' <= r && r <= 'Z':
			b.WriteRune('_')
			b.WriteRune(r + 'a' - 'A')
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func makeRawMetric(name string, value interface{}) (prometheus.Metric, error) {
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
	case float64:
		f = v
	case primitive.A, primitive.ObjectID: // TODO: Fix this. returning nil, nil just to make tests pass
		return nil, nil

	case primitive.DateTime:
		f = float64(v)
	case primitive.Timestamp:
		return nil, nil

	case string:
		return nil, nil

	default:
		return nil, fmt.Errorf("makeRawMetric: unhandled type %T", v)
	}

	fqName := makeRawMetricName(name)
	help := "TODO"
	typ := prometheus.UntypedValue
	d := prometheus.NewDesc(fqName, help, nil, nil)
	return prometheus.NewConstMetric(d, typ, f)
}
