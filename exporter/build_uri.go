// mongodb_exporter
// Copyright (C) 2025 Percona LLC
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

package exporter

import (
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
)

func ParseURIList(uriList []string, logger *slog.Logger, splitCluster bool) []string { //nolint:gocognit,cyclop
	var URIs []string

	// If server URI is prefixed with mongodb scheme string, then every next URI in
	// line not prefixed with mongodb scheme string is a part of cluster. Otherwise,
	// treat it as a standalone server
	realURI := ""
	matchRegexp := regexp.MustCompile(`^mongodb(\+srv)?://`)
	for _, URI := range uriList {
		matches := matchRegexp.FindStringSubmatch(URI)
		if matches != nil {
			if realURI != "" {
				// Add the previous host buffer to the url list as we met the scheme part
				URIs = append(URIs, realURI)
				realURI = ""
			}
			if matches[1] == "" {
				realURI = URI
			} else {
				// There can be only one host in SRV connection string
				if splitCluster {
					// In splitCluster mode we get srv connection string from SRV recors
					URI = GetSeedListFromSRV(URI, logger)
				}
				URIs = append(URIs, URI)
			}
		} else {
			if realURI == "" {
				URIs = append(URIs, "mongodb://"+URI)
			} else {
				realURI += "," + URI
			}
		}
	}
	if realURI != "" {
		URIs = append(URIs, realURI)
	}

	if splitCluster {
		// In this mode we split cluster strings into separate targets
		separateURIs := []string{}
		for _, hosturl := range URIs {
			urlParsed, err := url.Parse(hosturl)
			if err != nil {
				log.Fatalf("Failed to parse URI %s: %v", hosturl, err)
			}
			for _, host := range strings.Split(urlParsed.Host, ",") {
				targetURI := "mongodb://"
				if urlParsed.User != nil {
					targetURI += urlParsed.User.String() + "@"
				}
				targetURI += host
				if urlParsed.Path != "" {
					targetURI += urlParsed.Path
				}
				if urlParsed.RawQuery != "" {
					targetURI += "?" + urlParsed.RawQuery
				}
				separateURIs = append(separateURIs, targetURI)
			}
		}
		return separateURIs
	}
	return URIs
}

// buildURIManually builds the URI manually by checking if the user and password are supplied
func buildURIManually(uri string, user string, password string) string {
	uriArray := strings.SplitN(uri, "://", 2) //nolint:mnd
	prefix := uriArray[0] + "://"
	uri = uriArray[1]

	// IF user@pass not contained in uri AND custom user and pass supplied in arguments
	// DO concat a new uri with user and pass arguments value
	if !strings.Contains(uri, "@") && user != "" && password != "" {
		// add user and pass to the uri
		uri = fmt.Sprintf("%s:%s@%s", user, password, uri)
	}

	// add back prefix after adding the user and pass
	uri = prefix + uri

	return uri
}

func BuildURI(uri string, user string, password string) string {
	defaultPrefix := "mongodb://" // default prefix

	if !strings.HasPrefix(uri, defaultPrefix) && !strings.HasPrefix(uri, "mongodb+srv://") {
		uri = defaultPrefix + uri
	}
	parsedURI, err := url.Parse(uri)
	if err != nil {
		// PMM generates URI with escaped path to socket file, so url.Parse fails
		// in this case we build URI manually
		return buildURIManually(uri, user, password)
	}

	if parsedURI.User == nil && user != "" && password != "" {
		parsedURI.User = url.UserPassword(user, password)
	}

	return parsedURI.String()
}
