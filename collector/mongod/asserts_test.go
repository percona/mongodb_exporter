package collector_mongod

import (
	"testing"
)

func Test_AssertsCollectData(t *testing.T) {
	asserts := &AssertsStats{
		Regular:   1,
		Warning:   2,
		Msg:       3,
		User:      4,
		Rollovers: 5,
	}

	asserts.Export()
}
