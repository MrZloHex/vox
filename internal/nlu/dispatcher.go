package nlu

import (
	"fmt"
	"vox/pkg/protocol"
)

func Dispatch(cmd Result, ptcl *protocol.Protocol) (string, error) {
	var verb, noun, to string
	switch cmd.Intent {
	case "turn_on":
		verb = "ON"
	case "turn_off":
		verb = "OFF"
	default:
		return "", fmt.Errorf("Unknown intnent")
	}

	switch cmd.Entities["device"] {
	case "lamp":
		to = "VERTEX"
		noun = "LAMP"
	default:
		return "", fmt.Errorf("Unknown device")
	}

	msg, err := ptcl.TransmitReceive([]string{to, noun, verb})
	if err != nil {
		return "", err
	}

	return msg.String(), nil
}
