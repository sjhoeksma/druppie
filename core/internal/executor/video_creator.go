package executor

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/sjhoeksma/druppie/core/internal/llm"
	"github.com/sjhoeksma/druppie/core/internal/model"
)

// VideoCreatorExecutor handles Visual generation and assembly
type VideoCreatorExecutor struct {
	LLM llm.Provider
}

func (e *VideoCreatorExecutor) CanHandle(action string) bool {
	return action == "video-creator" || action == "video-generation"
}

func (e *VideoCreatorExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	// Extract Scene ID
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

	outputChan <- fmt.Sprintf("🎥 [Video Creator] Processing Scene %s...", sceneID)

	// Extract params
	visual := ""
	if v, ok := step.Params["visual_description"]; ok {
		visual = fmt.Sprintf("%v", v)
	} else if v, ok := step.Params["visual_prompt"]; ok {
		visual = fmt.Sprintf("%v", v)
	} else if v, ok := step.Params["visual_prompts"]; ok {
		visual = fmt.Sprintf("%v", v)
	}

	duration := "5s"
	if d, ok := step.Params["duration"]; ok {
		duration = fmt.Sprintf("%v", d)
	} else if d, ok := step.Params["audio_duration"]; ok {
		duration = fmt.Sprintf("%v", d)
	}

	audioFile := ""
	if f, ok := step.Params["audio_file"]; ok {
		audioFile = fmt.Sprintf("%v", f)
	}

	imageFile := ""
	if f, ok := step.Params["image_file"]; ok {
		imageFile = fmt.Sprintf("%v", f)
	}

	outputChan <- fmt.Sprintf("   👀 Visual: \"%s\"", visual)
	if imageFile != "" {
		outputChan <- fmt.Sprintf("   🖼️ Starting Image: %s", imageFile)
	}
	outputChan <- fmt.Sprintf("   ⏱️ Duration: %s", duration)

	if audioFile != "" {
		outputChan <- fmt.Sprintf("   🎵 Synced to: %s", audioFile)
	} else {
		outputChan <- "   ⚠️ No Audio ID provided, using default pacing."
	}

	// Try LLM Provider "video_creator"
	if e.LLM != nil {
		if mgr, ok := e.LLM.(*llm.Manager); ok {
			prompt := fmt.Sprintf("Create a video based on: %s", visual)
			resp, _, err := mgr.GenerateWithProvider(ctx, "video_creator", prompt, "Generate Video")
			if err == nil && resp != "" {
				filename := fmt.Sprintf("video_scene_%s.mp4", sceneID)
				if planID != "" {
					if err := saveAsset(planID, filename, resp); err == nil {
						outputChan <- fmt.Sprintf("✅ [Video Creator] Generated via Provider: %s", filename)
						outputChan <- fmt.Sprintf("RESULT_VIDEO_FILE=%s", filename)
						return nil
					}
					outputChan <- fmt.Sprintf("⚠️ Failed to save video from provider: %v", err)
				} else {
					outputChan <- "⚠️ Plan ID missing, cannot save file."
				}
			}
		}
	}

	outputChan <- "   ⚙️ sending to ai-video-comfyui..."
	// Simulate Latency (1-5s)
	delay := time.Duration(1000+rand.Intn(4000)) * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}

	filename := fmt.Sprintf("video_scene_%s.mp4", sceneID)
	// Mock
	outputChan <- fmt.Sprintf("✅ [Video Creator] Asset Generated (Mock): %s", filename)
	outputChan <- fmt.Sprintf("RESULT_VIDEO_FILE=%s", filename)

	return nil
}
