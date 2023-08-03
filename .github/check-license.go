// mongodb_exporter
// Copyright (C) 2017 Percona LLC
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
// +build ignore

// check-license checks that Apache 2.0 header in all files matches header in this file.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

var (
	generatedHeader = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.`)

	copyrightText = `// mongodb_exporter
// Copyright (C) 2022 Percona LLC
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
`

	copyrightPattern = regexp.MustCompile(`^// mongodb_exporter
// Copyright \(C\) 20\d{2} Percona LLC
//
// Licensed under the Apache License, Version 2\.0 \(the "License"\);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www\.apache\.org/licenses/LICENSE-2\.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied\.
// See the License for the specific language governing permissions and
// limitations under the License\.
`)
)

func checkHeader(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	actual := make([]byte, len(copyrightText))
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

	if !copyrightPattern.Match(actual) {
		log.Print(path)
		return false
	}
	return true
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
			if !checkHeader(path) {
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
