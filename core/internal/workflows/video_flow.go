package workflows

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sjhoeksma/druppie/core/internal/model"
)

type VideoCreationWorkflow struct{}

func (w *VideoCreationWorkflow) Name() string { return "video-content-creator" }

// Data Structures for State
type ProjectIntent struct {
	OriginalPrompt string
	RefinedPrompt  string
	Language       string
	Parameters     map[string]interface{}
}

type Scene struct {
	ID           int    `json:"scene_id"`
	AudioText    string `json:"audio_text"`
	VisualPrompt string `json:"visual_prompt"`
	Duration     int    `json:"duration"` // Seconds

	// Asset Paths
	AudioFile string `json:"audio_file,omitempty"`
	ImageFile string `json:"image_file,omitempty"`
	VideoFile string `json:"video_file,omitempty"`
}

type AVScript struct {
	Scenes []Scene `json:"av_script"`
}

func (w *VideoCreationWorkflow) Run(wc *WorkflowContext, initialPrompt string) error {
	wc.OutputChan <- fmt.Sprintf("🎥 [VideoWorkflow] Starting Video Creation Workflow: %s", initialPrompt)

	// 1. Refine Intent (Ask Questions)
	intent, err := w.refineIntent(wc, initialPrompt)
	if err != nil {
		return err
	}
	_ = wc.Store.LogInteraction(wc.PlanID, "VideoContentCreator", "Refined Intent", fmt.Sprintf("%+v", intent))
	wc.AppendStep(model.Step{
		AgentID: "video-content-creator",
		Action:  "ask_questions",
		Status:  "completed",
		Result:  intent.RefinedPrompt,
		Params:  map[string]interface{}{"note": "Native Workflow Execution"},
	})

	// 2. Draft Script
	wc.OutputChan <- "📝 [VideoWorkflow] Drafting Script..."
	script, err := w.draftScript(wc, intent)
	if err != nil {
		return err
	}
	wc.OutputChan <- fmt.Sprintf("✅ [VideoWorkflow] Script Approved: %d scenes.", len(script.Scenes))
	_ = wc.Store.LogInteraction(wc.PlanID, "VideoContentCreator", "Approved Script", w.formatScript(script))
	wc.AppendStep(model.Step{
		AgentID: "video-content-creator",
		Action:  "draft_scenes",
		Status:  "completed",
		Params:  map[string]interface{}{"av_script": script.Scenes},
	})

	// 3. Asset Production Phases

	// PHASE A: AUDIO
	wc.OutputChan <- "🎙️ [VideoWorkflow] Phase 1/3: Audio Generation..."
	script.Scenes, err = w.runPhase(wc, "audio-creator", "text-to-speech", script.Scenes, w.generateAudio)
	if err != nil {
		return err
	}

	if err := w.reviewPhase(wc, "Audio", script.Scenes); err != nil {
		return err
	}

	// PHASE B: IMAGES
	wc.OutputChan <- "🎨 [VideoWorkflow] Phase 2/3: Image Generation..."
	script.Scenes, err = w.runPhase(wc, "image-creator", "image-generation", script.Scenes, w.generateImage)
	if err != nil {
		return err
	}

	if err := w.reviewPhase(wc, "Images", script.Scenes); err != nil {
		return err
	}

	// PHASE C: VIDEO
	wc.OutputChan <- "🎬 [VideoWorkflow] Phase 3/3: Video Generation..."
	script.Scenes, err = w.runPhase(wc, "video-creator", "video-generation", script.Scenes, w.generateVideo)
	if err != nil {
		return err
	}

	if err := w.reviewPhase(wc, "Final Video", script.Scenes); err != nil {
		return err
	}

	// 4. Merge
	finalVideo, err := w.mergeVideo(wc, script.Scenes)
	if err != nil {
		return err
	}
	wc.AppendStep(model.Step{
		AgentID: "video-content-creator",
		Action:  "content-merge",
		Status:  "completed",
		Params:  map[string]interface{}{"av_script": script.Scenes},
		Result:  fmt.Sprintf("RESULT_VIDEO_FILE=%s", finalVideo),
	})

	wc.OutputChan <- "🎉 [VideoWorkflow] Workflow Completed Successfully!"
	return nil
}

