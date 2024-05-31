package testutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// LocalTLSServerCert creates a root CA and server TLS certificate.
func LocalTLSServerCert() (*x509.CertPool, tls.Certificate, error) {
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}
	rootTemplate, err := certTemplate()
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("root cert template: %w", err)
	}
	// CA certificate.
	rootTemplate.IsCA = true
	rootTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	rootTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

	_, rootCert, err := cert(
		rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey,
	)
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("root cert: %w", err)
	}

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}
	serverTemplate, err := certTemplate()
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("server cert template: %w", err)
	}
	serverTemplate.KeyUsage = x509.KeyUsageDigitalSignature
	serverTemplate.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	serverTemplate.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1)}

	// Sign the cert using the root CA.
	serverCertDER, _, err := cert(
		serverTemplate, rootCert, &serverKey.PublicKey, rootKey,
	)
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("server cert: %w", err)
	}

	rootCACertPool := x509.NewCertPool()
	rootCACertPool.AddCert(rootCert)

	serverCertPEM := pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: serverCertDER,
	})
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey),
	})

	serverTLSCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("server key pair: %w", err)
	}

	return rootCACertPool, serverTLSCert, nil
}

func cert(
	template *x509.Certificate,
	parent *x509.Certificate,
	publicKey interface{},
	parentPrivateKey interface{},
) ([]byte, *x509.Certificate, error) {
	certDER, err := x509.CreateCertificate(
		rand.Reader, template, parent, publicKey, parentPrivateKey,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create cert: %w", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("parse cert: %w", err)
	}
	return certDER, cert, nil
}

func certTemplate() (*x509.Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	return &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"Piko"}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 30),
		BasicConstraintsValid: true,
	}, nil
}
