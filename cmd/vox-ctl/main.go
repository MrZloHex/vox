package main

import (
	"fmt"
	"vox/internal/ipc"
)

func main() {
	err := ipc.SendCommand("trigger")
	if err != nil {
		fmt.Println("vox-daemon not running:", err)
		return
	}
}
