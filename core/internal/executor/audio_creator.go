package executor

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/sjhoeksma/druppie/core/internal/llm"
	"github.com/sjhoeksma/druppie/core/internal/model"
)

// AudioCreatorExecutor handles TTS generation
type AudioCreatorExecutor struct {
	LLM llm.Provider
}

func (e *AudioCreatorExecutor) CanHandle(action string) bool {
	return action == "audio-creator" || action == "text-to-speech"
}

func (e *AudioCreatorExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	// Extract Scene ID for naming
	sceneID := fmt.Sprintf("%d", step.ID)
	if sID, ok := step.Params["scene_id"]; ok {
		sceneID = fmt.Sprintf("%v", sID)
	} else if sID, ok := step.Params["scene"]; ok {
		sceneID = fmt.Sprintf("%v", sID)
	}
	planID := ""
	if p, ok := step.Params["plan_id"].(string); ok {
		planID = p
	}

	outputChan <- fmt.Sprintf("🎙️ [Audio Creator] Processing Scene %s...", sceneID)

	// Extract params
	text := ""
	if t, ok := step.Params["audio_text"]; ok {
		text = fmt.Sprintf("%v", t)
	} else if t, ok := step.Params["audio_texts"]; ok {
		text = fmt.Sprintf("%v", t)
	} else if t, ok := step.Params["script_segment"]; ok {
		text = fmt.Sprintf("%v", t) // Fallback
	}

	voice := "Default"
	if v, ok := step.Params["voice_profile"]; ok {
		voice = fmt.Sprintf("%v", v)
	}

	outputChan <- fmt.Sprintf("   📝 Logic: Generating speech for \"%s\" (Voice: %s)", text, voice)

	// Try LLM Provider "text_to_speech"
	if e.LLM != nil {
		if mgr, ok := e.LLM.(*llm.Manager); ok {
			resp, _, err := mgr.GenerateWithProvider(ctx, "audio_creator", text, "Generate Audio")
			if err == nil && resp != "" {
				filename := fmt.Sprintf("audio_scene_%s.mp3", sceneID)
				if planID != "" {
					if err := saveAsset(planID, filename, resp); err == nil {
						outputChan <- fmt.Sprintf("✅ [Audio Creator] Generated via Provider: %s", filename)
						outputChan <- fmt.Sprintf("RESULT_AUDIO_FILE=%s", filename)
						return nil
					}
					outputChan <- fmt.Sprintf("⚠️ Failed to save audio from provider: %v", err)
				} else {
					outputChan <- "⚠️ Plan ID missing, cannot save file."
				}
			}
		}
	}

	// Simulate Latency (1-5s)
	delay := time.Duration(1000+rand.Intn(4000)) * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}

	// Calculate fake duration based on text length (rough calc: 15 chars ~ 1 sec)
	durationSeconds := len(text) / 15
	if durationSeconds < 2 {
		durationSeconds = 2
	}
	durationStr := fmt.Sprintf("%ds", durationSeconds)

	filename := fmt.Sprintf("audio_scene_%s.mp3", sceneID)
	// Mock file creation if planID exists?
	outputChan <- fmt.Sprintf("✅ [Audio Creator] Generated (Mock): %s (Duration: %s)", filename, durationStr)

	// Return structural result
	outputChan <- fmt.Sprintf("RESULT_DURATION=%s", durationStr)
	outputChan <- fmt.Sprintf("RESULT_AUDIO_FILE=%s", filename)

	return nil
}
