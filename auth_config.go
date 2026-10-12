// mongodb_exporter
// Copyright (C) 2026 Percona LLC
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

// Package main runs the MongoDB exporter.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	userPassAuthType  = "userpass"
	defaultAuthSource = "admin"
)

var (
	errUnsupportedAuthModuleType = errors.New("unsupported auth module type")
	errAuthModuleCredentials     = errors.New("auth module username and password are required")
	errAuthModuleTLS             = errors.New("auth module tls must be enabled when tls_insecure_skip_verify is true")
	errAuthModuleNotFound        = errors.New("auth module not found")
	errAuthModuleRequired        = errors.New("auth_module is required")
	errNoAuthModules             = errors.New("no auth modules configured")
)

type authConfig struct {
	AuthModules map[string]authModule `yaml:"auth_modules"` //nolint:tagliatelle // Match Prometheus auth module config conventions.
}

type authModule struct {
	Type     string            `yaml:"type"`
	UserPass userPass          `yaml:"userpass"`
	Options  authModuleOptions `yaml:"options"`
}

type userPass struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type authModuleOptions struct {
	AuthSource            string `yaml:"auth_source"` //nolint:tagliatelle // Match Prometheus auth module config conventions.
	TLS                   bool   `yaml:"tls"`
	TLSInsecureSkipVerify bool   `yaml:"tls_insecure_skip_verify"` //nolint:tagliatelle // Match Prometheus auth module config conventions.
}

func loadAuthConfig(path string) (authConfig, error) {
	if path == "" {
		return authConfig{}, nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // The operator explicitly selects this config file.
	if err != nil {
		return authConfig{}, fmt.Errorf("read config file: %w", err)
	}

	var config authConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	err = decoder.Decode(&config)
	if err != nil {
		return authConfig{}, fmt.Errorf("decode config file: %w", err)
	}

	for name, module := range config.AuthModules {
		if module.Type != userPassAuthType {
			return authConfig{}, fmt.Errorf("%w: module %q has type %q", errUnsupportedAuthModuleType, name, module.Type)
		}
		if module.UserPass.Username == "" || module.UserPass.Password == "" {
			return authConfig{}, fmt.Errorf("%w: %q", errAuthModuleCredentials, name)
		}
		if module.Options.AuthSource == "" {
			module.Options.AuthSource = defaultAuthSource
			config.AuthModules[name] = module
		}
		if module.Options.TLSInsecureSkipVerify && !module.Options.TLS {
			return authConfig{}, fmt.Errorf("%w: %q", errAuthModuleTLS, name)
		}
	}

	return config, nil
}

func resolveAuthModule(modules map[string]authModule, name string) (authModule, error) {
	if name != "" {
		module, ok := modules[name]
		if !ok {
			return authModule{}, fmt.Errorf("%w: %q", errAuthModuleNotFound, name)
		}

		return module, nil
	}

	if len(modules) != 1 {
		return authModule{}, errAuthModuleRequired
	}
	for _, module := range modules {
		return module, nil
	}

	return authModule{}, errNoAuthModules
}

func buildDynamicURI(target string, module authModule) string {
	authSource := module.Options.AuthSource
	if authSource == "" {
		authSource = defaultAuthSource
	}
	uri := &url.URL{
		Scheme: "mongodb",
		User:   url.UserPassword(module.UserPass.Username, module.UserPass.Password),
		Host:   target,
		Path:   "/" + authSource,
	}
	query := url.Values{"authSource": []string{authSource}}
	if module.Options.TLS {
		query.Set("tls", "true")
	}
	if module.Options.TLSInsecureSkipVerify {
		query.Set("tlsInsecure", "true")
	}
	uri.RawQuery = query.Encode()

	return uri.String()
}
