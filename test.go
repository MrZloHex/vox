package main

import (
	"context"
	"fmt"
	"os"

	openai "github.com/openai/openai-go"
)

func main() {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		panic("OPENAI_API_KEY is not set")
	}

	client := openai.NewClient(key)

	// Простой запрос к GPT-5 nano
	resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model: openai.F("gpt-5-nano"), // ← твоя модель
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.ChatCompletionUserMessageParam{
				Content: openai.F([]openai.ChatCompletionContentPartUnion{
					openai.ChatCompletionContentPartText{
						Text: openai.F("Привет, расскажи коротко что ты умеешь?"),
					},
				}),
			},
		}),
	})

	if err != nil {
		panic(err)
	}

	fmt.Println(resp.Choices[0].Message.Content[0].Text)
}
