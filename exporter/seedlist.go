package exporter

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

func GetSeedListFromSRV(uri string, log *logrus.Logger) string {
	uriParsed, err := url.Parse(uri)
	if err != nil {
		log.Fatalf("Failed to parse URI %s: %v", uri, err)
	}

	cname, srvRecords, err := net.LookupSRV("mongodb", "tcp", uriParsed.Hostname())
	if err != nil {
		log.Errorf("Failed to lookup SRV records for %s: %v", uri, err)
		return uri
	}

	if len(srvRecords) == 0 {
		log.Errorf("No SRV records found for %s", uri)
		return uri
	}

	queryString := uriParsed.RawQuery

	txtRecords, err := net.LookupTXT(uriParsed.Hostname())
	if err != nil {
		log.Errorf("Failed to lookup TXT records for %s: %v", cname, err)
	}
	if len(txtRecords) > 1 {
		log.Errorf("Multiple TXT records found for %s, thus were not applied", cname)
	}
	if len(txtRecords) == 1 {
		// We take connection paramters from the TXT record
		uriParams, err := url.ParseQuery(txtRecords[0])
		if err != nil {
			log.Errorf("Failed to parse TXT record %s: %v", txtRecords[0], err)
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
		servers[i] = net.JoinHostPort(strings.TrimSuffix(srv.Target, "."), fmt.Sprint(srv.Port))
	}
	uri = "mongodb://"
	if uriParsed.User != nil {
		uri += uriParsed.User.String() + "@"
	}
	uri = uri + strings.Join(servers, ",")
	if uriParsed.Path != "" {
		uri += uriParsed.Path
	} else {
		uri += "/"
	}
	if queryString != "" {
		uri += "?" + queryString
	}
	log.Debugf("Made seedlist from srv: %s to %s", uriParsed.String(), uri)
	return uri
}
