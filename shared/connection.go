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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/network/connstring"
)

// RedactMongoUri removes login and password from mongoUri.
func RedactMongoUri(uri string) string {
	if strings.HasPrefix(uri, "mongodb://") && strings.Contains(uri, "@") {
		if strings.Contains(uri, "ssl=true") {
			uri = strings.Replace(uri, "ssl=true", "", 1)
		}

		cStr, err := connstring.Parse(uri)
		if err != nil {
			log.Errorf("Cannot parse mongodb server url: %s", err)
			return "unknown/error"
		}

		if cStr.Username != "" && cStr.Password != "" {
			uri = strings.Replace(uri, cStr.Username, "****", 1)
			uri = strings.Replace(uri, cStr.Password, "****", 1)
			return uri
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
	AuthentificationDB    string
}

// MongoClient connects to MongoDB and returns ready to use MongoDB client.
func MongoClient(opts *MongoSessionOpts) *mongo.Client {
	if strings.Contains(opts.URI, "ssl=true") {
		opts.URI = strings.Replace(opts.URI, "ssl=true", "", 1)
		opts.TLSConnection = true
	}

	cOpts := options.Client().
		ApplyURI(opts.URI).
		SetDirect(true).
		SetSocketTimeout(opts.SocketTimeout).
		SetConnectTimeout(opts.SyncTimeout).
		SetMaxPoolSize(uint16(opts.PoolLimit)).
		SetReadPreference(readpref.Nearest()).
		SetAppName("mongodb_exporter")

	if cOpts.Auth != nil {
		cOpts.Auth.AuthSource = opts.AuthentificationDB
	}

	err := opts.configureDialInfoIfRequired(cOpts)
	if err != nil {
		log.Errorf("%s", err)
		return nil
	}

	client, err := mongo.NewClient(cOpts)
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.SyncTimeout)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		log.Errorf("Cannot connect to server using url %s: %s", RedactMongoUri(opts.URI), err)
		return nil
	}

	return client
}

func (opts *MongoSessionOpts) configureDialInfoIfRequired(cOpts *options.ClientOptions) error {
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
		cOpts.SetDialer(&tlsDialer{config: config})
	}
	return nil
}

type tlsDialer struct {
	config *tls.Config
}

// DialContext custom dialer with ability to skip hostname validation.
func (d *tlsDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := tls.Dial(network, address, d.config)
	if err != nil {
		log.Errorf("Could not connect to %v. Got: %v", address, err)
		return nil, err
	}
	if d.config.InsecureSkipVerify {
		err = enrichWithOwnChecks(conn, d.config)
		if err != nil {
			log.Errorf("Could not disable hostname validation. Got: %v", err)
		}
	}
	return conn, err
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

// MongoSessionServerVersion returns mongo server version.
func MongoSessionServerVersion(client *mongo.Client) (string, error) {
	buildInfo, err := GetBuildInfo(client)
	if err != nil {
		log.Errorf("Could not get MongoDB BuildInfo: %s!", err)
		return "unknown", err
	}
	return buildInfo.Version, nil
}

// MongoSessionNodeType returns mongo node type.
func MongoSessionNodeType(client *mongo.Client) (string, error) {
	masterDoc := struct {
		SetName interface{} `bson:"setName"`
		Hosts   interface{} `bson:"hosts"`
		Msg     string      `bson:"msg"`
	}{}

	res := client.Database("admin").RunCommand(context.TODO(), bson.D{{Key: "isMaster", Value: 1}})

	err := res.Decode(&masterDoc)
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
	client := MongoClient(&opts)
	if client == nil {
		return nil, fmt.Errorf("Cannot connect using uri: %s", opts.URI)
	}
	buildInfo, err := GetBuildInfo(client)
	if err != nil {
		return nil, fmt.Errorf("Cannot get buildInfo() for MongoDB using uri %s: %s", opts.URI, err)
	}

	b, err := json.MarshalIndent(buildInfo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("Cannot create json: %s", err)
	}

	return b, nil
}

// BuildInfo represents mongodb build info.
type BuildInfo struct {
	Version        string
	VersionArray   []int  `bson:"versionArray"` // On MongoDB 2.0+; assembled from Version otherwise
	GitVersion     string `bson:"gitVersion"`
	OpenSSLVersion string `bson:"OpenSSLVersion"`
	SysInfo        string `bson:"sysInfo"` // Deprecated and empty on MongoDB 3.2+.
	Bits           int
	Debug          bool
	MaxObjectSize  int `bson:"maxBsonObjectSize"`
}

// GetBuildInfo gets mongo build info.
func GetBuildInfo(client *mongo.Client) (info BuildInfo, err error) {
	res := client.Database("admin").RunCommand(context.TODO(), bson.D{{Key: "buildInfo", Value: "1"}})
	err = res.Decode(&info)

	if len(info.VersionArray) == 0 {
		for _, a := range strings.Split(info.Version, ".") {
			i, err := strconv.Atoi(a)
			if err != nil {
				break
			}
			info.VersionArray = append(info.VersionArray, i)
		}
	}
	for len(info.VersionArray) < 4 {
		info.VersionArray = append(info.VersionArray, 0)
	}
	if i := strings.IndexByte(info.GitVersion, ' '); i >= 0 {
		// Strip off the " modules: enterprise" suffix. This is a _git version_.
		// That information may be moved to another field if people need it.
		info.GitVersion = info.GitVersion[:i]
	}
	if info.SysInfo == "deprecated" {
		info.SysInfo = ""
	}
	return
}
