package collector

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func LoadFixture(name string) []byte {
	data, err := ioutil.ReadFile("fixtures/" + name)
	if err != nil {
		panic(err)
	}

	return data
}
