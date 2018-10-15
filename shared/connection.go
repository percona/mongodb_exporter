// Copyright 2017 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/prometheus/common/log"
	"gopkg.in/mgo.v2"
)

func RedactMongoUri(uri string) string {
	if strings.HasPrefix(uri, "mongodb://") && strings.Contains(uri, "@") {
		if strings.Contains(uri, "ssl=true") {
			uri = strings.Replace(uri, "ssl=true", "", 1)
		}
		dialInfo, err := mgo.ParseURL(uri)
		if err != nil {
			log.Errorf("Cannot parse mongodb server url: %s", err)
			return "unknown/error"
		}
		if dialInfo.Username != "" && dialInfo.Password != "" {
			return "mongodb://****:****@" + strings.Join(dialInfo.Addrs, ",")
		}
	}
	return uri
}

type MongoSessionOpts struct {
	URI                   string
	TLSConnection         bool
	TLSCertificateFile    string
	TLSPrivateKeyFile     string
	TLSCaFile             string
	TLSHostnameValidation bool
	PoolLimit             int
	SocketTimeout         time.Duration
	SyncTimeout           time.Duration
}

// MongoSession connects to MongoDB and returns ready to use MongoDB session.
func MongoSession(opts *MongoSessionOpts) *mgo.Session {
	if strings.Contains(opts.URI, "ssl=true") {
		opts.URI = strings.Replace(opts.URI, "ssl=true", "", 1)
		opts.TLSConnection = true
	}
	dialInfo, err := mgo.ParseURL(opts.URI)
	if err != nil {
		log.Errorf("Cannot parse mongodb server url: %s", err)
		return nil
	}

	// connect directly, fail faster, do not retry - for faster responses and accurate metrics, including mongoUp
	dialInfo.Direct = true
	dialInfo.Timeout = opts.SocketTimeout
	dialInfo.FailFast = true

	err = opts.configureDialInfoIfRequired(dialInfo)
	if err != nil {
		log.Errorf("%s", err)
		return nil
	}

	session, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		log.Errorf("Cannot connect to server using url %s: %s", RedactMongoUri(opts.URI), err)
		return nil
	}
	session.SetMode(mgo.Eventual, true)
	session.SetPoolLimit(opts.PoolLimit)
	session.SetPrefetch(0.00)
	session.SetSyncTimeout(opts.SyncTimeout)
	session.SetSocketTimeout(opts.SocketTimeout)
	return session
}

func (opts *MongoSessionOpts) configureDialInfoIfRequired(dialInfo *mgo.DialInfo) error {
	if opts.TLSConnection {
		config := &tls.Config{
			InsecureSkipVerify: !opts.TLSHostnameValidation,
		}
		if len(opts.TLSCertificateFile) > 0 {
			certificates, err := LoadKeyPairFrom(opts.TLSCertificateFile, opts.TLSPrivateKeyFile)
			if err != nil {
				return fmt.Errorf("Cannot load key pair from '%s' and '%s' to connect to server '%s'. Got: %v", opts.TLSCertificateFile, opts.TLSPrivateKeyFile, opts.URI, err)
			}
			config.Certificates = []tls.Certificate{certificates}
		}
		if len(opts.TLSCaFile) > 0 {
			ca, err := LoadCaFrom(opts.TLSCaFile)
			if err != nil {
				return fmt.Errorf("Couldn't load client CAs from %s. Got: %s", opts.TLSCaFile, err)
			}
			config.RootCAs = ca
		}
		dialInfo.DialServer = func(addr *mgo.ServerAddr) (net.Conn, error) {
			conn, err := tls.Dial("tcp", addr.String(), config)
			if err != nil {
				log.Errorf("Could not connect to %v. Got: %v", addr, err)
				return nil, err
			}
			if config.InsecureSkipVerify {
				err = enrichWithOwnChecks(conn, config)
				if err != nil {
					log.Errorf("Could not disable hostname validation. Got: %v", err)
				}
			}
			return conn, err
		}
	}
	return nil
}

func enrichWithOwnChecks(conn *tls.Conn, tlsConfig *tls.Config) error {
	var err error
	if err = conn.Handshake(); err != nil {
		conn.Close()
		return err
	}

	opts := x509.VerifyOptions{
		Roots:         tlsConfig.RootCAs,
		CurrentTime:   time.Now(),
		DNSName:       "",
		Intermediates: x509.NewCertPool(),
	}

	certs := conn.ConnectionState().PeerCertificates
	for i, cert := range certs {
		if i == 0 {
			continue
		}
		opts.Intermediates.AddCert(cert)
	}

	_, err = certs[0].Verify(opts)
	if err != nil {
		conn.Close()
		return err
	}

	return nil
}

func MongoSessionServerVersion(session *mgo.Session) (string, error) {
	buildInfo, err := session.BuildInfo()
	if err != nil {
		log.Errorf("Could not get MongoDB BuildInfo: %s!", err)
		return "unknown", err
	}
	return buildInfo.Version, nil
}

func MongoSessionNodeType(session *mgo.Session) (string, error) {
	masterDoc := struct {
		SetName interface{} `bson:"setName"`
		Hosts   interface{} `bson:"hosts"`
		Msg     string      `bson:"msg"`
	}{}
	err := session.Run("isMaster", &masterDoc)
	if err != nil {
		log.Errorf("Got unknown node type: %s", err)
		return "unknown", err
	}

	if masterDoc.SetName != nil || masterDoc.Hosts != nil {
		return "replset", nil
	} else if masterDoc.Msg == "isdbgrid" {
		// isdbgrid is always the msg value when calling isMaster on a mongos
		// see http://docs.mongodb.org/manual/core/sharded-cluster-query-router/
		return "mongos", nil
	}
	return "mongod", nil
}

// TestConnection connects to MongoDB and returns BuildInfo.
func TestConnection(opts MongoSessionOpts) ([]byte, error) {
	session := MongoSession(&opts)
	if session == nil {
		return nil, fmt.Errorf("Cannot connect using uri: %s", opts.URI)
	}
	buildInfo, err := session.BuildInfo()
	if err != nil {
		return nil, fmt.Errorf("Cannot get buildInfo() for MongoDB using uri %s: %s", opts.URI, err)
	}

	b, err := json.MarshalIndent(buildInfo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("Cannot create json: %s", err)
	}

	return b, nil
}
