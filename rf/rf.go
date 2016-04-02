package rf

import (
	"fmt"
	"homeautomation/apihelpers"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

// Switch contains an "on" and an "off" attribute. These attributes
// are integer representations of the RF codes to send.
type Switch struct {
	On, Off int
}

// Convert a decimal representation of signal to pilight raw
func decimalToRaw(n int) string {
	rf0 := "174 522"
	rf1 := "522 174"
	rffoot := "174 5916"
	bin := make([]string, 0, 25)

	for n > 0 {
		if n%2 == 0 {
			bin = append(bin, rf0)
		} else {
			bin = append(bin, rf1)
		}
		n = n / 2
	}
	for len(bin) < 24 {
		bin = append(bin, rf0)
	}
	for i := 0; i < 12; i++ {
		bin[i], bin[23-i] = bin[23-i], bin[i]
	}
	bin = append(bin, rffoot)
	return strings.Join(bin, " ")
}

// SendCode executes the External "pilight-send" command given an rf code
func SendCode(code int) error {
	binCode := decimalToRaw(code)
	pilightArgs := []string{"-p", "raw", "-c", binCode}
	cmd := exec.Command("pilight-send", pilightArgs...)
	return cmd.Run()
}

// ----------------------------------------------------------------------------
// API Handler for switches
// ----------------------------------------------------------------------------
var switches = []Switch{
	Switch{On: 1398067, Off: 1398076},
	Switch{On: 1398211, Off: 1398220},
	Switch{On: 1398531, Off: 1398540},
	Switch{On: 1400067, Off: 1400076},
	Switch{On: 1406211, Off: 1406220},
}

// SwitchHandler is an HTTP Handler that deals with calls to turn switches on and off.
// Query Params Supported:
// state (string): "on | off"
// switch (int): which switch from the remote to use
func SwitchHandler(w http.ResponseWriter, r *http.Request) {
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
	err = SendCode(code)

	// Handle errors from pilight
	if err != nil {
		apihelpers.EncodeError(w, http.StatusInternalServerError, "Unable to send RF Code")
		return
	}
	// Success!
	success := fmt.Sprintf("Successfully turned switch %d %s", intSwitchNum, state)
	apihelpers.EncodeJSON(w, http.StatusOK, map[string]string{"message": success})
}
