package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/andreyvit/diff"
	pmmVersion "github.com/percona/pmm/version"
	"github.com/prometheus/common/version"
	"github.com/stretchr/testify/assert"

	"github.com/percona/mongodb_exporter/shared"
)

var Update = flag.Bool("update", false, "update .golden files")

// bin stores information about path of executable and attached port
type bin struct {
	path string
	port int
}

// TestBin builds, runs and tests binary.
func TestBin(t *testing.T) {
	if testing.Short() {
		t.Skip("-short is passed, skipping integration test")
	}

	var err error
	binName := "mongodb_exporter"

	binDir, err := ioutil.TempDir("/tmp", binName+"-test-bindir-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(binDir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	importpath := "github.com/percona/mongodb_exporter/vendor/github.com/percona/pmm"
	path := binDir + "/" + binName
	xVariables := map[string]string{
		importpath + "/version.Version":    "gotest-version",
		importpath + "/version.Branch":     "gotest-branch",
		importpath + "/version.FullCommit": "gotest-revision",
	}
	var ldflags []string
	for x, value := range xVariables {
		ldflags = append(ldflags, fmt.Sprintf("-X %s=%s", x, value))
	}
	cmd := exec.Command(
		"go",
		"build",
		"-o",
		path,
		"-ldflags",
		strings.Join(ldflags, " "),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed to build: %s", err)
	}

	tests := []func(*testing.T, bin){
		testFlagHelp,
		testFlagTest,
		testFlagTestWithTLS,
		testFlagVersion,
		testLandingPage,
		testDefaultGatherer,
		testBuildVersionGatherer,
	}

	portStart := 56000
	t.Run(binName, func(t *testing.T) {
		for _, f := range tests {
			f := f // capture range variable
			fName := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			portStart++
			data := bin{
				path: path,
				port: portStart,
			}
			t.Run(fName, func(t *testing.T) {
				t.Parallel()
				f(t, data)
			})
		}
	})
}

func testFlagHelp(t *testing.T, data bin) {
	cmd := exec.Command(
		data.path,
		"--help",
	)

	output, _ := cmd.CombinedOutput()
	output = regexp.MustCompile(regexp.QuoteMeta(data.path)).ReplaceAll(output, []byte("mongodb_exporter"))
	actual := string(output)

	filename := path.Join("testdata", path.Base(t.Name())+".golden")
	if *Update {
		err := ioutil.WriteFile(filename, output, 0600)
		assert.NoError(t, err)
	}
	b, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)
	expected := string(b)

	if actual != expected {
		t.Errorf("diff:\n%s", diff.LineDiff(expected, actual))
	}
}

func testFlagVersion(t *testing.T, data bin) {
	// TODO: Doesn't work with go 1.13+. Should be refactored.
	t.Skip()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--version",
		"--web.listen-address", fmt.Sprintf(":%d", data.port),
	)

	b := &bytes.Buffer{}
	cmd.Stdout = b
	cmd.Stderr = b

	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	expectedRegexp := `mongodb_exporter, version gotest-version \(branch: gotest-branch, revision: gotest-revision\)
  build user:
  build date:
  go version:
`

	expectedScanner := bufio.NewScanner(bytes.NewBufferString(expectedRegexp))
	defer func() {
		if err := expectedScanner.Err(); err != nil {
			t.Fatal(err)
		}
	}()

	gotScanner := bufio.NewScanner(b)
	defer func() {
		if err := gotScanner.Err(); err != nil {
			t.Fatal(err)
		}
	}()

	for gotScanner.Scan() {
		if !expectedScanner.Scan() {
			t.Fatalf("didn't expected more data but got '%s'", gotScanner.Text())
		}
		ok, err := regexp.MatchString(expectedScanner.Text(), gotScanner.Text())
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("'%s' does not match regexp '%s'", gotScanner.Text(), expectedScanner.Text())
		}
	}

	if expectedScanner.Scan() {
		t.Errorf("expected '%s' but didn't got more data", expectedScanner.Text())
	}
}

func testLandingPage(t *testing.T, data bin) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Run exporter.
	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--web.listen-address", fmt.Sprintf(":%d", data.port),
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	// Get the main page.
	urlToGet := fmt.Sprintf("http://127.0.0.1:%d", data.port)
	body, err := waitForBody(urlToGet)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)

	expected := `<html>
<head>
	<title>MongoDB exporter</title>
</head>
<body>
	<h1>MongoDB exporter</h1>
	<p><a href="/metrics">Metrics</a></p>
</body>
</html>`
	assert.Equal(t, expected, got)
}

