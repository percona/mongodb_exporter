package shared

import (
	"strings"
	"time"

	"github.com/golang/glog"
	"gopkg.in/mgo.v2"
)

const (
	dialMongodbTimeout = 5 * time.Second
	syncMongodbTimeout = 1 * time.Minute
)

func RedactMongoUri(uri string) string {
	dialInfo, err := mgo.ParseURL(uri)
	if err != nil {
		glog.Errorf("Cannot parse mongodb server url: %s", err)
		return ""
	}
	return "mongodb://" + strings.Join(dialInfo.Addrs, ",")
}

func MongoSession(uri string) *mgo.Session {
	dialInfo, err := mgo.ParseURL(uri)
	if err != nil {
		glog.Errorf("Cannot parse mongodb server url: %s", err)
		return nil
	}

	dialInfo.Direct = true // Force direct connection
	dialInfo.Timeout = dialMongodbTimeout

	session, err := mgo.DialWithInfo(dialInfo)
	if err != nil {
		glog.Errorf("Cannot connect to server using url %s: %s", RedactMongoUri(uri), err)
		return nil
	}
	session.SetMode(mgo.Eventual, true)
	session.SetSyncTimeout(syncMongodbTimeout)
	session.SetSocketTimeout(0)
	return session
}

func MongoSessionServerVersion(session *mgo.Session) (string, error) {
	buildInfo, err := session.BuildInfo()
	if err != nil {
		glog.Errorf("Could not get MongoDB BuildInfo: %s!", err)
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
		glog.Errorf("Got unknown node type: %s", err)
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
