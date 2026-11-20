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

package exporter

import (
	"log"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/percona/mongodb_exporter/internal/redact"
)

// GetSeedListFromSRV converts mongodb+srv URI to flat connection string.
func GetSeedListFromSRV(uri string, logger *slog.Logger) string { //nolint:cyclop
	uriParsed, err := url.Parse(uri)
	if err != nil {
		// The parse error is not logged: it quotes the URI cut short at the byte url.Parse choked
		// on, and when that byte is "?" or "#" from the password, the quote keeps the part before.
		log.Fatalf("Failed to parse URI %s", redact.MongoURI(uri))
	}

	cname, srvRecords, err := net.LookupSRV("mongodb", "tcp", uriParsed.Hostname())
	if err != nil {
		logger.Error("Failed to lookup SRV records", "uri", redact.MongoURI(uri), "error", err)
		return uri
	}

	if len(srvRecords) == 0 {
		logger.Error("No SRV records found", "uri", redact.MongoURI(uri))
		return uri
	}

	queryString := uriParsed.RawQuery

	txtRecords, err := net.LookupTXT(uriParsed.Hostname())
	if err != nil {
		logger.Error("Failed to lookup TXT records", "cname", cname, "error", err)
	}
	if len(txtRecords) > 1 {
		logger.Error("Multiple TXT records were found and none will be applied", "cname", cname)
	}
	if len(txtRecords) == 1 {
		// We take connection parameters from the TXT record
		uriParams, err := url.ParseQuery(txtRecords[0])
		if err != nil {
			logger.Error("Failed to parse TXT record", "txt_record", txtRecords[0], "error", err)
		} else {
			// Override connection parameters with ones from URI query string
			for p, v := range uriParsed.Query() {
				uriParams[p] = v
			}
			queryString = uriParams.Encode()
		}
	}

	// Build final connection URI
	servers := make([]string, len(srvRecords))
	for i, srv := range srvRecords {
		servers[i] = net.JoinHostPort(strings.TrimSuffix(srv.Target, "."), strconv.FormatUint(uint64(srv.Port), 10))
	}
	uri = "mongodb://"
	if uriParsed.User != nil {
		uri += uriParsed.User.String() + "@"
	}
	uri += strings.Join(servers, ",")
	if uriParsed.Path != "" {
		uri += uriParsed.Path
	} else {
		uri += "/"
	}
	if queryString != "" {
		uri += "?" + queryString
	}

	return uri
}
