package vox

import (
	"context"
	"log/slog"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

type NLU struct {
	client *openai.Client
}

func NewNLU() *NLU {
	apiKey := os.Getenv("OPENAI_API_KEY")
	return &NLU{
		client: openai.NewClient(apiKey),
	}
}

func (n *NLU) Process(text string) (string, error) {
	resp, err := n.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: "gpt-5-nano",
			Messages: []openai.ChatCompletionMessage{
				{Role: "system", Content: "You are Vox, a friendly voice assistant in the Monolith system."},
				{Role: "user", Content: text},
			},
		},
	)
	if err != nil {
		return "", err
	}

	slog.Info("NLU response ready")
	return resp.Choices[0].Message.Content, nil
}
