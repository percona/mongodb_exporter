// mongodb_exporter
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

package exporter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var systemDBs = []string{"admin", "config", "local"} //nolint:gochecknoglobals

const unauthorizedErrorCode = 13

func listCollections(ctx context.Context, client *mongo.Client, database string, filterInNamespaces []string, skipViews bool) ([]string, error) {
	opts := &options.ListCollectionsOptions{NameOnly: pointer.ToBool(true), AuthorizedCollections: pointer.ToBool(true)}
	filter := bson.D{} // Default=empty -> list all collections

	// if there is a filter with the list of collections we want, create a filter like
	// $or: {
	//     {"$regex": "collection1"},
	//     {"$regex": "collection2"},
	// }
	if len(filterInNamespaces) > 0 {
		matchExpressions := []bson.D{}

		for _, namespace := range filterInNamespaces {
			parts := strings.Split(namespace, ".") // db.collection.name.with.dots
			if len(parts) > 1 {
				// The part before the first dot is the database name.
				// The rest is the collection name and it can have dots. We need to rebuild it.
				collection := strings.Join(parts[1:], ".")
				matchExpressions = append(matchExpressions,
					bson.D{{Key: "name", Value: primitive.Regex{Pattern: collection, Options: "i"}}})
			}
		}

		if len(matchExpressions) > 0 {
			filter = bson.D{{Key: "$or", Value: matchExpressions}}
		}
	}

	if skipViews {
		filter = append(filter, primitive.E{Key: "type", Value: bson.D{
			{Key: "$in", Value: bson.A{"collection", "timeseries"}},
		}})
	}

	collections, err := client.Database(database).ListCollectionNames(ctx, filter, opts)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the list of collections for discovery")
	}

	return collections, nil
}

// databases returns the list of databases matching the filters.
// - filterInNamespaces: Include only the database names matching the any of the regular expressions in this list.
//
//	Case will be ignored because the function will automatically add the ignore case
//	flag to the regular expression.
//
// - exclude: List of databases to be excluded. Useful to ignore system databases.
//
// Only databases the user has privileges on are returned, so collectors don't run
// commands that fail with "not authorized".
func databases(ctx context.Context, client *mongo.Client, filterInNamespaces []string, exclude []string) ([]string, error) {
	dbNames, err := authorizedDatabaseNames(ctx, client)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the database names list")
	}

	return filterDatabaseNames(dbNames, filterInNamespaces, exclude), nil
}

// authorizedDatabaseNames returns the databases the connected user has privileges on.
//
// Some managed MongoDB services put a proxy in front of the server that only accepts a fixed
// set of listDatabases arguments and rejects the others, for example "Unallowed argument in
// listDatabases command: authorizedDatabases" or "...: filter". When the server rejects the
// command, retry without those arguments and apply the authorization filter on our side.
func authorizedDatabaseNames(ctx context.Context, client *mongo.Client) ([]string, error) {
	opts := options.ListDatabases().SetNameOnly(true).SetAuthorizedDatabases(true)

	dbNames, err := client.ListDatabaseNames(ctx, bson.D{}, opts)
	if err == nil {
		return dbNames, nil
	}

	var cmdErr mongo.CommandError
	if !errors.As(err, &cmdErr) {
		return nil, errors.Wrap(err, "listDatabases failed")
	}

	allNames, fallbackErr := listDatabaseNamesBare(ctx, client)
	if fallbackErr != nil {
		return nil, errors.Wrap(err, "listDatabases failed")
	}

	access, privErr := userDatabasePrivileges(ctx, client)
	if privErr != nil {
		// Without the user's privileges we can't do better than the full list.
		return allNames, nil //nolint:nilerr
	}

	authorized := make([]string, 0, len(allNames))
	for _, name := range allNames {
		if access.allows(name) {
			authorized = append(authorized, name)
		}
	}

	return authorized, nil
}

// listDatabaseNamesBare runs listDatabases without filter and authorizedDatabases.
// The driver's ListDatabaseNames always sends a filter, even an empty one.
func listDatabaseNamesBare(ctx context.Context, client *mongo.Client) ([]string, error) {
	var res struct {
		Databases []struct {
			Name string `bson:"name"`
		} `bson:"databases"`
	}

	cmd := bson.D{{Key: "listDatabases", Value: 1}, {Key: "nameOnly", Value: true}}

	err := client.Database("admin").RunCommand(ctx, cmd).Decode(&res)
	if err != nil {
		return nil, errors.Wrap(err, "listDatabases without filter and authorizedDatabases failed")
	}

	names := make([]string, 0, len(res.Databases))
	for _, db := range res.Databases {
		names = append(names, db.Name)
	}

	return names, nil
}

