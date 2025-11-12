package vox

/*
import (
	"log/slog"
	"os"
)

type Vox struct {
	bus *Bus
	stt *STT
	nlu *NLU
}

func NewVox(bus *Bus) *Vox {
	whisperPath := os.Getenv("WHISPER_PATH")
	if whisperPath == "" {
		whisperPath = "/usr/local/bin/whisper"
	}

	return &Vox{
		bus: bus,
		stt: NewSTT(whisperPath),
		nlu: NewNLU(),
	}
}

func (v *Vox) Run() {
	slog.Info("Vox ready")

	for {
		msg, err := v.bus.Read()
		if err != nil {
			slog.Error("bus read failed", "error", err)
			continue
		}

		var text string

		if len(msg.Audio) > 0 {
			// Assume audio was saved by sender; can store to temp file if needed.
			slog.Info("Received audio message, ignoring content and running STT")
			// TODO: save msg.Audio to file and pass to whisper
			text = "[audio received — STT integration not yet fully implemented]"
		} else {
			text = msg.Content
		}

		reply, err := v.nlu.Process(text)
		if err != nil {
			slog.Error("NLU error", "error", err)
			continue
		}

		resp := &BusMessage{
			From:    "vox",
			To:      msg.From,
			Kind:    "reply",
			Content: reply,
		}

		if err := v.bus.Write(resp); err != nil {
			slog.Error("failed to send response", "error", err)
		}
	}
}
*/
