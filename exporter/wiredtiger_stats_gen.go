// mongodb_exporter
// Copyright (C) 2026 Percona LLC
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

//go:build ignore

// This program generates wiredtiger_stats.go, the list of WiredTiger statistics and whether
// each one is a counter or a gauge. It reads dist/stat_data.py, the file WiredTiger generates
// its statistics code from, on every branch MongoDB ships.
//
// Run it with go generate ./exporter/... whenever a new MongoDB release is supported.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

// wiredTigerBranches are the WiredTiger branches vendored by the MongoDB versions we support.
var wiredTigerBranches = []string{
	"mongodb-4.4",
	"mongodb-5.0",
	"mongodb-6.0",
	"mongodb-7.0",
	"mongodb-8.0",
	"mongodb-8.2",
	"develop",
}

const statDataURL = "https://raw.githubusercontent.com/wiredtiger/wiredtiger/%s/dist/stat_data.py"

var (
	classRe = regexp.MustCompile(`class (\w+)Stat\(Stat\):\s*prefix = '([^']*)'`)
	statRe  = regexp.MustCompile(`(\w+)Stat\(\s*'[^']*'\s*,\s*'([^']*)'\s*(?:,\s*'([^']*)')?`)

	// Almost all no_clear and no_scale statistics are point-in-time values, but these are
	// running totals WiredTiger keeps for the lifetime of the connection.
	cumulativeRe = regexp.MustCompile(`(` +
		` calls| calls due to cache pressure|(^| )total time \(msecs\)| cumulative` +
		`|deferred operations applied|unchanged and skipped|conflicted with checkpoint` +
		`|pulled into eviction|would not reduce file size|compact timeout` +
		`|checked against the data store|compact interrupted|compact recovered bytes` +
		`|sleeps due to cache pressure)$` +
		`|^background compact skipped`)

	// WiredTiger sets these statistics to a point-in-time value (the size of a cache, a
	// threshold, what the most recent checkpoint or step-up did), but doesn't flag them
	// no_clear or no_scale and doesn't say "currently" in their description.
	pointInTimeRe = regexp.MustCompile(`^(` +
		`backup granularity size` +
		`|(block cache )?file size causing bypass` +
		`|(block cache )?total (blocks|bytes)` +
		`|eviction threshold .+ multiplied by 100 for precision` +
		`|threshold to call fsync` +
		`|(transaction checkpoint )?most recent (handles|in-memory pages) .+` +
		`|checkpoint has acquired a snapshot for its transaction` +
		`|time spent on per-tree checkpoint work \(usecs\)` +
		`|total time \(msecs\) writing pages to stable storage during checkpoint reconciliation` +
		`|transaction checkpoint history store file duration \(usecs\)` +
		`|total (bytes used by the cache( for pinned chunks)?|(pinned )?chunks held by the chunk cache)` +
		`|.+ most recent time \(msecs\)` +
		`|database size|role leader` +
		`|pick up checkpoint time at startup \(msecs\)` +
		`|step up (deferred checkpoint pickup retries before stepping up|ingest table drain bytes moved to stable tables` +
		`|ingest tables drained|missing stable tables created)` +
		`|the number of tables the layered table manager has open` +
		`|average length of delta chain on (internal|leaf) page with deltas` +
		`|max deltas seen on (internal|leaf) page during reconciliation` +
		`|table compact passes` +
		`|active filesystem (fsync|read|write) calls` +
		`)$`)
)

func main() {
	gauges := map[string]bool{}
	counters := map[string]bool{}

	for _, branch := range wiredTigerBranches {
		src, err := fetch(fmt.Sprintf(statDataURL, branch))
		if err != nil {
			log.Fatalf("cannot read the statistics of WiredTiger %s: %s", branch, err)
		}

		sections := map[string]string{}
		for _, m := range classRe.FindAllStringSubmatch(src, -1) {
			sections[m[1]] = m[2]
		}

		for _, m := range statRe.FindAllStringSubmatch(src, -1) {
			section, ok := sections[m[1]]
			if !ok {
				continue
			}

			desc := strings.TrimSpace(m[2])
			name := section + ": " + desc
			if isGauge(desc, m[3]) {
				gauges[name] = true
			} else {
				counters[name] = true
			}
		}
	}

	// A statistic WiredTiger reports as a gauge on any branch is a gauge: calling a gauge a
	// counter breaks rate() and OpenTelemetry conversions, the other way round only loses a hint.
	for name := range gauges {
		delete(counters, name)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `// mongodb_exporter
// Copyright (C) %d Percona LLC
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

// Code generated by wiredtiger_stats_gen.go; DO NOT EDIT.

package exporter

// WiredTiger statistics, written "<section>: <description>" as WiredTiger reports them, read
// from dist/stat_data.py on the WiredTiger branches %s.
`, time.Now().Year(), strings.Join(wiredTigerBranches, ", "))

	writeList(&buf, "wiredTigerCounters", counters)
	writeList(&buf, "wiredTigerGauges", gauges)

	out, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("cannot format the generated code: %s", err)
	}

	if err := os.WriteFile("wiredtiger_stats.go", out, 0o644); err != nil { //nolint:gosec
		log.Fatal(err)
	}
}

// isGauge reports whether a statistic holds a point-in-time value. WiredTiger flags those
// no_clear (not reset with the other statistics) or no_scale (not turned into a rate per second
// by its tools), and its convention is to put "currently" in the description of any other.
func isGauge(desc, flags string) bool {
	if strings.Contains(desc, "currently") || pointInTimeRe.MatchString(desc) {
		return true
	}

	if !strings.Contains(flags, "no_clear") && !strings.Contains(flags, "no_scale") {
		return false
	}

	return !cumulativeRe.MatchString(desc)
}

func writeList(buf *bytes.Buffer, name string, set map[string]bool) {
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	slices.Sort(names)

	fmt.Fprintf(buf, "\nvar %s = []string{\n", name)
	for _, n := range names {
		fmt.Fprintf(buf, "\t%q,\n", n)
	}
	fmt.Fprintf(buf, "}\n")
}

func fetch(url string) (string, error) {
	resp, err := http.Get(url) //nolint:gosec,noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)

	return string(body), err
}
