package main

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/pschou/go-flowfile"
)

var (
	certFile = flag.String("cert", "someCertFile", "A PEM eoncoded certificate file.")
	keyFile  = flag.String("key", "someKeyFile", "A PEM encoded private key file.")
	caFile   = flag.String("CA", "someCertCAFile", "A PEM eoncoded CA's certificate file.")
)

var tlsConfig *tls.Config

func loadTLS() {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatal(err)
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(*caFile)
	if err != nil {
		log.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Setup HTTPS client
	tlsConfig = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: false,
		ClientAuth:         tls.RequireAndVerifyClientCert,

		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
	tlsConfig.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	http.DefaultClient = &http.Client{Transport: transport}
	http.DefaultClient = &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

func certPKIXString(name pkix.Name, sep string) (out string) {
	for i := len(name.Names) - 1; i > 0; i-- {
		if out != "" {
			out += sep
		}
		out += pkix.RDNSequence([]pkix.RelativeDistinguishedNameSET{name.Names[i : i+1]}).String()
	}
	return
}

func updateChain(f *flowfile.File, r *http.Request) error {
	opts := x509.VerifyOptions{
		Intermediates: tlsConfig.RootCAs,
	}

	chain := []string{}
	// Get client certificate
	for _, c := range r.TLS.PeerCertificates {
		_, err := c.Verify(opts)
		if err == nil {
			chain = []string{certPKIXString(c.Subject, ",")}
			break
		}
	}
	if len(chain) == 0 || chain[0] == "" {
		return fmt.Errorf("Failed to verify client")
	}

	// Get the current chain:
	for i := 0; i < 20; i++ {
		v := fmt.Sprintf("connection-chain-%d", i)
		if c := f.Attrs.Get(v); c != "" {
			chain = append(chain, c)
		}
		f.Attrs.Unset(v)
	}

	// Set the current chain:
	for i, c := range chain {
		v := fmt.Sprintf("connection-chain-%d", i)
		f.Attrs.Set(v, c)
	}
	return nil
}
