package vox

import (
	"bytes"
	"os/exec"
)

// STT uses WhisperCPP via command line.
type STT struct {
	ExecPath string
}

func NewSTT(path string) *STT {
	return &STT{ExecPath: path}
}

// Transcribe runs whisper.cpp and returns recognized text.
func (s *STT) Transcribe(audioFile string) (string, error) {
	cmd := exec.Command(s.ExecPath, "-m", "models/ggml-base.en.bin", "-f", audioFile, "-otxt")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return out.String(), nil
}
