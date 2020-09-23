package exporter

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/percona/exporter_shared/helpers"
	"github.com/pkg/errors"
)

func filterMetrics(metrics []*helpers.Metric, filters []string) []*helpers.Metric {
	res := make([]*helpers.Metric, 0, len(metrics))

	for _, m := range metrics {
		m.Value = 0
		for _, filterName := range filters {
			if m.Name == filterName {
				res = append(res, m)

				break
			}
		}
	}

	return res
}

func zeroMetrics(metrics []*helpers.Metric) []*helpers.Metric {
	res := make([]*helpers.Metric, 0, len(metrics))

	for _, m := range metrics {
		m.Value = 0
		res = append(res, m)
	}

	return res
}

func getMetricNames(lines []string) []string {
	names := []string{}

	for _, line := range lines {
		if strings.HasPrefix(line, "# TYPE ") {
			m := strings.Split(line, " ")
			names = append(names, m[2])
		}
	}

	return names
}

func writeJSON(filename string, data interface{}) error {
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.Wrap(err, "cannot parse input data")
	}
	return ioutil.WriteFile(filepath.Clean(filename), buf, os.ModePerm)
}

func readJSON(filename string, data interface{}) error {
	buf, err := ioutil.ReadFile(filepath.Clean(filename))
	if err != nil {
		return errors.Wrap(err, "cannot read sample file")
	}

	return json.Unmarshal(buf, data)
}