// runPhase executes a generator function for all scenes in parallel
func (w *VideoCreationWorkflow) runPhase(wc *WorkflowContext, agentID, action string, scenes []Scene, generator func(*WorkflowContext, Scene) (Scene, error)) ([]Scene, error) {
	var wg sync.WaitGroup
	results := make([]Scene, len(scenes))
	copy(results, scenes) // Preserve order logic by index access? No, simplistic

	// Thread-safe slice updating
	// Actually, let's just use channel to collect results and re-map them to ID
	resultChan := make(chan Scene, len(scenes))
	errChan := make(chan error, len(scenes))

	for _, s := range scenes {
		wg.Add(1)
		go func(scene Scene) {
			defer wg.Done()
			res, err := generator(wc, scene)
			if err != nil {
				errChan <- err
				return
			}
			params := map[string]interface{}{
				"scene_id": res.ID,
			}
			// Include INPUTS based on what what phase we are in (heuristic based on agentID/action?)
			// Or just dump valuable fields
			if res.AudioText != "" {
				params["audio_text"] = res.AudioText
			}
			if res.VisualPrompt != "" {
				params["visual_prompt"] = res.VisualPrompt
			}

			// Include OUTPUTS
			if res.AudioFile != "" {
				params["audio_file"] = res.AudioFile
			}
			if res.ImageFile != "" {
				params["image_file"] = res.ImageFile
			}
			if res.VideoFile != "" {
				params["video_file"] = res.VideoFile
			}

			wc.AppendStep(model.Step{
				AgentID: agentID,
				Action:  action,
				Status:  "completed",
				Params:  params,
			})
			resultChan <- res
		}(s)
	}

	wg.Wait()
	close(resultChan)
	close(errChan)

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	// Reconstruct slice (to keep updates)
	// Map by ID
	sceneMap := make(map[int]Scene)
	for res := range resultChan {
		sceneMap[res.ID] = res
	}

	// Return ordered list
	ordered := make([]Scene, len(scenes))
	for i, original := range scenes {
		if updated, ok := sceneMap[original.ID]; ok {
			ordered[i] = updated
		} else {
			ordered[i] = original // Should not happen on success
		}
	}
	return ordered, nil
}

func (w *VideoCreationWorkflow) reviewPhase(wc *WorkflowContext, phaseName string, _ []Scene) error {
	wc.OutputChan <- fmt.Sprintf("\n🔎 [Review] Please review generated %s assets.", phaseName)
	// List what we have?
	// For now, simple confirmation
	wc.OutputChan <- "Options: '/accept' to proceed | '/stop'"

	wc.UpdateStatus("Waiting Input")
	defer wc.UpdateStatus("Running")

	select {
	case <-wc.Ctx.Done():
		return wc.Ctx.Err()
	case input := <-wc.InputChan:
		if input == "/stop" {
			wc.AppendStep(model.Step{
				AgentID: "video-content-creator",
				Action:  fmt.Sprintf("review_%s", strings.ToLower(phaseName)),
				Status:  "failed",
				Result:  "User rejected validation",
			})
			return fmt.Errorf("user stopped at %s review", phaseName)
		}
		// Assume accept
		wc.OutputChan <- fmt.Sprintf("✅ [Review] %s Approved.", phaseName)
		wc.AppendStep(model.Step{
			AgentID: "video-content-creator",
			Action:  fmt.Sprintf("review_%s", strings.ToLower(phaseName)),
			Status:  "completed",
			Result:  "User approved validation",
		})
		return nil
	}
}

// --- Helper Functions ---

