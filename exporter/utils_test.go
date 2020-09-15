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

func filterMetrics(metrics []*helpers.Metric) []*helpers.Metric {
	res := make([]*helpers.Metric, 0, len(metrics))
	processList := make(map[string]struct{})

	for _, m := range metrics {
		m.Value = 0

		// skip processList metrics that contain process IDs in labels that change too often
		if strings.Contains(m.Name, "_processList_") {
			if _, ok := processList[m.Name]; ok {
				continue
			}
			processList[m.Name] = struct{}{}
		}

		res = append(res, m)
	}

	return res
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
