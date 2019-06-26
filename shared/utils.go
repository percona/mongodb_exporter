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
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"runtime"
	"strconv"
)

func LoadCaFrom(pemFile string) (*x509.CertPool, error) {
	caCert, err := ioutil.ReadFile(pemFile)
	if err != nil {
		return nil, err
	}
	certificates := x509.NewCertPool()
	certificates.AppendCertsFromPEM(caCert)
	return certificates, nil
}

func LoadKeyPairFrom(pemFile string, privateKeyPemFile string) (tls.Certificate, error) {
	targetPrivateKeyPemFile := privateKeyPemFile
	if len(targetPrivateKeyPemFile) <= 0 {
		targetPrivateKeyPemFile = pemFile
	}
	return tls.LoadX509KeyPair(pemFile, targetPrivateKeyPemFile)
}

// GetCallerLocation gets location of the caller in the source code (e.g. "oplog_status.go:91").
func GetCallerLocation() string {
	_, fileName, lineNum, ok := runtime.Caller(1)
	if !ok {
		return ""
	}
	return fileName + ":" + strconv.Itoa(lineNum)
}
