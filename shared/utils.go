package shared

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
)

var (
	snakeRegexp        = regexp.MustCompile("\\B[A-Z]+[^_$]")
	parameterizeRegexp = regexp.MustCompile("[^A-Za-z0-9_]+")
)

// SnakeCase converts the given text to snakecase/underscore syntax.
func SnakeCase(text string) string {
	result := snakeRegexp.ReplaceAllStringFunc(text, func(match string) string {
		return "_" + match
	})

	return ParameterizeString(result)
}

// ParameterizeString parameterizes the given string.
func ParameterizeString(text string) string {
	result := parameterizeRegexp.ReplaceAllString(text, "_")
	return strings.ToLower(result)
}

func IsVersionGreater(version string, major int, minor int, release int) bool {
	split := strings.Split(version, ".")
	cmp_major, _ := strconv.Atoi(split[0])
	cmp_minor, _ := strconv.Atoi(split[1])
	cmp_release, _ := strconv.Atoi(split[2])

	if cmp_major >= major && cmp_minor >= minor && cmp_release >= release {
		return true
	}

	return false
}

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
