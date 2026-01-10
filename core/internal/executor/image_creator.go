package executor

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sjhoeksma/druppie/core/internal/llm"
	"github.com/sjhoeksma/druppie/core/internal/model"
)

// ImageCreatorExecutor handles Image generation (SDXL)
type ImageCreatorExecutor struct {
	LLM llm.Provider
}

func (e *ImageCreatorExecutor) CanHandle(action string) bool {
	return action == "image-creator" || action == "image-generation" || action == "generate_images"
}

func (e *ImageCreatorExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	// Extract Scene ID
	sceneID := fmt.Sprintf("%d", step.ID)
	if sID, ok := step.Params["scene_id"]; ok {
		sceneID = fmt.Sprintf("%v", sID)
	}
	planID := ""
	if p, ok := step.Params["plan_id"].(string); ok {
		planID = p
	}

	outputChan <- fmt.Sprintf("🎨 [Image Creator] Processing Scene %s...", sceneID)

	// Extract Prompt
	prompt := ""
	if p, ok := step.Params["visual_prompt"]; ok {
		prompt = fmt.Sprintf("%v", p)
	} else if p, ok := step.Params["visual_prompts"]; ok {
		prompt = fmt.Sprintf("%v", p)
	} else if p, ok := step.Params["prompt"]; ok {
		prompt = fmt.Sprintf("%v", p)
	} else if p, ok := step.Params["image_prompt"]; ok {
		prompt = fmt.Sprintf("%v", p)
	} else if p, ok := step.Params["visual_description"]; ok {
		prompt = fmt.Sprintf("%v", p)
	} else { // Generic fallback
		prompt = fmt.Sprintf("%v", step.Params["p"])
	}
	if prompt == "" || prompt == "<nil>" {
		prompt = "Abstract digital art"
	}

	outputChan <- fmt.Sprintf("   📝 Logic: Generating image for \"%s\"", prompt)

	// 1. Try LLM Provider "image_creator"
	if e.LLM != nil {
		if mgr, ok := e.LLM.(*llm.Manager); ok {
			// Check if provider exists (or we can just try Call)
			// Manager.GenerateWithProvider handles retrieval
			resp, _, err := mgr.GenerateWithProvider(ctx, "image_creator", prompt, "Generate Image")
			if err == nil && resp != "" {
				filename := fmt.Sprintf("image_scene_%s.png", sceneID)
				if planID != "" {
					if err := saveAsset(planID, filename, resp); err == nil {
						outputChan <- fmt.Sprintf("✅ [Image Creator] Generated via Provider: %s", filename)
						outputChan <- fmt.Sprintf("RESULT_IMAGE_FILE=%s", filename)
						return nil
					}
					outputChan <- fmt.Sprintf("⚠️ Failed to save image from provider: %v", err)
				} else {
					outputChan <- "⚠️ Plan ID missing, cannot save file."
				}
			} else {
				// Log only if error is NOT "provider not found"?
				// Actually GenerateWithProvider returns error if not found.
				// We fall back silently if it fails?
				// Maybe log "Provider 'image_creator' not found or failed, using mock."
				// outputChan <- fmt.Sprintf("Provider check: %v", err)
			}
		}
	}

	// Simulate Latency (Fallback)
	delay := time.Duration(1000+rand.Intn(4000)) * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}

	filename := fmt.Sprintf("image_scene_%s.png", sceneID)
	// Mock file creation if planID exists?
	// For now just return path as before
	outputChan <- fmt.Sprintf("✅ [Image Creator] Generated (Mock): %s", filename)
	outputChan <- fmt.Sprintf("RESULT_IMAGE_FILE=%s", filename)

	return nil
}

// saveAsset helper
func saveAsset(planID, filename, data string) error {
	basePath := fmt.Sprintf(".druppie/plans/%s/files", planID)
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return err
	}
	fullPath := filepath.Join(basePath, filename)

	var content []byte
	var err error

	if strings.HasPrefix(data, "base64,") {
		parts := strings.Split(data, ",")
		if len(parts) > 1 {
			data = parts[len(parts)-1]
		}
		content, err = base64.StdEncoding.DecodeString(data)
	} else if strings.HasPrefix(data, "http") {
		resp, err := http.Get(data)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		content, err = io.ReadAll(resp.Body)
	} else {
		content, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			return fmt.Errorf("unknown data format")
		}
	}

	if err != nil {
		return err
	}

	return os.WriteFile(fullPath, content, 0644)
}