type privilegeResource struct {
	DB            *string `bson:"db"`
	Collection    *string `bson:"collection"`
	SystemBuckets *string `bson:"system_buckets"`
	AnyResource   bool    `bson:"anyResource"`
}

// databaseAccess describes the databases the connected user has privileges on.
type databaseAccess struct {
	// all is true when every database is authorized.
	all bool
	// allNormal is true when every database except local and config is authorized.
	allNormal bool
	// dbs are the databases authorized by name.
	dbs map[string]struct{}
}

func (a databaseAccess) allows(name string) bool {
	if a.all {
		return true
	}

	if a.allNormal && name != "local" && name != "config" {
		return true
	}

	_, ok := a.dbs[name]

	return ok
}

// userDatabasePrivileges returns the databases the connected user has any privilege on, using
// connectionStatus. Every database is authorized when auth is disabled.
func userDatabasePrivileges(ctx context.Context, client *mongo.Client) (databaseAccess, error) {
	var res struct {
		AuthInfo struct {
			AuthenticatedUsers []bson.Raw `bson:"authenticatedUsers"`
			Privileges         []struct {
				Resource privilegeResource `bson:"resource"`
			} `bson:"authenticatedUserPrivileges"`
		} `bson:"authInfo"`
	}

	cmd := bson.D{{Key: "connectionStatus", Value: 1}, {Key: "showPrivileges", Value: true}}

	err := client.Database("admin").RunCommand(ctx, cmd).Decode(&res)
	if err != nil {
		return databaseAccess{}, errors.Wrap(err, "connectionStatus failed")
	}

	if len(res.AuthInfo.AuthenticatedUsers) == 0 {
		return databaseAccess{all: true}, nil
	}

	resources := make([]privilegeResource, 0, len(res.AuthInfo.Privileges))
	for _, p := range res.AuthInfo.Privileges {
		resources = append(resources, p.Resource)
	}

	return databasesFromPrivileges(resources), nil
}

// databasesFromPrivileges mirrors listDatabases with authorizedDatabases: true. A database is
// authorized if the user has a privilege on it or on any of its collections. A privilege on
// all databases ({db: "", collection: ""}) doesn't include local and config, like on the server.
// A privilege on one collection name in all databases ({db: "", collection: "name"}), on system
// buckets without a database, or on any resource authorizes every database. Cluster privileges
// don't authorize any database.
func databasesFromPrivileges(resources []privilegeResource) databaseAccess {
	access := databaseAccess{dbs: make(map[string]struct{})}

	for _, r := range resources {
		switch {
		case r.AnyResource:
			access.all = true
		case r.SystemBuckets != nil && (r.DB == nil || *r.DB == ""):
			access.all = true
		case r.DB == nil:
			// Cluster privilege.
		case *r.DB != "":
			access.dbs[*r.DB] = struct{}{}
		case r.Collection == nil || *r.Collection == "":
			access.allNormal = true
		default:
			access.all = true
		}
	}

	return access
}

// filterDatabaseNames keeps the databases named in filterInNamespaces (the part before the
// first dot of each namespace; all databases if the list is empty) and drops the ones in exclude.
// The order of dbNames is preserved.
func filterDatabaseNames(dbNames []string, filterInNamespaces []string, exclude []string) []string {
	excluded := make(map[string]struct{}, len(exclude))
	for _, name := range exclude {
		excluded[name] = struct{}{}
	}

	included := make(map[string]struct{})
	for _, namespace := range removeEmptyStrings(filterInNamespaces) {
		dbname, _ := splitNamespace(namespace)
		included[dbname] = struct{}{}
	}

	filtered := make([]string, 0, len(dbNames))
	for _, name := range dbNames {
		if _, ok := excluded[name]; ok {
			continue
		}
		if _, ok := included[name]; len(included) > 0 && !ok {
			continue
		}
		filtered = append(filtered, name)
	}

	return filtered
}

// isUnauthorizedError reports whether err is a server "not authorized" (Unauthorized, code 13) error.
func isUnauthorizedError(err error) bool {
	var cmdErr mongo.CommandError

	return errors.As(err, &cmdErr) && cmdErr.Code == unauthorizedErrorCode
}

func removeEmptyStrings(items []string) []string {
	cleanList := []string{}

	for _, item := range items {
		if item == "" {
			continue
		}
		cleanList = append(cleanList, item)
	}

	return cleanList
}

