package encrypt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/ericchiang/letsencrypt"
)

// Domain is the struct represntation of the domain to encrypt
type Domain struct {
	API          string
	Domain       string
	Challenge    letsencrypt.Challenge
	Registration letsencrypt.Registration
	AuthKey      *rsa.PrivateKey
	client       *letsencrypt.Client
}

// NewDomain instantiates a new domain to be encrypted
func NewDomain(domain string) *Domain {
	return &Domain{Domain: domain}
}

// Bootstrap goes through the acme certificate request process for the domain
func (d *Domain) Bootstrap(api string) error {
	d.API = api
	var err error
	// Create new let's encrypt client
	d.client, err = letsencrypt.NewClient(api)
	if err != nil {
		return fmt.Errorf("error: could not create letsencrypt client: %q\n", err)
	}

	// generate new auth key
	d.AuthKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("error: could not generate account key: %q\n", err)
	}

	// register new auth key
	d.Registration, err = d.client.NewRegistration(d.AuthKey)
	if err != nil {
		return fmt.Errorf("error: key registration failed: %q\n", err)
	}

	// request for authorization
	auth, _, err := d.client.NewAuthorization(d.AuthKey, "dns", d.Domain)
	if err != nil {
		return fmt.Errorf("error: could not request for authorization: %q\n", err)
	}

	// we want to complete the http-01 challenge
	for _, challenge := range auth.Challenges {
		if challenge.Type == "http-01" {
			d.Challenge = challenge
			break
		}
	}
	// try to complete the challenge
	if err := d.completeChallenge(); err != nil {
		return err
	}

	// request for certificate
	if err := d.requestCertificate(); err != nil {
		return err
	}
	return nil
}

func (d *Domain) completeChallenge() error {
	path, resource, err := d.Challenge.HTTP(d.AuthKey)
	if err != nil {
		return fmt.Errorf("error: could not complete challenge: %q\n", err)
	}

	l, err := net.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("error: could not create http listener for challenge: %q\n", err)
	}
	defer l.Close()

	// Spin Up http server to handle the challenge
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != path {
				http.NotFound(w, r)
				return
			}
			io.WriteString(w, resource)
		})
		http.Serve(l, mux)
	}()

	if err = d.client.ChallengeReady(d.AuthKey, d.Challenge); err != nil {
		return fmt.Errorf("error: failed challenge: %q\n", err)
	}
	return nil
}

func (d *Domain) requestCertificate() error {
	// create new certificate private key
	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("error: could not generate private key: %q\n", err)
	}

	// create the certifcate request template
	template := x509.CertificateRequest{
		SignatureAlgorithm: x509.SHA256WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		PublicKey:          &certKey.PublicKey,
		Subject:            pkix.Name{CommonName: d.Domain},
		DNSNames:           []string{d.Domain},
	}

	// Create new certifcate request
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, certKey)
	if err != nil {
		return fmt.Errorf("error: could not create CSR: %q\n", err)
	}

	// Save cert private key to file
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certKey),
	})

	if err := ioutil.WriteFile(d.Domain+".key", keyPem, 0644); err != nil {
		return fmt.Errorf("error: could not write privatekey.pem: %q\n", err)
	}

	// request new certificate request with let's encrypt
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return fmt.Errorf("error: could not parse csr: %q\n", err)
	}

	reg, err := d.client.NewCertificate(d.AuthKey, csr)
	if err != nil {
		return fmt.Errorf("error: could not request for a new certificate: %q\n", err)
	}

	// write new certificate to disk
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: reg.Certificate.Raw})
	if err := ioutil.WriteFile(d.Domain+".crt", certPem, 0644); err != nil {
		return fmt.Errorf("error: could not create certificate pem: %q\n", err)
	}

	return nil
}
