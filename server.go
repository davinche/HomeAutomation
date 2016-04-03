package main

import (
	"encoding/json"
	"flag"
	"github.com/kardianos/osext"
	"homeautomation/ddns"
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
}

func getConfig() *config {
	c := config{}
	dir, _ := osext.ExecutableFolder()
	configFile, err := os.Open(dir + "/config.json")
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

	// API Handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/api/switch", rf.SwitchHandler)

	// HTTP Server
	http.ListenAndServe(":"+*port, mux)
}
