// mongodb_exporter
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

//go:build ignore
// +build ignore

// check-license checks that AGPL license header in all files matches header in this file.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

func getHeader(startYear int) string {
	header := fmt.Sprintf(`// mongodb_exporter
// Copyright (C) %d Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.
`, startYear)
	return header
}

var generatedHeader = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.`)

// startYear represents the oldest copyright year present in the project
var startYear = 2017

func checkHeader(path string, startYear int) bool {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	actual := make([]byte, len(getHeader(startYear)))
	_, err = io.ReadFull(f, actual)
	if err == io.ErrUnexpectedEOF {
		err = nil // some files are shorter than license header
	}
	if err != nil {
		log.Printf("%s - %s", path, err)
		return false
	}

	if generatedHeader.Match(actual) {
		return true
	}

	for year := startYear; year <= time.Now().Year(); year++ {
		if string(actual) == getHeader(year) {
			return true
		}
	}
	log.Println(path)
	return false
}

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: go run .github/check-license.go")
		flag.CommandLine.PrintDefaults()
	}
	flag.Parse()

	ok := true
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}

		if filepath.Ext(info.Name()) == ".go" {
			if !checkHeader(path, startYear) {
				ok = false
			}
		}
		return nil
	})

	if ok {
		os.Exit(0)
	}
	log.Print("Please update license header in those files.")
	os.Exit(1)
}
