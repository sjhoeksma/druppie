package router

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sjhoeksma/druppie/core/internal/llm"
	"github.com/sjhoeksma/druppie/core/internal/model"
)

type Router struct {
	llm   llm.Provider
	Debug bool
}

func NewRouter(llm llm.Provider, debug bool) *Router {
	return &Router{llm: llm, Debug: debug}
}

const systemPrompt = `You are the Router Agent of the Druppie Platform.
Your job is to analyze the User's input and determine their Intent.
You must output a JSON object adhering to this schema:
{
  "summary": "Brief summary of what the user wants in the user's original language",
  "action": "create_project | update_project | query_registry | orchestrate_complex | general_chat",
  "category": "infrastructure | service | search | create content | unknown",
  "content_type": "video | blog | code | image | audio | ... (optional)",
  "language": "en | nl | fr | de"
}
Ensure the "action" is one of the allowed values.
Use "language" code to detect the user's input language.
IMPORTANT: The "summary" field MUST be in the correct language as detected in "language" code. Do NOT translate it to English.
Output ONLY valid JSON.`

func (r *Router) Analyze(ctx context.Context, input string) (model.Intent, error) {
	// Persistent Logging
	logFile := ".logs/ai_interaction.log"
	f, fileErr := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr == nil {
		defer f.Close()
		timestamp := time.Now().Format(time.RFC3339)
		f.WriteString(fmt.Sprintf("--- [Router] %s ---\nINPUT:\n%s\nUsers Input: %s\n", timestamp, systemPrompt, input))
	}

	resp, err := r.llm.Generate(ctx, input, systemPrompt)
	if err != nil {
		return model.Intent{}, fmt.Errorf("llm generation failed: %w", err)
	}

	if fileErr == nil {
		f.WriteString(fmt.Sprintf("OUTPUT:\n%s\n\n", resp))
	}

	// Simple cleanup if LLM adds markdown blocks
	// resp = strings.TrimPrefix(resp, "```json")
	// resp = strings.TrimSuffix(resp, "```")
	// For now assume mock returns clean JSON

	var intent model.Intent
	if err := json.Unmarshal([]byte(resp), &intent); err != nil {
		return model.Intent{}, fmt.Errorf("failed to parse router response: %w. Raw: %s", err, resp)
	}

	return intent, nil
}
