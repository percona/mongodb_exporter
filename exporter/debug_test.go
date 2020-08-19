package exporter

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestDebug(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	olderr := os.Stderr
	r, w, _ := os.Pipe()

	os.Stderr = w
	defer func() {
		os.Stderr = olderr
		logrus.SetLevel(logrus.ErrorLevel)
	}()

	log.Out = w

	m := bson.M{
		"f1": 1,
		"f2": "v2",
		"f3": bson.M{
			"f4": 4,
		},
	}
	want := `{
  "f1": 1,
  "f2": "v2",
  "f3": {
    "f4": 4
  }
}` + "\n"

	debugResult(log, m)
	assert.NoError(t, w.Close())
	out, _ := ioutil.ReadAll(r)

	assert.Equal(t, want, string(out))
}
