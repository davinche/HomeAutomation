package alexa

import (
	"homeautomation/apihelpers"
	"homeautomation/rf"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/davinche/glexa"
)

// Handler receives AWS Alexa requests
func Handler(w http.ResponseWriter, r *http.Request) {
	glexa.VerifyRequest(handleAlexaCommands)(w, r)
}

var handleAlexaCommands http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	body, err := glexa.ParseBody(r.Body)
	if err != nil {
		log.Printf("error: could not parse alexa request body: %q\n", err)
		http.Error(w, "", http.StatusBadRequest)
	}
	handleSwitches(w, r, body)
}

// RF Switches Alexa Request Handler
func handleSwitches(w http.ResponseWriter, r *http.Request, b *glexa.Body) {
	response := glexa.NewResponse()
	response.Response.ShouldEndSession = true

	if b.Request.IsLaunch() {
		response.Tell("I did not understand what you were asking. Please try again.")
		apihelpers.EncodeJSON(w, http.StatusOK, response)
		return
	}

	if b.Request.IsSessionEnded() {
		apihelpers.EncodeJSON(w, http.StatusOK, response)
		return
	}

	switchNum, err := strconv.Atoi(b.Request.Intent.Slots["switch"].Value)
	if err != nil {
		response.Tell("An error has occurred. Please try again.")
		apihelpers.EncodeJSON(w, http.StatusOK, response)
		return
	}
	switchStatus := strings.ToLower(b.Request.Intent.Slots["state"].Value)
	response.Tell("Okay")
	apihelpers.EncodeJSON(w, http.StatusOK, response)

	// Log any errors if they occur
	err = rf.SetSwitch(switchNum, switchStatus)
	if err != nil {
		log.Printf("error: could not toggle switch %d %s", switchNum, switchStatus)
	}
}
