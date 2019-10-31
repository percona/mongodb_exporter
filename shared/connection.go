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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/common/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
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
	URI string
}

// MongoClient connects to MongoDB and returns ready to use MongoDB client.
func MongoClient(opts *MongoSessionOpts) *mongo.Client {
	cOpts := options.Client().
		ApplyURI(opts.URI).
		SetDirect(true).
		SetReadPreference(readpref.Nearest()).
		SetAppName("mongodb_exporter")

	client, err := mongo.NewClient(cOpts)
	if err != nil {
		return nil
	}

	err = client.Connect(context.Background())
	if err != nil {
		log.Errorf("Cannot connect to server using url %s: %s", RedactMongoUri(opts.URI), err)
		return nil
	}

	return client
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
