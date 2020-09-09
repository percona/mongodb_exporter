package exporter

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
)

func collect(c prometheus.Collector) []prometheus.Metric {
	m := []prometheus.Metric{}
	ch := make(chan prometheus.Metric)

	go func() {
		for metric := range ch {
			m = append(m, metric)
		}
	}()

	c.Collect(ch)

	return m
}

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

func readTestMetrics(filename string) ([]*helpers.Metric, error) {
	m := []*helpers.Metric{}

	buf, err := ioutil.ReadFile(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func writeTestDataJSON(filename string, data interface{}) error {
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, buf, os.ModePerm)
}
