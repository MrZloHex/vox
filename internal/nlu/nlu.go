package nlu

import (
	"context"
	"encoding/json"
	"fmt"
	log "log/slog"

	openai "github.com/openai/openai-go/v3"
)

type Result struct {
	Intent   string            `json:"intent"`
	Entities map[string]string `json:"entities"`
	Query    string            `json:"query"`
}

const systemPrompt = `
You are VOX-NLU — the intent classifier for the Monolith system.
Your ONLY job is to convert the user’s utterance into a minimal structured JSON.

GENERAL RULES:
1. Do NOT converse.
2. Do NOT answer the question.
3. Do NOT add explanations.
4. Output ONLY JSON. No markdown.
5. Never hallucinate unknown devices or parameters.

OUTPUT FORMAT:
{
  "intent": "<string>",
  "entities": { ... },
  "query": "<original user text>"
}

INTENTS (canonical, snake_case):
- "turn_on"
- "turn_off"
- "set_brightness"
- "set_mode"
- "set_time"
- "stop"
- "unknown"  (if not classifiable)

ENTITIES (strict canonical schema):
{
  "device": "<canonical ID or null>",
  "brightness": <int or null>,
  "time": "<string or null>",
  "date": "<string or null>",
}

DEVICE REGISTRY (canonical identifiers):
- "lamp"          = ночник, лампа, night lamp, desk lamp, подсветка
- "led"           = союз печать, союз печаль
- "timer"         = термометр, погода, терми, weather display
- "alarm"         = колонки, аудио, sound, speakers

RULES FOR DEVICES:
- Map ANY synonyms to the canonical id.
- If multiple devices mentioned — choose the MAIN one (the one acted upon).
- If no device is relevant — output null for device.

ENTITY NORMALIZATION:
- brightness/volume must be 0–255 integers if present.
- colors: canonicalize to simple English: "red", "blue", "warm_white", etc.
- time/date: keep raw phrase ("tomorrow", "at 7", "in the evening").
- Never invent missing values.

If the meaning is unclear → intent = "unknown".

Be strict and minimal.
Do not generate text other than the JSON.
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

	log.Debug("Processed", "data", content)

	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return Result{}, fmt.Errorf("unmarshal NLU result: %w (raw: %s)", err, content)
	}

	return out, nil
}
