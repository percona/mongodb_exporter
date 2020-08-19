package exporter

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func debugResult(log *logrus.Logger, m interface{}) {
	if !log.IsLevelEnabled(logrus.DebugLevel) {
		return
	}

	debugStr, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		log.Errorf("cannot marshal struct for debug: %s", err)
		return
	}

	// don't use logrus because:
	// 1. It will escape new lines and " making it harder to read and to use
	// 2. It will add timestamp
	// 3. This way is easier to copy/paste to put the info in a ticket
	fmt.Fprintln(os.Stderr, string(debugStr))
}