func unique(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}

	for _, entry := range slice {
		if _, ok := keys[entry]; !ok {
			keys[entry] = true
			list = append(list, entry)
		}
	}

	return list
}

func checkNamespacesForViews(ctx context.Context, client *mongo.Client, collections []string) ([]string, error) {
	onlyCollectionsNamespaces, err := listAllCollections(ctx, client, nil, nil, true)
	if err != nil {
		return nil, err
	}

	namespaces := make(map[string]struct{})
	for db, collections := range onlyCollectionsNamespaces {
		for _, collection := range removeEmptyStrings(collections) {
			namespaces[fmt.Sprintf("%s.%s", db, collection)] = struct{}{}
		}
	}

	filteredCollections := []string{}
	for _, collection := range removeEmptyStrings(collections) {
		if len(strings.Split(collection, ".")) < 2 { //nolint:mnd
			continue
		}

		if _, ok := namespaces[collection]; !ok {
			return nil, errors.Errorf("namespace %s is a view and cannot be used for collstats/indexstats", collection)
		}

		filteredCollections = append(filteredCollections, collection)
	}

	return filteredCollections, nil
}

func listAllCollections(ctx context.Context, client *mongo.Client, filterInNamespaces []string, excludeDBs []string, skipViews bool) (map[string][]string, error) {
	namespaces := make(map[string][]string)

	dbs, err := databases(ctx, client, filterInNamespaces, excludeDBs)
	if err != nil {
		return nil, errors.Wrap(err, "cannot make the list of databases to list all collections")
	}

	filterNS := removeEmptyStrings(filterInNamespaces)

	// If there are no specified namespaces to search for collections, it means all dbs should be included.
	if len(filterNS) == 0 {
		filterNS = append(filterNS, dbs...)
	}

	for _, db := range dbs {
		for _, namespace := range filterNS {
			parts := strings.Split(namespace, ".")
			dbname := strings.TrimSpace(parts[0])

			if dbname == "" || dbname != db {
				continue
			}

			colls, err := listCollections(ctx, client, db, []string{namespace}, skipViews)
			if err != nil {
				// Can happen when the user's privileges couldn't be read (see authorizedDatabaseNames).
				// Skip the database so the authorized ones are still collected.
				if isUnauthorizedError(err) {
					continue
				}

				return nil, errors.Wrapf(err, "cannot list the collections for %q", db)
			}

			if _, ok := namespaces[db]; !ok {
				namespaces[db] = []string{}
			}

			namespaces[db] = append(namespaces[db], colls...)
		}
	}

	// Make it testable.
	for db, colls := range namespaces {
		uc := unique(colls)
		sort.Strings(uc)
		namespaces[db] = uc
	}

	return namespaces, nil
}

// nonSystemCollectionsCount counts the non-system collections. Databases the user isn't
// authorized on are skipped; complete is false when that happens, so the count only covers
// the databases that could be read.
func nonSystemCollectionsCount(ctx context.Context, client *mongo.Client, includeNamespaces []string, filterInCollections []string) (int, bool, error) {
	databases, err := databases(ctx, client, includeNamespaces, systemDBs)
	if err != nil {
		return 0, false, errors.Wrap(err, "cannot retrieve the collection names for count collections")
	}

	count := 0
	complete := true

	for _, dbname := range databases {
		colls, err := listCollections(ctx, client, dbname, filterInCollections, true)
		if err != nil {
			if isUnauthorizedError(err) {
				complete = false
				continue
			}

			return 0, false, errors.Wrap(err, "cannot get collections count")
		}
		count += len(colls)
	}

	return count, complete, nil
}

// splitNamespace splits the namespace into database and collection.
func splitNamespace(ns string) (string, string) {
	parts := strings.Split(ns, ".")
	// there is no collection?
	if len(parts) < 2 { //nolint:mnd
		return parts[0], ""
	}

	return parts[0], strings.Join(parts[1:], ".")
}

// setShardLabel sets the "shard" label from a single $collStats or $indexStats
// document; documents reporting no shard get an empty value. The label is set
// unconditionally because descriptors are built from these labels and
// MustRegister panics when a collector describes one fully-qualified name with
// two different label sets. Prometheus drops empty labels at ingestion, so
// deployments without shards keep the same series.
func setShardLabel(labels map[string]string, doc bson.M) {
	shard, _ := doc["shard"].(string)
	labels["shard"] = shard
}

func fromMapToSlice(databases map[string][]string) []string {
	var collections []string
	for db, cols := range databases {
		for _, value := range cols {
			collections = append(collections, db+"."+value)
		}
	}

	return collections
}
