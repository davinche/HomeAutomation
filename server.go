package main

import (
	"encoding/json"
	"flag"
	"homeautomation/alexa"
	"homeautomation/ddns"
	"homeautomation/encrypt"
	"homeautomation/rf"
	"log"
	"net/http"
	"os"
)

type config struct {
	Cloudflare struct {
		Email  string `json:"email"`
		Domain string `json:"domain"`
		APIKey string `json:"apiKey"`
		Record string `json:"record"`
	} `json:"cloudflare"`
	LetsEncrypt struct {
		API string
	} `json:"letsencrypt"`
}

func getConfig() *config {
	c := config{}
	configFile, err := os.Open("config.json")
	if err != nil {
		log.Fatalf("error: could not read config file: %q\n", err)
	}

	configDecoder := json.NewDecoder(configFile)
	err = configDecoder.Decode(&c)

	if err != nil {
		log.Fatalf("error: could not decode config file: %q\n", err)
	}
	return &c
}

func main() {
	port := flag.String("port", "8080", "The port to run the server on")
	flag.Parse()

	// Read the config
	config := getConfig()

	// DDNS
	log.Println("STARTING: DDNS Updater")
	go ddns.NewUpdater(
		config.Cloudflare.Email,
		config.Cloudflare.APIKey,
		config.Cloudflare.Domain,
		config.Cloudflare.Record,
	).Update()

	// Bootstrap the domain
	domainStr := config.Cloudflare.Record + "." + config.Cloudflare.Domain
	domain := encrypt.NewDomain(domainStr, config.LetsEncrypt.API)
	log.Println("STARTING: Let's Encrypt Bootstrap")
	err := domain.Bootstrap()

	if err != nil {
		log.Println(err)
	}

	// Go Renew in 30 days
	log.Println("STARTING: Let's Encrypt 30 Day Refresh")
	go domain.RefreshCertificate()

	// API Handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/api/switch", rf.SwitchHandler)

	// HTTP Server
	log.Println("STARTING: Raspberry PI Homeautomation API Server")
	go func() {
		log.Fatal(http.ListenAndServe(":"+*port, mux))
	}()

	// HTTPS
	smux := http.NewServeMux()

	// Alexa!
	smux.HandleFunc("/alexa", alexa.Handler)

	log.Println("STARTING: Alexa Handler")
	log.Fatal(http.ListenAndServeTLS(":31415",
		domainStr+".crt",
		domainStr+".key",
		smux,
	))
}
