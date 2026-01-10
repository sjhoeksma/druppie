package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/sjhoeksma/druppie/core/internal/model"
)

// StableDiffusionProvider implements the LLMProvider interface for Stable Diffusion.
// ./webui.sh --listen --port 7860 --api
type StableDiffusionProvider struct {
	BaseURL string
	Model   string // Optional: checkpoint name if API supports switching via payload
}

func NewStableDiffusionProvider(baseURL, modelName string) *StableDiffusionProvider {
	// Normalize URL
	baseURL = strings.TrimSuffix(baseURL, "/")
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "http://" + baseURL
	}
	return &StableDiffusionProvider{
		BaseURL: baseURL,
		Model:   modelName,
	}
}

type sdText2ImgRequest struct {
	Prompt         string `json:"prompt"`
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Steps          int    `json:"steps,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	CfgScale       int    `json:"cfg_scale,omitempty"`
	// Add other fields as needed
}

type sdText2ImgResponse struct {
	Images []string `json:"images"`
	Info   string   `json:"info"`
}

func (p *StableDiffusionProvider) Generate(ctx context.Context, prompt string, systemPrompt string) (string, model.TokenUsage, error) {
	// Default params
	req := sdText2ImgRequest{
		Prompt:   prompt,
		Steps:    20,
		Width:    512,
		Height:   512,
		CfgScale: 7,
	}

	// Parse System Prompt for overrides
	lines := strings.Split(systemPrompt, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			switch key {
			case "negative prompt", "negative_prompt":
				req.NegativePrompt = val
			case "steps":
				if n, err := strconv.Atoi(val); err == nil {
					req.Steps = n
				}
			case "width":
				if n, err := strconv.Atoi(val); err == nil {
					req.Width = n
				}
			case "height":
				if n, err := strconv.Atoi(val); err == nil {
					req.Height = n
				}
			case "cfg_scale", "cfg scale": // Corrected from "cfg", "scale" syntax error
				if n, err := strconv.Atoi(val); err == nil {
					req.CfgScale = n
				}
			}
		}
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return "", model.TokenUsage{}, err
	}

	url := fmt.Sprintf("%s/sdapi/v1/txt2img", p.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", model.TokenUsage{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", model.TokenUsage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", model.TokenUsage{}, fmt.Errorf("SD API Error %d: %s", resp.StatusCode, string(body))
	}

	var parsed sdText2ImgResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", model.TokenUsage{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(parsed.Images) == 0 {
		return "", model.TokenUsage{}, fmt.Errorf("no images returned")
	}

	// Return first image as base64 data URI
	// SD API returns raw base64 string
	return fmt.Sprintf("base64,%s", parsed.Images[0]), model.TokenUsage{}, nil
}

func (p *StableDiffusionProvider) Close() error {
	return nil
}
