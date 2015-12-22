package main

import (
	"flag"
	"fmt"
	"homeautomation/apihelpers"
	"homeautomation/rf"
	"net/http"
	"strconv"
	"strings"
)

// Map RF Remote Buttons------------------------------------------------
var switches = []rf.Switch{
	rf.Switch{On: 1398067, Off: 1398076},
	rf.Switch{On: 1398211, Off: 1398220},
	rf.Switch{On: 1398531, Off: 1398540},
	rf.Switch{On: 1400067, Off: 1400076},
	rf.Switch{On: 1406211, Off: 1406220},
}

func main() {
	port := flag.String("port", "8080", "The port to run the server on")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/switch", switchHandler)
	http.ListenAndServe(":"+*port, mux)
}

// ---------------------------------------------------------------------
// RF Switches API Handler
// ---------------------------------------------------------------------
func switchHandler(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	switchNum := r.URL.Query().Get("switch")
	if state == "" || switchNum == "" {
		apihelpers.EncodeError(w, http.StatusBadRequest, "Missing Switch Number or State")
		return
	}

	// Get the correspodning switch to turn on
	intSwitchNum, err := strconv.Atoi(switchNum)
	if err != nil || intSwitchNum > (len(switches)-1) {
		apihelpers.EncodeError(w, http.StatusBadRequest, "Invalid Switch Number")
		return
	}

	// Get the state to set the switch to
	state = strings.ToLower(state)
	if state != "on" && state != "off" {
		apihelpers.EncodeError(w, http.StatusBadRequest, "Invalid State: must be \"on\" or \"off\"")
		return
	}

	// Get the code we want to transmit
	selectedSwitch := switches[intSwitchNum]
	code := selectedSwitch.On
	if state == "off" {
		code = selectedSwitch.Off
	}

	// Send the code
	err = rf.SendCode(code)

	// Handle errors from pilight
	if err != nil {
		apihelpers.EncodeError(w, http.StatusInternalServerError, "Unable to send RF Code")
		return
	}
	// Success!
	success := fmt.Sprintf("Successfully turned switch %d %s", intSwitchNum, state)
	apihelpers.EncodeJSON(w, http.StatusOK, map[string]string{"message": success})
}
