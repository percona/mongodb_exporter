package mongod

import (
	"context"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/percona/mongodb_exporter/shared"
)

func Test_ParserTopStatus(t *testing.T) {
	raw := &TopStatusRaw{}
	uri := "mongodb://localhost:27017"
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.TODO())
	err = client.Database("admin").RunCommand(context.TODO(), bson.D{{"top", 1}}).Decode(&raw)
	if err != nil {
		t.Fatal(err)
	}

	topStatus := raw.TopStatus()

	buildInfo, err := shared.GetBuildInfo(client)
	if err != nil {
		t.Fatal(err)
	}

	collections := []string{
		"admin.system.roles",
		"admin.system.version",
		"local.system.replset",
	}

	collectionConstraints := []collectionConstraint{
		{"local.startup_log", "<=3.4.0"},
		{"local.oplog.rs", ">=3.6.0"},
	}

	collections = appendVersionSpecificCollections(collections, collectionConstraints, &buildInfo)

	assert.True(t, len(collections) <= len(topStatus.TopStats),
		"expected more than %d collections, got %d", len(collections), len(topStatus.TopStats))
	for _, col := range collections {
		assert.Contains(t, topStatus.TopStats, col)
		stats := topStatus.TopStats[col]
		assert.NotZero(t, stats.Total.Time, "%s: %+v", col, stats)
		assert.NotZero(t, stats.Total.Count, "%s: %+v", col, stats)
	}
}

type collectionConstraint struct {
	collection string
	constraint string
}

func appendVersionSpecificCollections(collections []string, collectionConstraints []collectionConstraint, buildInfo *shared.BuildInfo) []string {
	res := collections
	buildVersion, _ := semver.NewVersion(buildInfo.Version)

	for _, collectionConstraint := range collectionConstraints {
		constraints, _ := semver.NewConstraint(collectionConstraint.constraint)
		if constraints.Check(buildVersion) {
			res = append(collections, collectionConstraint.collection)
		}
	}

	return res
}
