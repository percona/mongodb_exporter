package exporter

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/percona/exporter_shared/helpers"
	"github.com/prometheus/client_golang/prometheus"
)

func collect(c helpers.Collector) []prometheus.Metric {
	m := make([]prometheus.Metric, 0)
	ch := make(chan prometheus.Metric)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for metric := range ch {
			m = append(m, metric)
		}
		wg.Done()
	}()

	c.Collect(ch)
	close(ch)

	wg.Wait()

	return m
}

// zeroMetrics returns a copy of the input with all values set to 0.
// The idea is to be able to compare metric names, help and labels but since values
// are not constant, all of them are being set to 0 to compare all other fields.
func zeroMetrics(metrics []*helpers.Metric) []*helpers.Metric {
	res := make([]*helpers.Metric, 0, len(metrics))

	for _, m := range metrics {
		m.Value = 0
		res = append(res, m)
	}

	return res
}

func readTestData(filename string, destination interface{}) error {
	buf, err := ioutil.ReadFile(filepath.Clean(filename))
	if err != nil {
		return err
	}

	err = json.Unmarshal(buf, &destination)
	if err != nil {
		return err
	}

	return nil
}

func writeTestDataJSON(filename string, data interface{}) error {
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, buf, os.ModePerm)
}
