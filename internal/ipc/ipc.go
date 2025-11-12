package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

const SocketPath = "/tmp/vox.sock"

func StartServer(handler func(ControlMessage)) error {
	os.Remove(SocketPath)

	ln, err := net.Listen("unix", SocketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go handleConn(conn, handler)
		}
	}()

	return nil
}

func handleConn(conn net.Conn, handler func(ControlMessage)) {
	defer conn.Close()

	var msg ControlMessage
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&msg); err != nil {
		return
	}
	handler(msg)
}

func SendCommand(cmd string) error {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	enc := json.NewEncoder(conn)
	return enc.Encode(ControlMessage{Cmd: cmd})
}
