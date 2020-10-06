package exporter

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// getMetricNames returns a map of metric names so it is easier to compare
// which metrics exists against a predefined list.
func getMetricNames(lines []string) map[string]bool {
	names := map[string]bool{}

	for _, line := range lines {
		if strings.HasPrefix(line, "# TYPE ") {
			m := strings.Split(line, " ")
			names[m[2]] = true
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

func inGithubActions() bool {
	return os.Getenv("GITHUB_ACTION") != ""
}
