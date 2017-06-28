package shared

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
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
