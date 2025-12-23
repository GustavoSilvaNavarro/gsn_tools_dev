package certificates

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

// KeyPair holds the private key and its PEM representation
type KeyPair struct {
	PrivateKey    *ecdsa.PrivateKey
	PrivateKeyPEM string
}

// CSRResult holds the CSR and its PEM representation
type CSRResult struct {
	CSR    *x509.CertificateRequest
	CSRPEM string
}

func GenerateCertsCmd() *cobra.Command {
	certCmd := cobra.Command{
		Use:   "csr <hash_algorithm>",
		Short: "Generates private key, csr and signed certificate to be used",
		Long:  "Generate a CSR and a signed certificate based on an specific hashing algorithm.",
		Run:   CertificateGeneration,
	}

	return &certCmd
}

// generateECDSAKeyPair generates an ECDSA key pair using P-256 curve
func generateECDSAKeyPair() (*KeyPair, error) {
	// Generate private key with P-256 curve
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Marshal private key to PKCS8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Encode to PEM
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	return &KeyPair{
		PrivateKey:    privateKey,
		PrivateKeyPEM: string(privateKeyPEM),
	}, nil
}

// createCSR creates a Certificate Signing Request
func createCSR(
	keyPair *KeyPair,
	commonName string,
	domainComponent *string,
	altNames []string,
	country string,
	state string,
	locality string,
	organization string,
	organizationalUnit *string,
) (*CSRResult, error) {
	// Build subject
	subject := pkix.Name{
		CommonName:   commonName,
		Country:      []string{country},
		Province:     []string{state},
		Locality:     []string{locality},
		Organization: []string{organization},
	}

	if organizationalUnit != nil {
		subject.OrganizationalUnit = []string{*organizationalUnit}
	}

	// Add domain component if provided
	if domainComponent != nil {
		// Domain Component OID: 0.9.2342.19200300.100.1.25
		dcOID := asn1.ObjectIdentifier{0, 9, 2342, 19200300, 100, 1, 25}
		subject.ExtraNames = []pkix.AttributeTypeAndValue{
			{
				Type:  dcOID,
				Value: *domainComponent,
			},
		}
	}

	// Build DNS names for SAN
	var dnsNames []string
	dnsNames = append(dnsNames, commonName)
	if altNames != nil {
		dnsNames = append(dnsNames, altNames...)
	}

	// Remove duplicates and sort
	uniqueDNS := make(map[string]bool)
	var finalDNS []string
	for _, name := range dnsNames {
		if !uniqueDNS[name] {
			uniqueDNS[name] = true
			finalDNS = append(finalDNS, name)
		}
	}

	// Create CSR template
	template := x509.CertificateRequest{
		Subject:            subject,
		SignatureAlgorithm: x509.ECDSAWithSHA256,
		DNSNames:           finalDNS,
	}

	// Create CSR
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, keyPair.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	// Parse CSR to get the object
	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSR: %w", err)
	}

	// Encode to PEM
	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})

	return &CSRResult{
		CSR:    csr,
		CSRPEM: string(csrPEM),
	}, nil
}

// signCSRToPKCS7 signs a CSR and returns a certificate in PKCS#7 format
func signCSRToPKCS7(
	csrPEM string,
	signingPrivateKey *ecdsa.PrivateKey,
	validityDays int,
) (string, error) {
	// Parse CSR from PEM
	block, _ := pem.Decode([]byte(csrPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode CSR PEM")
	}

	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse CSR: %w", err)
	}

	// Generate random serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Set validity period
	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 0, validityDays)

	// Create certificate template
	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               csr.Subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		BasicConstraintsValid: true,
		DNSNames:              csr.DNSNames,
	}

	// Add URIs if present in alt names
	for _, name := range csr.DNSNames {
		if u, err := url.Parse(name); err == nil && u.Scheme != "" {
			template.URIs = append(template.URIs, u)
		}
	}

	// Create self-signed certificate
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template, // Self-signed, so parent is same as template
		csr.PublicKey,
		signingPrivateKey,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Parse the certificate
	_, err = x509.ParseCertificate(certBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Create PKCS#7 structure
	// PKCS#7 ContentInfo structure
	type contentInfo struct {
		ContentType asn1.ObjectIdentifier
		Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
	}

	// PKCS#7 SignedData structure (simplified for certificate-only)
	type signedData struct {
		Version          int
		DigestAlgorithms asn1.RawValue
		ContentInfo      contentInfo
		Certificates     asn1.RawValue `asn1:"optional,tag:0"`
		CRLs             asn1.RawValue `asn1:"optional,tag:1"`
		SignerInfos      asn1.RawValue
	}

	// OIDs
	oidSignedData := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidData := asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1}

	// Empty digest algorithms
	emptySet, _ := asn1.Marshal([]interface{}{})

	// Content info with data type
	contentInfoData := contentInfo{
		ContentType: oidData,
	}

	// Wrap certificate in a SET
	certSet, err := asn1.Marshal([]asn1.RawValue{
		{FullBytes: certBytes},
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal certificate set: %w", err)
	}

	// Create signed data
	sd := signedData{
		Version:          1,
		DigestAlgorithms: asn1.RawValue{FullBytes: emptySet},
		ContentInfo:      contentInfoData,
		Certificates:     asn1.RawValue{FullBytes: certSet, Class: 2, Tag: 0, IsCompound: true},
		SignerInfos:      asn1.RawValue{FullBytes: emptySet},
	}

	// Marshal signed data
	signedDataBytes, err := asn1.Marshal(sd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal signed data: %w", err)
	}

	// Create outer content info
	ci := contentInfo{
		ContentType: oidSignedData,
		Content: asn1.RawValue{
			Class:      2,
			Tag:        0,
			IsCompound: true,
			Bytes:      signedDataBytes,
		},
	}

	// Marshal final PKCS#7
	pkcs7DER, err := asn1.Marshal(ci)
	if err != nil {
		return "", fmt.Errorf("failed to marshal PKCS#7: %w", err)
	}

	// Base64 encode
	pkcs7Base64 := base64.StdEncoding.EncodeToString(pkcs7DER)

	return pkcs7Base64, nil
}

func CertificateGeneration(cmd *cobra.Command, args []string) {
	// Generate key pair
	keyPair, err := generateECDSAKeyPair()
	if err != nil {
		fmt.Printf("Error generating key pair: %v\n", err)
		return
	}
	fmt.Println("Generated ECDSA key pair with P-256 curve")

	// Uncomment to print private key
	fmt.Printf("Private key PEM:\n%s\n", keyPair.PrivateKeyPEM)

	// Create CSR
	domainComponent := "CSO"
	csrResult, err := createCSR(
		keyPair,
		"USAMZS00000000000000000000001372070201E",
		&domainComponent,
		[]string{"https://www.powerflex.com"},
		"US",
		"California",
		"San Diego",
		"AMZ",
		nil,
	)
	if err != nil {
		fmt.Printf("Error creating CSR: %v\n", err)
		return
	}

	fmt.Printf("CSR PEM:\n%s\n", csrResult.CSRPEM)

	// Sign CSR and get PKCS#7 certificate
	pkcs7Cert, err := signCSRToPKCS7(csrResult.CSRPEM, keyPair.PrivateKey, 365)
	if err != nil {
		fmt.Printf("Error signing CSR: %v\n", err)
		return
	}

	fmt.Printf("PKCS#7 Certificate (Base64):\n%s\n", pkcs7Cert)
}
