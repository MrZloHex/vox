package notify

import (
	"os/exec"
)

func SwayNotify(text string) {
	cmd := exec.Command("notify-send", "VOX", text)
	_ = cmd.Run()
}
