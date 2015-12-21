package rf

import (
	"os/exec"
	"strings"
)

// Switch contains an "on" and an "off" attribute. These attributes
// are integer representations of the RF codes to send.
type Switch struct {
	On, Off int
}

// Raw Codes for Wave "on" and "off" -----------------------------------
var rf0 = "174 522"
var rf1 = "522 174"
var rffoot = "174 5916"

// Convert a decimal representation of signal to pilight raw -----------
func decimalToRaw(n int) string {
	var bin []string
	var rfcode string
	for n > 0 {
		if n%2 == 0 {
			rfcode = rf0
		} else {
			rfcode = rf1
		}
		bin = append(bin, rfcode)
		n = n / 2
	}

	for len(bin) < 24 {
		bin = append(bin, rf0)
	}

	binLen := len(bin)
	var binFinal = make([]string, binLen+1, binLen+1)
	for i := 0; i < binLen; i++ {
		binFinal[i] = bin[binLen-i-1]
	}
	binFinal = append(binFinal, rffoot)
	return strings.Join(binFinal, " ")
}

// SendCode executes the External "pilight-send" command given an rf code
func SendCode(code int) error {
	binCode := decimalToRaw(code)
	pilightArgs := []string{"-p", "raw", "-c", binCode}
	cmd := exec.Command("pilight-send", pilightArgs...)
	return cmd.Run()
}
