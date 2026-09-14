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
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"go.mongodb.org/mongo-driver/ext/awsauth"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const mongoDBAWSAuthMechanism = "MONGODB-AWS"

func configureAWSCredentialsProvider(ctx context.Context, clientOpts *options.ClientOptions) error {
	if clientOpts.Auth == nil || clientOpts.Auth.AuthMechanism != mongoDBAWSAuthMechanism {
		return nil
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS configuration for %s authentication: %w", mongoDBAWSAuthMechanism, err)
	}

	clientOpts.Auth.AWSCredentialsProvider = awsauth.NewCredentialsProvider(awsConfig.Credentials)

	return nil
}
