package networking

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
)

func SetTlsConfig() *tls.Config {
	rootCABytes, err := ioutil.ReadFile("pem_here")
	if err != nil {
		log.Fatal("failed to read root certificate file")
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(rootCABytes)
	if !ok {
		log.Fatal("failed to parse root certificate")
	}

	config := &tls.Config{RootCAs: roots}

	return config
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
