package apihelpers

import (
	"encoding/json"
	"net/http"
)

// EncodeJSON takes an http.ResponseWriter, status code and message
// to stream a proper JSON response
func EncodeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	encoder := json.NewEncoder(w)
	encoder.Encode(data)
}

// EncodeError wraps a message inside an object with an "error" key
// and streams that error as a JSON response
func EncodeError(w http.ResponseWriter, code int, message interface{}) {
	data := make(map[string]interface{})
	data["error"] = message
	EncodeJSON(w, code, data)
}
