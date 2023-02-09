package main

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pschou/go-flowfile"
)

var (
	chain                = flag.Bool("update-chain", true, "Update the connection chain attributes: \"custodyChain.#.*\"\nTo disable use -update-chain=false")
	AdditionalAttributes []flowfile.Attribute
)

func loadAttributes(file string) {
	if file != "" {
		if fh, err := os.Open(file); err != nil {
			log.Fatal(err)
		} else {
			scanner := bufio.NewScanner(fh)
			fmt.Println("Loading additional attributes from file", file)
			for scanner.Scan() {
				parts := strings.SplitN(strings.TrimSpace(scanner.Text()), ":", 2)
				if len(parts) == 2 && !strings.HasPrefix(parts[0], "#") {
					AdditionalAttributes = append(AdditionalAttributes,
						flowfile.Attribute{
							Name:  strings.TrimSpace(parts[0]),
							Value: strings.TrimSpace(parts[1]),
						},
					)
				}
			}
			fh.Close()
			fmt.Println("Loaded", len(AdditionalAttributes), "attributes.")
		}
	}

}

func updateChain(f *flowfile.File, r *http.Request, label string) {
	var cert *x509.Certificate

	if r != nil && r.TLS != nil {
		if len(r.TLS.PeerCertificates) > 0 {
			cert = r.TLS.PeerCertificates[0]
		}
	}

	var updated []flowfile.Attribute
	for _, a := range AdditionalAttributes {
		f.Attrs.Set(a.Name, a.Value)
	}

	// Shift the current chain:
	for _, kv := range []flowfile.Attribute(f.Attrs) {
		if strings.HasPrefix(kv.Name, "custodyChain.") {
			parts := strings.SplitN(strings.TrimPrefix(kv.Name, "custodyChain."), ".", 2)
			if v, err := strconv.Atoi(parts[0]); err == nil {
				if len(parts) == 2 {
					kv.Name = fmt.Sprintf("custodyChain.%d.%s", v+1, parts[1])
				} else {
					kv.Name = fmt.Sprintf("custodyChain.%d", v+1)
				}
				updated = append(updated, kv)
			}
		} else {
			updated = append(updated, kv)
		}
	}
	f.Attrs = updated

	if label != "" {
		f.Attrs.Set("custodyChain.0.action", label)
	}
	// Set the current chain link
	f.Attrs.Set("custodyChain.0.time", time.Now().Format(time.RFC3339))

	if !*chain {
		return
	}
	if hn, err := os.Hostname(); err == nil {
		f.Attrs.Set("custodyChain.0.local.hostname", hn)
	}
	if cert != nil {
		//f.Attrs.Set("custody.remote.user.dn", certPKIXString(cert.Subject, ","))
		//f.Attrs.Set("custody.remote.issuer.dn", certPKIXString(cert.Issuer, ","))
		f.Attrs.Set("custodyChain.0.user.dn", certPKIXString(cert.Subject, ","))
		f.Attrs.Set("custodyChain.0.issuer.dn", certPKIXString(cert.Issuer, ","))
	}
	if r != nil {
		if r.RequestURI != "" {
			f.Attrs.Set("custodyChain.0.request.uri", r.RequestURI)
		}
		if host, port, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			//f.Attrs.Set("custody.remote.source.host", host)
			f.Attrs.Set("custodyChain.0.source.host", host)
			f.Attrs.Set("custodyChain.0.source.port", port)
		}
		if host, port, err := net.SplitHostPort(*listen); err == nil {
			//f.Attrs.Set("custody.remote.source.host", host)
			if host != "" {
				f.Attrs.Set("custodyChain.0.local.host", host)
			}
			f.Attrs.Set("custodyChain.0.local.port", port)
		}
		if r.TLS != nil {
			f.Attrs.Set("custodyChain.0.protocol", "HTTPS")
			f.Attrs.Set("custodyChain.0.tls.cipher", tls.CipherSuiteName(r.TLS.CipherSuite))
			f.Attrs.Set("custodyChain.0.tls.host", r.TLS.ServerName)
			var v string
			switch r.TLS.Version {
			case tls.VersionTLS10:
				v = "1.0"
			case tls.VersionTLS11:
				v = "1.1"
			case tls.VersionTLS12:
				v = "1.2"
			case tls.VersionTLS13:
				v = "1.3"
			default:
				v = fmt.Sprintf("0x%02x", r.TLS.Version)
			}
			f.Attrs.Set("custodyChain.0.tls.version", v)
		} else {
			f.Attrs.Set("custodyChain.0.protocol", "HTTP")
		}
	}
}
