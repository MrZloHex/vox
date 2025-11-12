package nlu

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/openai/openai-go/v3"
)

type Result struct {
	Mode   string            `json:"mode"`
	Intent string            `json:"intent"`
	Args   map[string]string `json:"args"`
	Answer string            `json:"answer"`
}

const systemPrompt = `
You are VOX, NLU brain of the Monolith ecosystem.

You receive one user utterance (possibly in Russian or English).

You must respond with a SINGLE JSON object and NOTHING else.

The JSON format is:

{
  "mode": "command" | "question",
  "intent": "<short_snake_case_intent>",
  "args": { "<arg_name>": "<value>", ... },
  "answer": "<answer_text_or_empty>"
}

Rules:
- If the user is ASKING about something (info, explanation, etc.),
  then "mode" must be "question" and "answer" must contain a natural language reply.
- If the user is TELLING the assistant to do something, then "mode" must be "command".
- For "command" mode:
    - choose ONE best intent.
    - Fill "args" with any useful parameters (app names, times, free-form text, etc.).
    - "answer" can be a short confirmation, or an empty string.

Some example intents you can use (but you can invent more if needed):
- open_app
- run_shell
- search_web
- type_text
- set_timer
- get_weather
- get_time
- lamp_set_mode
- lamp_set_color
- lamp_set_brightness
- print_text
- print_status
`

func Analyze(client openai.Client, transcript string) (Result, error) {
	ctx := context.Background()

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(transcript),
		},
		Model: openai.ChatModelGPT5Nano,
	})
	if err != nil {
		return Result{}, fmt.Errorf("chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return Result{}, fmt.Errorf("no choices in response")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		return Result{}, fmt.Errorf("empty message content")
	}

	var out Result
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return Result{}, fmt.Errorf("unmarshal NLU result: %w (raw: %s)", err, content)
	}

	return out, nil
}
