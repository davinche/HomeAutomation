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
