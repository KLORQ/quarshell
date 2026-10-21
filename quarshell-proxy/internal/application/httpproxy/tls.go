package httpproxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
)

// CA issues short-lived leaf certificates for TLS interception.
type CA struct {
	certificate *x509.Certificate
	der         []byte
	key         *ecdsa.PrivateKey
	pool        *x509.CertPool
}

// NewCA creates an in-memory test/interception CA.
func NewCA() (*CA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, domain.Wrap(err, "httpproxy.ca", "generate key")
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Quarshell Interception CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, domain.Wrap(err, "httpproxy.ca", "create CA certificate")
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, domain.Wrap(err, "httpproxy.ca", "parse CA certificate")
	}
	pool := x509.NewCertPool()
	pool.AddCert(certificate)
	return &CA{certificate: certificate, der: der, key: key, pool: pool}, nil
}

// Pool returns a pool containing only this CA.
func (ca *CA) Pool() *x509.CertPool {
	if ca == nil {
		return nil
	}
	return ca.pool
}

// CertificateFor returns a short-lived leaf certificate for host.
func (ca *CA) CertificateFor(host string) (tls.Certificate, error) {
	if ca == nil {
		return tls.Certificate{}, domain.Invalidf("httpproxy.ca", "CA is required")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, domain.Wrap(err, "httpproxy.ca", "generate leaf key")
	}
	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{host}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca.certificate, &key.PublicKey, ca.key)
	if err != nil {
		return tls.Certificate{}, domain.Wrap(err, "httpproxy.ca", "create leaf certificate")
	}
	return tls.Certificate{Certificate: [][]byte{der, ca.der}, PrivateKey: key}, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, domain.Wrap(err, "httpproxy.ca", "generate serial")
	}
	return serial, nil
}
