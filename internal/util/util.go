// mongodb_exporter
// Copyright (C) 2023 Percona LLC
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

// Package util provides utility functions for the exporter.
package util

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"

	"github.com/percona/mongodb_exporter/internal/proto"
)

// Error codes returned by MongoDB.
const (
	ErrNotYetInitialized     = int32(94)
	ErrNoReplicationEnabled  = int32(76)
	ErrNotPrimaryOrSecondary = int32(13436)
)

// MyState returns the replica set and the instance's state if available.
func MyState(ctx context.Context, client *mongo.Client) (string, int, error) {
	var status proto.ReplicaSetStatus

	err := client.Database("admin").RunCommand(ctx, bson.M{"replSetGetStatus": 1}).Decode(&status)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get replica set status: %w", err)
	}

	return status.Set, int(status.MyState), nil
}

// MyRole returns the role of the mongo instance.
func MyRole(ctx context.Context, client *mongo.Client) (*proto.HelloResponse, error) {
	var role proto.HelloResponse
	err := client.Database("admin").RunCommand(ctx, bson.M{"isMaster": 1}).Decode(&role)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}

	return &role, nil
}

// ReplicasetConfig returns the replica set configuration.
func ReplicasetConfig(ctx context.Context, client *mongo.Client) (*proto.ReplicasetConfig, error) {
	var rs proto.ReplicasetConfig
	if err := client.Database("admin").RunCommand(ctx, bson.M{"replSetGetConfig": 1}).Decode(&rs); err != nil {
		return nil, fmt.Errorf("failed to get replica set config: %w", err)
	}

	return &rs, nil
}

// IsReplicationNotEnabledError checks if the error is related to replication not being enabled.
func IsReplicationNotEnabledError(err mongo.CommandError) bool {
	return err.Code == ErrNotYetInitialized || err.Code == ErrNoReplicationEnabled ||
		err.Code == ErrNotPrimaryOrSecondary
}

// ClusterID returns the cluster ID of the MongoDB instance.
func ClusterID(ctx context.Context, client *mongo.Client) (string, error) {
	var cv proto.ConfigVersion
	if err := client.Database("config").Collection("version").FindOne(ctx, bson.M{}).Decode(&cv); err == nil {
		return cv.ClusterID.Hex(), nil
	}

	var si proto.ShardIdentity

	filter := bson.M{"_id": "shardIdentity"}

	if err := client.Database("admin").Collection("system.version").FindOne(ctx, filter).Decode(&si); err == nil {
		return si.ClusterID.Hex(), nil
	}

	rc, err := ReplicasetConfig(ctx, client)
	if err != nil {
		if errors.As(err, &mongo.CommandError{}) && IsReplicationNotEnabledError(err.(mongo.CommandError)) {
			return "", nil
		}
		if errors.As(err, &topology.ServerSelectionError{}) {
			return "", nil
		}

		return "", err
	}

	return rc.Config.Settings.ReplicaSetID.Hex(), nil
}