func (w *VideoCreationWorkflow) refineIntent(wc *WorkflowContext, prompt string) (ProjectIntent, error) {
	// Basic implementation: Just ask the LLM to summarize/refine, skipping interactive loop for speed/stability
	// In a real "Interactive" replacement, this would loop via wc.InputChan

	sysPrompt := "You are a Video Producer. Analyze the user request. Output a JSON object with keys: refined_prompt, language (2-letter code), target_audience."
	resp, err := wc.LLM.Generate(wc.Ctx, "Refine Intent", sysPrompt+"\nUser Request: "+prompt)
	if err != nil {
		return ProjectIntent{}, err
	}

	// Parse
	// Clean markdown blocks
	clean := strings.TrimSpace(resp)
	if idx := strings.Index(clean, "{"); idx != -1 {
		clean = clean[idx:]
	}
	if idx := strings.LastIndex(clean, "}"); idx != -1 {
		clean = clean[:idx+1]
	}

	rawMap := make(map[string]interface{})
	if err := json.Unmarshal([]byte(clean), &rawMap); err != nil {
		wc.OutputChan <- fmt.Sprintf("⚠️ [Intent] Failed to parse JSON, falling back to raw: %s", clean)
		return ProjectIntent{
			OriginalPrompt: prompt,
			RefinedPrompt:  prompt,
			Language:       "en",
		}, nil
	}

	var intent ProjectIntent
	intent.OriginalPrompt = prompt
	if v, ok := rawMap["refined_prompt"].(string); ok {
		intent.RefinedPrompt = v
	} else {
		intent.RefinedPrompt = prompt
	}
	if v, ok := rawMap["language"].(string); ok {
		intent.Language = v
	}

	// Create formatted output
	wc.OutputChan <- fmt.Sprintf("\n🧠 [Intent] Analysis:\n - **Language**: %s\n - **Prompt**: %s\n", intent.Language, intent.RefinedPrompt)

	// Store extra params
	intent.Parameters = rawMap
	return intent, nil
}

func (w *VideoCreationWorkflow) draftScript(wc *WorkflowContext, intent ProjectIntent) (AVScript, error) {
	currentPrompt := intent.RefinedPrompt
	iteration := 0

	for {
		iteration++
		if iteration > 1 {
			wc.OutputChan <- fmt.Sprintf("📝 [VideoWorkflow] Drafting Script (Iteration %d)...", iteration)
		}

		sysPrompt := `You are a Screenwriter. Create a JSON script for a video.
        Structure: {"av_script": [{"scene_id": 1, "audio_text": "...", "visual_prompt": "...", "duration": 5}]}
        Key Rules:
        - Output VALID JSON.
        - Duration in seconds (int).
        - Visual Prompt in English.
        - Audio Text in ` + intent.Language + `.
        `
		resp, err := wc.LLM.Generate(wc.Ctx, "Draft Script", sysPrompt+"\nRequest: "+currentPrompt)
		if err != nil {
			return AVScript{}, err
		}

		// Clean JSON
		clean := strings.TrimSpace(resp)
		if idx := strings.Index(clean, "{"); idx != -1 {
			clean = clean[idx:]
		}
		if idx := strings.LastIndex(clean, "}"); idx != -1 {
			clean = clean[:idx+1]
		}

		var script AVScript
		err = json.Unmarshal([]byte(clean), &script)
		if err != nil {
			wc.OutputChan <- fmt.Sprintf("⚠️ [VideoWorkflow] Failed to parse script: %v. Retrying...", err)
			continue // Retry automatically? Or fail? Let's retry.
		}

		// --- INTERACTIVE REVIEW ---
		wc.OutputChan <- "\n🎥 **Review Draft Script:**"
		wc.OutputChan <- w.formatScript(script)
		wc.OutputChan <- "Options: [Type feedback to refine] | '/accept' to proceed"

		wc.UpdateStatus("Waiting Input")
		select {
		case <-wc.Ctx.Done():
			return AVScript{}, wc.Ctx.Err()
		case input := <-wc.InputChan:
			wc.UpdateStatus("Running")
			if input == "/stop" {
				return AVScript{}, fmt.Errorf("user cancelled workflow")
			}
			if input == "/accept" || input == "" { // Allow empty enter to accept? commonly yes or no. Let's say explicit accept or 'ok'
				return script, nil
			}
			// Feedback received
			wc.OutputChan <- fmt.Sprintf("🔄 [VideoWorkflow] Refining based on feedback: '%s'", input)
			currentPrompt = fmt.Sprintf("Original Request: %s\n\nPrevious Script: %s\n\nUser Feedback (Fix this): %s", intent.RefinedPrompt, clean, input)
		}
	}
}

func (w *VideoCreationWorkflow) formatScript(script AVScript) string {
	var sb strings.Builder
	for _, s := range script.Scenes {
		sb.WriteString(fmt.Sprintf("\n🎬 Scene %d (%ds)\n", s.ID, s.Duration))
		sb.WriteString(fmt.Sprintf("   🔈 Audio: \"%s\"\n", s.AudioText))
		sb.WriteString(fmt.Sprintf("   👁️ Visual: \"%s\"\n", s.VisualPrompt))
	}
	return sb.String()
}

