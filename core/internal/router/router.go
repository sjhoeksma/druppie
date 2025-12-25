package router

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/drug-nl/druppie/core/internal/llm"
	"github.com/drug-nl/druppie/core/internal/model"
)

type Router struct {
	llm llm.Provider
}

func NewRouter(llm llm.Provider) *Router {
	return &Router{llm: llm}
}

const systemPrompt = `You are the Router Agent of the Druppie Platform.
Your job is to analyze the User's input and determine their Intent.
You must output a JSON object adhering to this schema:
{
  "summary": "Brief summary of what the user wants",
  "action": "create_project | update_project | query_registry | orchestrate_complex | general_chat",
  "category": "infrastructure | service | unknown",
  "language": "en | nl | fr | de"
}
Ensure the "action" is one of the allowed values.
Use "language" to detect the user's input language.
Output ONLY valid JSON.`

func (r *Router) Analyze(ctx context.Context, input string) (model.Intent, error) {
	resp, err := r.llm.Generate(ctx, input, systemPrompt)
	if err != nil {
		return model.Intent{}, fmt.Errorf("llm generation failed: %w", err)
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
