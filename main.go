package main

import (
	"github.com/Percona-Lab/mnogo_exporter/exporter"
)

func main() {
	e, err := exporter.New(nil)
	if err != nil {
		panic(err)
	}
	_ = e
}