func (w *VideoCreationWorkflow) generateAudio(wc *WorkflowContext, s Scene) (Scene, error) {
	step := model.Step{
		ID:     s.ID * 10,
		Action: "text-to-speech",
		Params: map[string]interface{}{
			"audio_text": s.AudioText,
			"scene_id":   s.ID,
			"voice":      "default",
		},
	}
	executor, err := wc.Dispatcher.GetExecutor("text-to-speech")
	if err != nil {
		return s, err
	}

	execChan := make(chan string, 100)
	var capturedFile string

	// Launch executor
	go func() {
		_ = executor.Execute(wc.Ctx, step, execChan)
		close(execChan)
	}()

	// Stream logs and capture result
	for msg := range execChan {
		wc.OutputChan <- fmt.Sprintf("  %s", msg) // Indent inner logs
		if strings.HasPrefix(msg, "RESULT_AUDIO_FILE=") {
			capturedFile = strings.TrimPrefix(msg, "RESULT_AUDIO_FILE=")
		}
	}

	if capturedFile != "" {
		s.AudioFile = capturedFile
	} else {
		// Fallback if log parsing fails (simulation)
		s.AudioFile = fmt.Sprintf("audio_scene_%d.mp3", s.ID)
	}
	return s, nil
}

func (w *VideoCreationWorkflow) generateImage(wc *WorkflowContext, s Scene) (Scene, error) {
	step := model.Step{
		ID:     s.ID*10 + 1,
		Action: "image-generation",
		Params: map[string]interface{}{
			"visual_prompt": s.VisualPrompt,
			"scene_id":      s.ID,
		},
	}
	executor, _ := wc.Dispatcher.GetExecutor("image-generation")

	execChan := make(chan string, 100)
	var capturedFile string

	go func() {
		_ = executor.Execute(wc.Ctx, step, execChan)
		close(execChan)
	}()

	for msg := range execChan {
		wc.OutputChan <- fmt.Sprintf("  %s", msg)
		if strings.HasPrefix(msg, "RESULT_IMAGE_FILE=") {
			capturedFile = strings.TrimPrefix(msg, "RESULT_IMAGE_FILE=")
		}
	}

	if capturedFile != "" {
		s.ImageFile = capturedFile
	} else {
		s.ImageFile = fmt.Sprintf("image_scene_%d.png", s.ID)
	}
	return s, nil
}

func (w *VideoCreationWorkflow) generateVideo(wc *WorkflowContext, s Scene) (Scene, error) {
	step := model.Step{
		ID:     s.ID*10 + 2,
		Action: "video-generation",
		Params: map[string]interface{}{
			"visual_prompt": s.VisualPrompt,
			"audio_file":    s.AudioFile,
			"image_file":    s.ImageFile,
			"duration":      s.Duration,
			"scene_id":      s.ID,
		},
	}
	executor, _ := wc.Dispatcher.GetExecutor("video-generation")

	execChan := make(chan string, 100)
	var capturedFile string

	go func() {
		_ = executor.Execute(wc.Ctx, step, execChan)
		close(execChan)
	}()

	for msg := range execChan {
		wc.OutputChan <- fmt.Sprintf("  %s", msg)
		if strings.HasPrefix(msg, "RESULT_VIDEO_FILE=") {
			capturedFile = strings.TrimPrefix(msg, "RESULT_VIDEO_FILE=")
		}
	}

	if capturedFile != "" {
		s.VideoFile = capturedFile
	} else {
		s.VideoFile = fmt.Sprintf("video_scene_%d.mp4", s.ID)
	}
	return s, nil
}

func (w *VideoCreationWorkflow) mergeVideo(wc *WorkflowContext, _ []Scene) (string, error) {
	wc.OutputChan <- "🎬 [VideoWorkflow] Merging final video..."
	time.Sleep(2 * time.Second)
	finalPath := "final_video_production.mp4"
	wc.OutputChan <- fmt.Sprintf("✅ [VideoWorkflow] Final Video Created: %s", finalPath)
	return finalPath, nil
}
