package exporter

import (
	"context"

	"github.com/AlekSi/pointer"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var systemDBs = []string{"admin", "config", "local"} //nolint:gochecknoglobals

func listCollections(ctx context.Context, client *mongo.Client, collections []string, database string) ([]string, error) {
	filter := bson.D{} // Default=empty -> list all collections

	// if there is a filter with the list of collections we want, create a filter like
	// $or: {
	//     {"$regex": "collection1"},
	//     {"$regex": "collection2"},
	// }
	if len(collections) > 0 {
		matchExpressions := []bson.D{}

		for _, collection := range collections {
			matchExpressions = append(matchExpressions,
				bson.D{{Key: "name", Value: primitive.Regex{Pattern: collection, Options: "i"}}})
		}

		filter = bson.D{{Key: "$or", Value: matchExpressions}}
	}

	databases, err := client.Database(database).ListCollectionNames(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the list of collections for discovery")
	}

	return databases, nil
}

func databases(ctx context.Context, client *mongo.Client, exclude []string) ([]string, error) {
	opts := &options.ListDatabasesOptions{NameOnly: pointer.ToBool(true), AuthorizedDatabases: pointer.ToBool(true)}
	filterExpressions := []bson.D{}
	for _, dbname := range exclude {
		filterExpressions = append(filterExpressions,
			bson.D{{Key: "name", Value: bson.D{{Key: "$ne", Value: dbname}}}},
		)
	}

	filter := bson.D{{Key: "$and", Value: filterExpressions}}

	dbNames, err := client.ListDatabaseNames(ctx, filter, opts)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the database names list")
	}

	return dbNames, nil
}

func listAllCollections(ctx context.Context, client *mongo.Client, filter []string) (map[string][]string, error) {
	namespaces := make(map[string][]string)
	// exclude system databases
	dbnames, err := databases(ctx, client, systemDBs)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the list of all collections in the server")
	}

	for _, dbname := range dbnames {
		colls, err := listCollections(ctx, client, filter, dbname)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot list the collections for %q", dbname)
		}
		namespaces[dbname] = colls
	}

	return namespaces, nil
}

func allCollectionsCount(ctx context.Context, client *mongo.Client, filter []string) (int, error) {
	databases, err := databases(ctx, client, systemDBs)
	if err != nil {
		return 0, errors.Wrap(err, "cannot retrieve the collection names for count collections")
	}

	var count int

	for _, dbname := range databases {
		colls, err := listCollections(ctx, client, filter, dbname)
		if err != nil {
			return 0, errors.Wrap(err, "cannot get collections count")
		}
		count += len(colls)
	}

	return count, nil
}
