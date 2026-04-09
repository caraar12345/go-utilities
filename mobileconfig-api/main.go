package main

import (
	"bytes"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"text/template"
	"time"

	"go.mozilla.org/pkcs7"
)

var serialNumberRe = regexp.MustCompile(`^[A-Za-z0-9\-]+$`)

const acmePlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>PayloadCertificateFileName</key>
			<string>root_ca.crt</string>
			<key>PayloadContent</key>
			<data>{{.RootCABase64}}</data>
			<key>PayloadDescription</key>
			<string>Adds a CA root certificate</string>
			<key>PayloadDisplayName</key>
			<string>aaroncarson Step Root CA</string>
			<key>PayloadIdentifier</key>
			<string>com.apple.security.root.4152BEA7-BFC4-4F0A-900E-48FF67A80D60</string>
			<key>PayloadType</key>
			<string>com.apple.security.root</string>
			<key>PayloadUUID</key>
			<string>4152BEA7-BFC4-4F0A-900E-48FF67A80D60</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
		<dict>
			<key>AllowAllAppsAccess</key>
			<true/>
			<key>Attest</key>
			<true/>
			<key>ClientIdentifier</key>
			<string>{{.SerialNumber}}</string>
			<key>DirectoryURL</key>
			<string>https://vault.srv.adhd.energy/acme/acme-da/directory</string>
			<key>ExtendedKeyUsage</key>
			<array>
				<string>1.3.6.1.5.5.7.3.2</string>
			</array>
			<key>HardwareBound</key>
			<true/>
			<key>KeySize</key>
			<integer>384</integer>
			<key>KeyType</key>
			<string>ECSECPrimeRandom</string>
			<key>PayloadDisplayName</key>
			<string>ACME Certificate</string>
			<key>PayloadIdentifier</key>
			<string>D27D2AAF-9FEE-4413-8CD6-0D68495BD98E</string>
			<key>PayloadType</key>
			<string>com.apple.security.acme</string>
			<key>PayloadUUID</key>
			<string>D27D2AAF-9FEE-4413-8CD6-0D68495BD98E</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
			<key>Subject</key>
			<array>
				<array>
					<array>
						<string>CN</string>
						<string>{{.SerialNumber}}</string>
					</array>
				</array>
			</array>
		</dict>
	</array>
	<key>PayloadDisplayName</key>
	<string>aaroncarson Step ACME MDA</string>
	<key>PayloadIdentifier</key>
	<string>42AC914A-DFFF-488D-8D91-741666017028</string>
	<key>PayloadScope</key>
	<string>System</string>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>42AC914A-DFFF-488D-8D91-741666017028</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`

const rootCAPlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>PayloadCertificateFileName</key>
			<string>root_ca.crt</string>
			<key>PayloadContent</key>
			<data>{{.RootCABase64}}</data>
			<key>PayloadDescription</key>
			<string>Adds a CA root certificate</string>
			<key>PayloadDisplayName</key>
			<string>aaroncarson Step Root CA</string>
			<key>PayloadIdentifier</key>
			<string>com.apple.security.root.FA618494-A2B0-4296-9D90-B8067775371E</string>
			<key>PayloadType</key>
			<string>com.apple.security.root</string>
			<key>PayloadUUID</key>
			<string>FA618494-A2B0-4296-9D90-B8067775371E</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
	</array>
	<key>PayloadDisplayName</key>
	<string>aaroncarson Step Root CA</string>
	<key>PayloadIdentifier</key>
	<string>F9B2D8D0-E8C4-41A2-BC64-CFAACC033EB7</string>
	<key>PayloadScope</key>
	<string>System</string>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>F9B2D8D0-E8C4-41A2-BC64-CFAACC033EB7</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`

type srv struct {
	token      string
	mu         sync.RWMutex
	signingCert *x509.Certificate
	signingKey  crypto.PrivateKey
	chainCerts  []*x509.Certificate
	rootCACert  *x509.Certificate
	rootCAB64   string
	stepCAURL  string
	stepCAPool *x509.CertPool
}

