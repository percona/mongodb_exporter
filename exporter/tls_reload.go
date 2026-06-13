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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strings"

	"go.mongodb.org/mongo-driver/mongo/options"
)

type tlsFilePaths struct {
	CertificateKeyFile         string
	CertificateKeyFilePassword string
	CAFile                     string
}

// extractTLSFilePaths returns TLS file paths supplied via URI query parameters.
func extractTLSFilePaths(uri string) tlsFilePaths {
	var p tlsFilePaths
	u, err := url.Parse(uri)
	if err != nil {
		return p
	}
	for k, v := range u.Query() {
		if len(v) == 0 {
			continue
		}
		switch strings.ToLower(k) {
		case "tlscertificatekeyfile":
			p.CertificateKeyFile = v[0]
		case "tlscertificatekeyfilepassword":
			p.CertificateKeyFilePassword = v[0]
		case "tlscafile":
			p.CAFile = v[0]
		}
	}
	return p
}

// installReloadableTLS replaces static cert material on clientOpts.TLSConfig
// with closures that re-read tlsCertificateKeyFile / tlsCAFile from disk on
// every new TLS handshake. Rotated certs take effect on the next freshly
// dialled connection without restarting the exporter or recreating the
// *mongo.Client.
//
// No-op when TLS is not configured or no file paths were given in the URI.
func installReloadableTLS(uri string, clientOpts *options.ClientOptions) error {
	if clientOpts == nil || clientOpts.TLSConfig == nil {
		return nil
	}
	paths := extractTLSFilePaths(uri)
	if paths.CertificateKeyFile == "" && paths.CAFile == "" {
		return nil
	}

	tlsCfg := clientOpts.TLSConfig.Clone()

	if paths.CertificateKeyFile != "" {
		if paths.CertificateKeyFilePassword != "" {
			return fmt.Errorf("tls reload: password-protected key files are not supported")
		}
		if _, err := loadKeyPair(paths.CertificateKeyFile); err != nil {
			return fmt.Errorf("tls reload: initial load of %q: %w", paths.CertificateKeyFile, err)
		}
		tlsCfg.Certificates = nil
		certFile := paths.CertificateKeyFile
		tlsCfg.GetClientCertificate = func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return loadKeyPair(certFile)
		}
	}

	if paths.CAFile != "" {
		if _, err := loadCAPool(paths.CAFile); err != nil {
			return fmt.Errorf("tls reload: initial load of CA %q: %w", paths.CAFile, err)
		}
		// Disable the stdlib default verification path; our callback drives it.
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // verification still performed below
		serverName := tlsCfg.ServerName
		caFile := paths.CAFile
		tlsCfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			pool, err := loadCAPool(caFile)
			if err != nil {
				return fmt.Errorf("tls reload: read CA %q: %w", caFile, err)
			}
			if len(rawCerts) == 0 {
				return fmt.Errorf("tls reload: no peer certificates presented")
			}
			certs := make([]*x509.Certificate, 0, len(rawCerts))
			for _, raw := range rawCerts {
				c, err := x509.ParseCertificate(raw)
				if err != nil {
					return fmt.Errorf("tls reload: parse peer cert: %w", err)
				}
				certs = append(certs, c)
			}
			verifyOpts := x509.VerifyOptions{
				Roots:         pool,
				DNSName:       serverName,
				Intermediates: x509.NewCertPool(),
			}
			for _, c := range certs[1:] {
				verifyOpts.Intermediates.AddCert(c)
			}
			_, err = certs[0].Verify(verifyOpts)
			return err
		}
	}

	clientOpts.TLSConfig = tlsCfg
	return nil
}

// loadKeyPair reads a MongoDB-style combined PEM file (cert + key bundled in
// the same file) from disk and returns a parsed *tls.Certificate.
func loadKeyPair(certKeyFile string) (*tls.Certificate, error) {
	pem, err := os.ReadFile(certKeyFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(pem, pem)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func loadCAPool(caFile string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no certificates found in %q", caFile)
	}
	return pool, nil
}
