package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/pschou/go-flowfile"
)

func updateChain(f *flowfile.File, r *http.Request, label string) {
	var cert *x509.Certificate

	if r != nil && r.TLS != nil {
		if len(r.TLS.PeerCertificates) > 0 {
			cert = r.TLS.PeerCertificates[0]
		}
	}

	// Get the current chain:
	for i := 20; i > 0; i-- {
		for _, name := range []string{"source.host", "issuer.dn", "user.dn", "request.uri", "source.port",
			"cipher", "time", "source"} {
			ahead := fmt.Sprintf("restlistener.chain.%d.%s", i-1, name)
			cur := fmt.Sprintf("restlistener.chain.%d.%s", i, name)
			if c := f.Attrs.Get(ahead); c != "" {
				f.Attrs.Set(cur, c)
			}
			f.Attrs.Unset(ahead)
		}
	}

	f.Attrs.Set("restlistener.chain.0.time", time.Now().Format(time.RFC3339))
	if cert != nil {
		//f.Attrs.Set("restlistener.remote.user.dn", certPKIXString(cert.Subject, ","))
		//f.Attrs.Set("restlistener.remote.issuer.dn", certPKIXString(cert.Issuer, ","))
		f.Attrs.Set("restlistener.chain.0.user.dn", certPKIXString(cert.Subject, ","))
		f.Attrs.Set("restlistener.chain.0.issuer.dn", certPKIXString(cert.Issuer, ","))
	}
	if r != nil {
		f.Attrs.Set("restlistener.chain.0.request.uri", r.URL.Path)
		if host, port, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			//f.Attrs.Set("restlistener.remote.source.host", host)
			f.Attrs.Set("restlistener.chain.0.source.host", host)
			f.Attrs.Set("restlistener.chain.0.source.port", port)
		}
	} else {
		f.Attrs.Set("restlistener.chain.0.source", label)
	}
	if r != nil && r.TLS != nil {
		f.Attrs.Set("restlistener.chain.0.cipher", tls.CipherSuiteName(r.TLS.CipherSuite))
	}
}
