package shared

import (
	"testing"
)

func TestRedactMongoUri(t *testing.T) {
	uri := "mongodb://mongodb_exporter:s3cr3tpassw0rd@localhost:27017"
	expected := "mongodb://****:****@localhost:27017"
	actual := RedactMongoUri(uri)
	if expected != actual {
		t.Errorf("%q != %q", expected, actual)
	}
}
