package main

import (
	"flag"
	"homeautomation/rf"
	"net/http"
)

func main() {
	port := flag.String("port", "8080", "The port to run the server on")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/switch", rf.SwitchHandler)
	http.ListenAndServe(":"+*port, mux)
}
