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
	"log"
	"net"
	"net/http"
	"time"

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
	certificate  *x509.Certificate
}

// NewDomain instantiates a new domain to be encrypted
func NewDomain(domain, api string) *Domain {
	return &Domain{Domain: domain, API: api}
}

// Bootstrap goes through the acme certificate request process for the domain
func (d *Domain) Bootstrap() error {
	log.Printf("action: beginning %q bootstrap\n", d.Domain)

	var err error
	// Create new let's encrypt client
	if d.client == nil {
		if d.client, err = letsencrypt.NewClient(d.API); err != nil {
			return fmt.Errorf("error: could not create letsencrypt client: %q\n", err)
		}
	}

	// see if we already have an auth key on disk
	if authBytes, err := ioutil.ReadFile("auth.key"); err == nil {
		log.Println("action: reading auth.key")
		authBlock, _ := pem.Decode(authBytes)
		if authBlock != nil {
			log.Println("action: decoding auth.key")
			d.AuthKey, err = x509.ParsePKCS1PrivateKey(authBlock.Bytes)
			if err != nil {
				log.Printf("error: found auth.key file: could not parse: %q\n", err)
			}
		}
	}

	// Try to create a new auth key if we don't have one
	if d.AuthKey == nil {
		log.Println("action: creating auth.key")
		// generate new auth key
		d.AuthKey, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return fmt.Errorf("error: could not generate auth key: %q\n", err)
		}

		// register new auth key
		log.Println("action: registering auth.key")
		d.Registration, err = d.client.NewRegistration(d.AuthKey)
		if err != nil {
			return fmt.Errorf("error: key registration failed: %q\n", err)
		}

		// request for authorization over the domain using this authkey
		log.Printf("action: requesting auth.key authorization over %q\n", d.Domain)
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
		log.Println("action: attempting authorization challenge")
		if err := d.completeChallenge(); err != nil {
			return err
		}

		// write auth key to file for later
		authPem := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA AUTH KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(d.AuthKey),
		})

		log.Println("action: writing auth.key to disk")
		if err := ioutil.WriteFile("auth.key", authPem, 0644); err != nil {
			return fmt.Errorf("error: could not write authkey.pem: %q\n", err)
		}
	}

	// if we already have a cert, refresh it
	log.Printf("action: attempting to read %q\n", d.Domain+".crt")
	if certBytes, err := ioutil.ReadFile(d.Domain + ".crt"); err == nil {
		if certPem, _ := pem.Decode(certBytes); certPem != nil {
			log.Println("action: decoding certificate")
			if cert, err := x509.ParseCertificate(certPem.Bytes); err == nil {
				d.certificate = cert
				log.Println("action: attempting certificate refresh")
				if err := d.refreshCertificate(); err == nil {
					return nil
				}
			}
		}
	}

	// request for certificate
	log.Println("action: requesting for new certificate")
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
	log.Println("action: creating new cert key")
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
	log.Println("action: creating new csr")
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &template, certKey)
	if err != nil {
		return fmt.Errorf("error: could not create CSR: %q\n", err)
	}

	// Save cert private key to file
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certKey),
	})

	log.Println("action: saving cert private key")
	if err := ioutil.WriteFile(d.Domain+".key", keyPem, 0644); err != nil {
		return fmt.Errorf("error: could not write privatekey.pem: %q\n", err)
	}

	// request new certificate request with let's encrypt
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return fmt.Errorf("error: could not parse csr: %q\n", err)
	}

	log.Println("action: requesting for new cert")
	reg, err := d.client.NewCertificate(d.AuthKey, csr)
	if err != nil {
		return fmt.Errorf("error: could not request for a new certificate: %q\n", err)
	}

	// write new certificate to disk
	log.Println("action: writing certificate to disk")
	if err := d.persistCertificate(reg.Certificate); err != nil {
		return fmt.Errorf("error: could not create certificate pem: %q\n", err)
	}

	// save it in memory
	cert, err := x509.ParseCertificate(reg.Certificate.Raw)
	if err != nil {
		return fmt.Errorf("error: could not parse certifcate: %q\n", err)
	}
	d.certificate = cert
	return nil
}

// RefreshCertificate refreshes the certificate every 30 days
func (d *Domain) RefreshCertificate() {
	for {
		<-time.After(time.Hour * 24 * 30)
		err := d.refreshCertificate()
		if err != nil {
			log.Printf("error: could not renew certificate: %q\n", err)
		}
	}
}

// persist certificate to disk
func (d *Domain) persistCertificate(cert *x509.Certificate) error {
	certPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err := ioutil.WriteFile(d.Domain+".crt", certPem, 0644); err != nil {
		return err
	}
	return nil
}

func (d *Domain) refreshCertificate() error {
	certResp, err := d.client.RenewCertificate("https://" + d.Domain)
	if err != nil {
		return err
	}
	// Renewal returned the same cert: we should request for a new one
	if certResp.Certificate.Equal(d.certificate) {
		if err := d.requestCertificate(); err != nil {
			return err
		}
	} else {
		// update in memory cert
		d.certificate = certResp.Certificate
		// save cert to disk
		return d.persistCertificate(certResp.Certificate)
	}
	return nil
}
