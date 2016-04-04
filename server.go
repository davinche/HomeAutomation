package main

import (
	"encoding/json"
	"flag"
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
	go ddns.NewUpdater(
		config.Cloudflare.Email,
		config.Cloudflare.APIKey,
		config.Cloudflare.Domain,
		config.Cloudflare.Record,
	).Update()

	domain := config.Cloudflare.Record + "." + config.Cloudflare.Domain
	err := encrypt.
		NewDomain(domain).Bootstrap(config.LetsEncrypt.API)

	if err != nil {
		log.Println(err)
	}

	// API Handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/api/switch", rf.SwitchHandler)

	// HTTP Server
	go func() {
		log.Fatal(http.ListenAndServe(":"+*port, mux))
	}()

	// HTTPS (Alexa)
	log.Fatal(http.ListenAndServeTLS(":10443",
		domain+".crt",
		domain+".key",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte("hello"))
		}),
	))
}