func main() {
	token := mustEnv("MOBILECONFIG_TOKEN")
	signingCerts, signingKey := loadCertChainAndKey(mustEnv("SIGN_CERT_FILE"), mustEnv("SIGN_KEY_FILE"))
	rootCACert, rootCAB64 := loadRootCA(mustEnv("STEP_ROOT_FILE"))

	pool := x509.NewCertPool()
	pool.AddCert(rootCACert)

	s := &srv{
		token:       token,
		signingCert: signingCerts[0],
		signingKey:  signingKey,
		chainCerts:  signingCerts[1:],
		rootCACert:  rootCACert,
		rootCAB64:   rootCAB64,
		stepCAURL:   envOr("STEP_CA_URL", "https://vault.srv.adhd.energy"),
		stepCAPool:  pool,
	}

	go s.startRenewalLoop()

	mux := http.NewServeMux()
	mux.HandleFunc("/acme.mobileconfig", s.handleACME)
	mux.HandleFunc("/rootca.mobileconfig", s.handleRootCA)

	addr := envOr("LISTEN_ADDR", ":8765")
	log.Printf("Listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func (s *srv) authenticate(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get("token") != s.token {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return false
	}
	return true
}

func (s *srv) handleACME(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(w, r) {
		return
	}
	serialNumber := r.URL.Query().Get("serial_number")
	if serialNumber == "" {
		http.Error(w, "serial_number query parameter required", http.StatusBadRequest)
		return
	}
	if !serialNumberRe.MatchString(serialNumber) {
		http.Error(w, "invalid serial_number", http.StatusBadRequest)
		return
	}

	tmpl := template.Must(template.New("acme").Parse(acmePlistTmpl))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"RootCABase64": s.rootCAB64,
		"SerialNumber": serialNumber,
	}); err != nil {
		log.Printf("acme template error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	signed, err := s.sign(buf.Bytes())
	if err != nil {
		log.Printf("signing error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Write(signed)
}

func (s *srv) handleRootCA(w http.ResponseWriter, r *http.Request) {
	if !s.authenticate(w, r) {
		return
	}

	tmpl := template.Must(template.New("rootca").Parse(rootCAPlistTmpl))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"RootCABase64": s.rootCAB64,
	}); err != nil {
		log.Printf("rootca template error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	signed, err := s.sign(buf.Bytes())
	if err != nil {
		log.Printf("signing error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-apple-aspen-config")
	w.Write(signed)
}

func (s *srv) sign(data []byte) ([]byte, error) {
	s.mu.RLock()
	cert, key, chain := s.signingCert, s.signingKey, s.chainCerts
	s.mu.RUnlock()

	sd, err := pkcs7.NewSignedData(data)
	if err != nil {
		return nil, fmt.Errorf("creating signed data: %w", err)
	}
	sd.SetDigestAlgorithm(asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}) // SHA-256
	if err := sd.AddSigner(cert, key, pkcs7.SignerInfoConfig{}); err != nil {
		return nil, fmt.Errorf("adding signer: %w", err)
	}
	for _, c := range chain {
		sd.AddCertificate(c)
	}
	s.mu.RLock()
	sd.AddCertificate(s.rootCACert)
	s.mu.RUnlock()
	return sd.Finish()
}

// startRenewalLoop checks every hour whether the signing cert needs renewal,
// and renews it when less than ⅓ of its lifetime remains.
func (s *srv) startRenewalLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.RLock()
		cert := s.signingCert
		s.mu.RUnlock()

		lifetime := cert.NotAfter.Sub(cert.NotBefore)
		remaining := time.Until(cert.NotAfter)
		if remaining < lifetime/3 {
			log.Printf("signing cert expires in %v (lifetime %v), renewing", remaining.Round(time.Minute), lifetime.Round(time.Minute))
			if err := s.renewCert(); err != nil {
				log.Printf("cert renewal failed: %v", err)
			}
		}
	}
}

// renewCert calls Step CA's /1.0/renew endpoint, authenticating with the
// current signing certificate via mTLS. On success it atomically swaps in
// the new cert and key.
func (s *srv) renewCert() error {
	s.mu.RLock()
	currentCert := s.signingCert
	currentKey := s.signingKey
	chainCerts := s.chainCerts
	s.mu.RUnlock()

	// Build a tls.Certificate from the current x509.Certificate + key.
	// We need the raw DER bytes for tls.Certificate.Certificate.
	leafDER := currentCert.Raw
	tlsCertBytes := [][]byte{leafDER}
	for _, c := range chainCerts {
		tlsCertBytes = append(tlsCertBytes, c.Raw)
	}
	tlsCert := tls.Certificate{
		Certificate: tlsCertBytes,
		PrivateKey:  currentKey,
		Leaf:        currentCert,
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				RootCAs:      s.stepCAPool,
			},
		},
	}

	resp, err := client.Post(s.stepCAURL+"/1.0/renew", "application/json", nil)
	if err != nil {
		return fmt.Errorf("POST /1.0/renew: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("renew returned %d: %s", resp.StatusCode, body)
	}

	var result struct {
		CRT struct {
			Raw string `json:"raw"`
		} `json:"crt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding renew response: %w", err)
	}

	newCerts, newKey, err := parseCertChainAndKey([]byte(result.CRT.Raw), currentKey)
	if err != nil {
		return fmt.Errorf("parsing renewed cert: %w", err)
	}

	s.mu.Lock()
	s.signingCert = newCerts[0]
	s.signingKey = newKey
	s.chainCerts = newCerts[1:]
	s.mu.Unlock()

	log.Printf("signing cert renewed: serial=%s notAfter=%s", newCerts[0].SerialNumber, newCerts[0].NotAfter.Format(time.RFC3339))
	return nil
}

// parseCertChainAndKey decodes a PEM cert chain. The key is carried over from
// the previous certificate (Step CA renewal reuses the existing key pair).
func parseCertChainAndKey(certPEM []byte, key crypto.PrivateKey) ([]*x509.Certificate, crypto.PrivateKey, error) {
	var certs []*x509.Certificate
	for len(certPEM) > 0 {
		var block *pem.Block
		block, certPEM = pem.Decode(certPEM)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no certificates in PEM")
	}
	return certs, key, nil
}

// loadCertChainAndKey reads a PEM file that may contain one or more certificates
// (e.g. leaf + intermediate), and a separate PEM private key file.
// Returns all certs in order (leaf first) and the private key.
func loadCertChainAndKey(certFile, keyFile string) ([]*x509.Certificate, crypto.PrivateKey) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		log.Fatalf("reading cert file %s: %v", certFile, err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		log.Fatalf("reading key file %s: %v", keyFile, err)
	}

	var certs []*x509.Certificate
	for len(certPEM) > 0 {
		var block *pem.Block
		block, certPEM = pem.Decode(certPEM)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			log.Fatalf("parsing certificate from %s: %v", certFile, err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		log.Fatalf("no certificates found in %s", certFile)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		log.Fatalf("failed to decode key PEM from %s", keyFile)
	}
	var key crypto.PrivateKey
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	default:
		log.Fatalf("unsupported key type %q in %s", keyBlock.Type, keyFile)
	}
	if err != nil {
		log.Fatalf("parsing private key from %s: %v", keyFile, err)
	}

	return certs, key
}

func loadRootCA(pemFile string) (*x509.Certificate, string) {
	data, err := os.ReadFile(pemFile)
	if err != nil {
		log.Fatalf("reading root CA file %s: %v", pemFile, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		log.Fatalf("failed to decode PEM from %s", pemFile)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Fatalf("parsing root CA from %s: %v", pemFile, err)
	}
	return cert, base64.StdEncoding.EncodeToString(block.Bytes)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
