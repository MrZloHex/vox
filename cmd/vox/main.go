package main

import (
	"log/slog"
	"os"

	"vox/internal/vox"
)

func main() {
	slog.Info("Starting Vox shard")

	wsURL := os.Getenv("BUS_URL")
	if wsURL == "" {
		wsURL = "ws://localhost:8092/ws"
	}

	bus, err := vox.NewBus(wsURL)
	if err != nil {
		slog.Error("failed to connect to bus", "error", err)
		os.Exit(1)
	}

	v := vox.NewVox(bus)
	v.Run()
}
