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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestConfigureAWSCredentialsProvider(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "test-session-token")
	t.Setenv("AWS_REGION", "eu-central-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	clientOpts := options.Client().SetAuth(options.Credential{AuthMechanism: mongoDBAWSAuthMechanism})

	require.NoError(t, configureAWSCredentialsProvider(context.Background(), clientOpts))
	require.NotNil(t, clientOpts.Auth.AWSCredentialsProvider)

	credentials, err := clientOpts.Auth.AWSCredentialsProvider.Retrieve(context.Background())
	require.NoError(t, err)
	require.Equal(t, "test-access-key", credentials.AccessKeyID)
	require.Equal(t, "test-secret-key", credentials.SecretAccessKey)
	require.Equal(t, "test-session-token", credentials.SessionToken)
}

func TestConfigureAWSCredentialsProviderSkipsOtherMechanisms(t *testing.T) {
	t.Parallel()

	clientOpts := options.Client().SetAuth(options.Credential{AuthMechanism: "SCRAM-SHA-256"})

	require.NoError(t, configureAWSCredentialsProvider(context.Background(), clientOpts))
	require.Nil(t, clientOpts.Auth.AWSCredentialsProvider)
}