func testDefaultGatherer(t *testing.T, data bin) {
	metricPath := "/metrics"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--web.telemetry-path", metricPath,
		"--web.listen-address", fmt.Sprintf(":%d", data.port),
	)

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	body, err := waitForBody(fmt.Sprintf("http://127.0.0.1:%d%s", data.port, metricPath))
	if err != nil {
		t.Fatalf("unable to get metrics: %s", err)
	}
	got := string(body)

	metricsPrefixes := []string{
		"go_gc_duration_seconds",
		"go_goroutines",
		"go_memstats",
	}

	for _, prefix := range metricsPrefixes {
		if !strings.Contains(got, prefix) {
			t.Fatalf("no metric starting with %s", prefix)
		}
	}
}

func testBuildVersionGatherer(t *testing.T, data bin) {
	metricPath := "/metrics"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--web.telemetry-path", metricPath,
		"--web.listen-address", fmt.Sprintf(":%d", data.port),
	)

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()
	defer cmd.Process.Kill()

	body, err := waitForBody(fmt.Sprintf("http://127.0.0.1:%d%s", data.port, metricPath))
	if err != nil {
		t.Fatalf("unable to get metrics: %s", err)
	}
	got := string(body)

	metricsPrefixes := []string{
		"mongodb_exporter_build_info",
	}

	for _, prefix := range metricsPrefixes {
		if !strings.Contains(got, prefix) {
			t.Fatalf("no metric starting with %s", prefix)
		}
	}
}

func testFlagTest(t *testing.T, data bin) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--test",
	)

	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Log(string(b))
		t.Fatal(err)
	}
	buildInfo := shared.BuildInfo{}
	err = json.Unmarshal(b, &buildInfo)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(buildInfo, shared.BuildInfo{}) {
		t.Fatalf("buildInfo is empty")
	}
}

func testFlagTestWithTLS(t *testing.T, data bin) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tlsCertificateKeyFile := "testdata/client.pem"
	tlsCAFile := "testdata/ca.crt"

	cmd := exec.CommandContext(
		ctx,
		data.path,
		"--mongodb.uri=mongodb://127.0.0.1:27017/admin/?ssl=true&tlsCertificateKeyFile="+tlsCertificateKeyFile+"&tlsCAFile="+tlsCAFile+"&tlsInsecure=true&serverSelectionTimeoutMS=2000",
		"--test",
	)

	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Log(string(b))
		t.Fatal(err)
	}
	buildInfo := shared.BuildInfo{}
	err = json.Unmarshal(b, &buildInfo)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(buildInfo, shared.BuildInfo{}) {
		t.Fatalf("buildInfo is empty")
	}
}

// waitForBody is a helper function which makes http calls until http server is up
// and then returns body of the successful call.
func waitForBody(urlToGet string) (body []byte, err error) {
	tries := 60

	// Get data, but we need to wait a bit for http server.
	for i := 0; i <= tries; i++ {
		// Try to get web page.
		body, err = getBody(urlToGet)
		if err == nil {
			return body, err
		}

		// If there is a syscall.ECONNREFUSED error (web server not available) then retry.
		if urlError, ok := err.(*url.Error); ok {
			if opError, ok := urlError.Err.(*net.OpError); ok {
				if osSyscallError, ok := opError.Err.(*os.SyscallError); ok {
					if osSyscallError.Err == syscall.ECONNREFUSED {
						time.Sleep(1 * time.Second)
						continue
					}
				}
			}
		}

		// There was an error, and it wasn't syscall.ECONNREFUSED.
		return nil, err
	}

	return nil, fmt.Errorf("failed to GET %s after %d tries: %s", urlToGet, tries, err)
}

// getBody is a helper function which retrieves http body from given address.
func getBody(urlToGet string) ([]byte, error) {
	resp, err := http.Get(urlToGet)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func TestVersionInfo(t *testing.T) {
	t.Run("Check version parameters", func(t *testing.T) {
		var currentTime = time.Now()
		pmmVersion.Version = "1.2.3"
		pmmVersion.PMMVersion = "4.5.6"
		pmmVersion.FullCommit = "test-commit-sha"
		pmmVersion.Branch = "test-branch"
		pmmVersion.Timestamp = strconv.FormatInt(currentTime.Unix(), 10)

		initVersionInfo()

		assert.Equal(t, pmmVersion.Version+"-pmm-"+pmmVersion.PMMVersion, version.Version)
		assert.Equal(t, pmmVersion.FullCommit, version.Revision)
		assert.Equal(t, pmmVersion.Branch, version.Branch)
		assert.Equal(t, currentTime.Format(versionDataFormat), version.BuildDate)
	})

	t.Run("Check Empty Timestamp", func(t *testing.T) {
		pmmVersion.Timestamp = ""
		initVersionInfo()
		assert.Equal(t, time.Unix(0, 0).Format(versionDataFormat), version.BuildDate)
	})

	t.Run("Check PMMVersion Empty", func(t *testing.T) {
		pmmVersion.Version = "1.2.3"
		pmmVersion.PMMVersion = ""
		initVersionInfo()
		assert.Equal(t, pmmVersion.Version, version.Version)
	})
}
