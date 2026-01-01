package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sjhoeksma/druppie/core/internal/builder"
	"github.com/sjhoeksma/druppie/core/internal/config"
	"github.com/sjhoeksma/druppie/core/internal/llm"
	"github.com/sjhoeksma/druppie/core/internal/model"
	"github.com/sjhoeksma/druppie/core/internal/planner"
	"github.com/sjhoeksma/druppie/core/internal/registry"
	"github.com/sjhoeksma/druppie/core/internal/router"
	"github.com/sjhoeksma/druppie/core/internal/store"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "druppie-core",
		Short: "Druppie Core Helper CLI & API Server",
		Long: `Druppie Core manages the Registry, Planner, and Orchestration API. 
By default, it starts the API Server on port 8080. 
Use global flags like --plan-id to resume existing planning tasks or --llm-provider to switch backends.`,
	}

	// CLI Flags
	var llmProviderOverride string
	var buildProviderOverride string
	var debug bool
	var planID string

	// Register commands
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newCliCmd())

	// Helper to bootstrap dependencies
	// Returns ConfigManager to allow updates, and Builder Engine
	setup := func(_ *cobra.Command) (*config.Manager, *registry.Registry, *router.Router, *planner.Planner, builder.BuildEngine, error) {
		// Check if we are in 'core' and need to move up to project root
		cwd, _ := os.Getwd()
		if filepath.Base(cwd) == "core" {
			_ = os.Chdir("..")
		}

		rootDir, err := findProjectRoot()

		fmt.Printf("Loading registry from: %s\n", rootDir)
		reg, err := registry.LoadRegistry(rootDir)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("registry load error: %w", err)
		}

		// Initialize Store (Central .druppie dir for all persistence)
		storeDir := filepath.Join(rootDir, ".druppie")
		druppieStore, err := store.NewFileStore(storeDir)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("store init error: %w", err)
		}

		// Load Configuration from Store
		cfgMgr, err := config.NewManager(druppieStore)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("config load error: %w", err)
		}
		cfg := cfgMgr.Get()

		// Apply Overrides
		if llmProviderOverride != "" {
			fmt.Printf("Overriding LLM Provider to: %s\n", llmProviderOverride)
			cfg.LLM.DefaultProvider = llmProviderOverride
		}
		if buildProviderOverride != "" {
			fmt.Printf("Overriding Build Provider to: %s\n", buildProviderOverride)
			cfg.Build.DefaultProvider = buildProviderOverride
		}

		// Initialize Build Engine
		buildEngine, err := builder.NewEngine(cfg.Build)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("builder init error: %w", err)
		}

		// Initialize LLM with Config
		llmManager, err := llm.NewManager(context.Background(), cfg.LLM)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("llm init error: %w", err)
		}

		r := router.NewRouter(llmManager, druppieStore, debug)
		p := planner.NewPlanner(llmManager, reg, druppieStore, debug)

		return cfgMgr, reg, r, p, buildEngine, nil
	}

	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the Druppie Core API Server",
		Run: func(cmd *cobra.Command, args []string) {
			cfgMgr, reg, routerService, plannerService, buildEngine, err := setup(cmd)
			if err != nil {
				fmt.Printf("Startup Error: %v\n", err)
				os.Exit(1)
			}
			tm := NewTaskManager(plannerService)
			cfg := cfgMgr.Get()

			// Start log drainer to:
			// 1. Unblock the buffered channel
			// 2. Print to server console (stdout)
			// 3. Persist to store logs for UI
			go func() {
				// Simple regex to extract Plan ID: [plan-12345]
				// Note: using regexp.MustCompile outside loop would be better but this is fine
				for {
					select {
					case msg := <-tm.OutputChan:
						//fmt.Println(msg) // Output to console for visibility

						// Extract Plan ID roughly
						var planID string
						// Find "plan-"
						if idx := strings.Index(msg, "plan-"); idx != -1 {
							// Check previous char is [ or space? Not strictly necessary if ID is unique enough
							// Let's take next 15-20 chars until ] or space
							rest := msg[idx:]
							endIdx := strings.IndexAny(rest, "] ")
							if endIdx != -1 {
								planID = rest[:endIdx]
							} else {
								planID = rest
							}
							// Sanitize ID (remove trailing punctuation)
							planID = strings.TrimRight(planID, ".,;:!?\")")
						}

						// Save to store
						_ = plannerService.Store.AppendRawLog(planID, msg)

					case <-tm.TaskDoneChan:
						// Just consume
					}
				}
			}()

			r := chi.NewRouter()
			// Custom Error-Only Logger (Silences 200 OKs)
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
					t1 := time.Now()
					defer func() {
						// Log only if status >= 400 and not 404 (suppress Not Found noise)
						if ww.Status() >= 400 && ww.Status() != 404 {
							fmt.Printf("[HTTP] %s %s -> %d (%dB) in %s\n",
								r.Method, r.URL.Path, ww.Status(), ww.BytesWritten(), time.Since(t1))
						}
					}()
					next.ServeHTTP(ww, r)
				})
			})
			r.Use(middleware.Recoverer)

			// API Routes
			r.Route("/v1", func(r chi.Router) {
				r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("OK"))
				})

				// Registry Endpoints
				r.Get("/registry", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					blocks := reg.ListBuildingBlocks()
					json.NewEncoder(w).Encode(blocks)
				})

				// Chat / Intent Endpoint
				r.Post("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
					var req struct {
						Prompt string `json:"prompt"`
						PlanID string `json:"plan_id"` // Optional: Context for continuing chat
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						http.Error(w, "Invalid request body", http.StatusBadRequest)
						return
					}

					var currentPlan model.ExecutionPlan
					var planID string
					isNewPlan := true

					// Check if we are continuing an existing conversation
					if req.PlanID != "" {
						if p, err := plannerService.Store.GetPlan(req.PlanID); err == nil {
							currentPlan = p
							planID = p.ID
							isNewPlan = false
						}
					}

					if isNewPlan {
						// Create new plan skeleton
						planID = fmt.Sprintf("plan-%d", time.Now().Unix())
						currentPlan = model.ExecutionPlan{
							ID: planID,
							Intent: model.Intent{
								InitialPrompt: req.Prompt,
								Prompt:        req.Prompt,
							},
							Status: "completed",
							Steps:  []model.Step{},
						}
					} else {
						// Update status of existing plan to indicate activity
						currentPlan.Status = "running"
					}

					// Store User Input as a Step (for both New and Existing plans)
					newStepID := 1
					if len(currentPlan.Steps) > 0 {
						newStepID = currentPlan.Steps[len(currentPlan.Steps)-1].ID + 1
					}
					currentPlan.Steps = append(currentPlan.Steps, model.Step{
						ID:      newStepID,
						AgentID: "user",
						Action:  "user_query",
						Status:  "running",
						Result:  req.Prompt,
					})
					currentPlan.Status = "running"

					// Save plan state (create or update timestamp/status)
					if err := plannerService.Store.SavePlan(currentPlan); err != nil {
						http.Error(w, fmt.Sprintf("Failed to save plan: %v", err), http.StatusInternalServerError)
						return
					}

					// Return plan ID immediately to UI
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"intent": model.Intent{Action: "analyzing"},
						"plan":   currentPlan,
					})

					// Process asynchronously
					// Process asynchronously
					go func() {
						tm.OutputChan <- fmt.Sprintf("[%s] Analyzing request...", planID)

						effectivePrompt := req.Prompt
						if !isNewPlan {
							tm.OutputChan <- fmt.Sprintf("[DEBUG] Loading plan %s. Steps found: %d", planID, len(currentPlan.Steps))
							// 1. Build Context
							history := ""
							// Exclude the last step (current request) which was already appended
							priorSteps := currentPlan.Steps[:len(currentPlan.Steps)-1]

							startIdx := 0
							if len(priorSteps) > 20 {
								startIdx = len(priorSteps) - 20
							}
							for _, s := range priorSteps[startIdx:] {
								switch s.Action {
								case "user_query":
									history += fmt.Sprintf("User: %s\n", s.Result)
								case "general_chat":
									history += fmt.Sprintf("AI: %s\n", s.Result)
								}
							}
							effectivePrompt = fmt.Sprintf("History:\n%s\nRequest: %s", history, req.Prompt)
							tm.OutputChan <- fmt.Sprintf("[DEBUG] Constructed History: %s", history)
						}

						// 2. Analyze Intent
						intent, rawRouterResp, err := routerService.Analyze(context.Background(), effectivePrompt)
						if err != nil {
							tm.OutputChan <- fmt.Sprintf("[%s] Router failed: %v", planID, err)
							// Update pending plan to failed
							currentPlan.Status = "stopped"
							_ = plannerService.Store.SavePlan(currentPlan)
							return
						}

						tm.OutputChan <- fmt.Sprintf("[%s] Intent: %s", planID, intent.Action)

						// Update currentPlan with the analyzed intent
						currentPlan.Intent.Action = intent.Action
						currentPlan.Intent.Category = intent.Category
						currentPlan.Intent.Language = intent.Language
						currentPlan.Intent.ContentType = intent.ContentType

						// Mark user_query as completed (since plan is now generated)
						// This ensures it is preserved by task_manager's cleanup logic,
						// fixing the issue where "pending" steps were being deleted.
						for i, s := range currentPlan.Steps {
							if s.Action == "user_query" && s.Status == "running" {
								currentPlan.Steps[i].Status = "completed"
								///currentPlan.Status = "running" //We are now running
								_ = plannerService.Store.SavePlan(currentPlan)
							}
						}

						// 3. Execution
						if intent.Action == "create_project" {
							// If we were chatting, we might want to start fresh or pivot.
							// For simplicity, we treat this as a "New Plan" triggering event.
							// Stop the active task to prevent it from marking the plan as "Completed"
							// while we are generating the new plan.
							tm.StopTask(planID)
							// Wait longer to ensure `runTaskLoop` has fully exited and flushed its "stopped" state to disk
							time.Sleep(500 * time.Millisecond)

							// Reload plan from store to ensure we have the very latest state (including any "stopped" status wrote by the dying task)
							// This prevents us from saving a stale version that might be overwritten or merging blindly.
							if freshPlan, err := plannerService.Store.GetPlan(planID); err == nil {
								// We must preserve the INTENT from our local `currentPlan` / `req` because
								// the fresh plan from disk might be stale regarding the *new* intent we just analyzed.
								// `currentPlan` has the `intent` from router. `freshPlan` has the old intent.
								// Only `Steps` and `Status` are relevant from disk.
								// Actually, `currentPlan` in this scope has the NEW intent from Router (Line 305).
								// We want to keep that. We only want to ensure `Steps` are up to date (e.g. if Step 1 completed).
								currentPlan.Steps = freshPlan.Steps
								// But we ignore freshPlan.Status because we are about to force it to Running.
							}

							// User wants "content of current chat retained".
							// So we pass the effectivePrompt (with history) to the Planner!
							intent.InitialPrompt = effectivePrompt
							intent.Prompt = effectivePrompt // Planner uses this

							// Indicate we are starting the work
							currentPlan.Status = "running"
							// Add a temporary step to show in Kanban even without UI hacks
							newStepID := 1
							if len(currentPlan.Steps) > 0 {
								newStepID = currentPlan.Steps[len(currentPlan.Steps)-1].ID + 1
							}
							genPlanStep := model.Step{
								ID:      newStepID,
								AgentID: "planner",
								Action:  "generate_plan",
								Status:  "running",
								Result:  "Designing execution plan...",
							}
							currentPlan.Steps = append(currentPlan.Steps, genPlanStep)
							_ = plannerService.Store.SavePlan(currentPlan)
							tm.OutputChan <- fmt.Sprintf("[%s] Generating Plan...", planID)

							fullPlan, err := plannerService.CreatePlan(context.Background(), intent, planID)
							if err != nil {
								tm.OutputChan <- fmt.Sprintf("[%s] Planner failed: %v", planID, err)
								currentPlan.Status = "stopped"
								// Mark generate_plan as failed
								currentPlan.Steps[len(currentPlan.Steps)-1].Status = "failed"
								currentPlan.Steps[len(currentPlan.Steps)-1].Result = fmt.Sprintf("Failed: %v", err)
								_ = plannerService.Store.SavePlan(currentPlan)
								return
							}

							// Mark generate_plan as completed
							currentPlan.Steps[len(currentPlan.Steps)-1].Status = "completed"
							currentPlan.Steps[len(currentPlan.Steps)-1].Result = "Plan generated."

							// MERGE new steps into current plan
							nextID := currentPlan.Steps[len(currentPlan.Steps)-1].ID + 1
							for i := range fullPlan.Steps {
								fullPlan.Steps[i].ID = nextID + i
								// Fix dependencies if they refer to local IDs (1, 2...)
								// This is tricky. Usually dependencies are relative to the new block.
								// We should shift them by (nextID - 1).
								var newDeps []int
								for _, dep := range fullPlan.Steps[i].DependsOn {
									newDeps = append(newDeps, dep+(nextID-1))
								}
								fullPlan.Steps[i].DependsOn = newDeps
							}
							currentPlan.Steps = append(currentPlan.Steps, fullPlan.Steps...)
							currentPlan.SelectedAgents = fullPlan.SelectedAgents
							currentPlan.Status = "running"

							tm.OutputChan <- fmt.Sprintf("[%s] Plan created. Starting task...", planID)

							// Log router step
							_ = plannerService.Store.LogInteraction(currentPlan.ID, "Router", req.Prompt, rawRouterResp)

							// Save updated plan
							_ = plannerService.Store.SavePlan(currentPlan)

							// START THE TASK
							tm.StartTask(context.Background(), currentPlan)
						} else {
							tm.OutputChan <- fmt.Sprintf("[%s] Request handled by Router (no plan needed).", planID)

							// Log to plan-specific log so UI sees it
							_ = plannerService.Store.LogInteraction(planID, "Router", effectivePrompt, rawRouterResp)

							// Determine result to show
							resultText := intent.Answer
							if resultText == "" {
								resultText = rawRouterResp
							}

							// Output the response to the console
							tm.OutputChan <- fmt.Sprintf("[%s] Response: %s", planID, resultText)

							// Update plan status to completed and store the answer as a step
							currentPlan.Status = "completed"

							newID := 1
							if len(currentPlan.Steps) > 0 {
								newID = currentPlan.Steps[len(currentPlan.Steps)-1].ID + 1
							}

							currentPlan.Steps = append(currentPlan.Steps, model.Step{
								ID:      newID,
								AgentID: "router",
								Action:  "general_chat",
								Status:  "completed",
								Result:  resultText,
							})
							_ = plannerService.Store.SavePlan(currentPlan)
						}
					}()
				})

				// Agent Endpoint
				r.Get("/agents", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					agents := reg.ListAgents()
					json.NewEncoder(w).Encode(agents)
				})

				// Skill Endpoint
				r.Get("/skills", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					skills := reg.ListSkills()
					json.NewEncoder(w).Encode(skills)
				})

				// Configuration Endpoint
				r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(cfgMgr.Get().Sanitize())
				})

				// Build Trigger Endpoint
				r.Post("/build", func(w http.ResponseWriter, r *http.Request) {
					var req struct {
						RepoURL    string `json:"repo_url"`
						CommitHash string `json:"commit_hash"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						http.Error(w, "Invalid request body", http.StatusBadRequest)
						return
					}

					id, err := buildEngine.TriggerBuild(r.Context(), req.RepoURL, req.CommitHash)
					if err != nil {
						http.Error(w, fmt.Sprintf("Build failed: %v", err), http.StatusInternalServerError)
						return
					}
					json.NewEncoder(w).Encode(map[string]string{"build_id": id})
				})

				// HITL Endpoint (Stub)
				r.Get("/approvals", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]string{})
				})

				// --- Task & Plan Monitoring ---
				r.Post("/interaction", func(w http.ResponseWriter, r *http.Request) {
					var req struct {
						PlanID  string `json:"plan_id"`
						AgentID string `json:"agent_id"`
						Action  string `json:"action"`
						Result  string `json:"result"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						http.Error(w, "Invalid request", http.StatusBadRequest)
						return
					}
					_ = plannerService.Store.LogInteraction(req.PlanID, req.AgentID, req.Action, req.Result)
					w.WriteHeader(http.StatusOK)
				})

				r.Get("/plans", func(w http.ResponseWriter, r *http.Request) {
					plans, err := plannerService.Store.ListPlans()
					if err != nil {
						http.Error(w, "Failed to list plans", http.StatusInternalServerError)
						return
					}

					// Update plan statuses based on actual task states
					tm.mu.Lock()
					for i := range plans {
						if task, ok := tm.tasks[plans[i].ID]; ok {
							switch task.Status {
							case TaskStatusRunning:
								plans[i].Status = "running"
							case TaskStatusWaitingInput:
								plans[i].Status = "waiting_input"
							case TaskStatusCompleted:
								plans[i].Status = "completed"
							case TaskStatusError:
								plans[i].Status = "stopped"
							}
						} else {
							// No active task - mark as stopped if it claims to be running
							// Commented out to prevent UI flickering "Stopped" during short transitions or if task is running in background but not in memory list yet?
							// Actually, if task is NOT in memory, it IS stopped effectively.
							// BUT during `CreatePlan`, we StopTask provided we manually set status="running".
							// So if we trust Disk, we should NOT override here.
							// if plans[i].Status == "running" || plans[i].Status == "waiting_input" {
							// 	plans[i].Status = "stopped"
							// }
						}
					}
					tm.mu.Unlock()

					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(plans)
				})

				r.Get("/plans/{id}", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if !strings.HasPrefix(id, "plan-") {
						id = "plan-" + id
					}
					plan, err := plannerService.Store.GetPlan(id)
					if err != nil {
						http.Error(w, "Plan not found", http.StatusNotFound)
						return
					}

					// Update plan status based on actual task state
					tm.mu.Lock()
					if task, ok := tm.tasks[id]; ok {
						switch task.Status {
						case TaskStatusRunning:
							plan.Status = "running"
						case TaskStatusWaitingInput:
							plan.Status = "waiting_input"
						case TaskStatusCompleted:
							plan.Status = "completed"
						case TaskStatusError:
							plan.Status = "stopped"
						}
					} else {
						// No active task - check if plan says running but task is gone
						// We prefer to return the persistent status (even if zombie) to avoid race conditions during startup
						// if plan.Status == "running" || plan.Status == "waiting_input" {
						// 	  plan.Status = "stopped"
						// }
					}
					tm.mu.Unlock()

					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(plan)
				})

				r.Delete("/plans/{id}", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if !strings.HasPrefix(id, "plan-") {
						id = "plan-" + id
					}

					// Stop the task if running
					tm.mu.Lock()
					if task, ok := tm.tasks[id]; ok {
						task.Cancel()
						delete(tm.tasks, id)
					}
					tm.mu.Unlock()

					// Wait for task to cleanup and logs to drain preventing race condition where log file is recreated
					time.Sleep(500 * time.Millisecond)

					// Delete plan file
					if err := plannerService.Store.DeletePlan(id); err != nil {
						http.Error(w, "Failed to delete plan", http.StatusInternalServerError)
						return
					}

					w.WriteHeader(http.StatusOK)
				})

				r.Post("/plans/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if !strings.HasPrefix(id, "plan-") {
						id = "plan-" + id
					}

					plan, err := plannerService.Store.GetPlan(id)
					if err != nil {
						http.Error(w, "Plan not found", http.StatusNotFound)
						return
					}

					// Update status to running and restart task
					plan.Status = "running"
					if err := plannerService.Store.SavePlan(plan); err != nil {
						http.Error(w, "Failed to update plan", http.StatusInternalServerError)
						return
					}

					// Restart the task
					tm.StartTask(context.Background(), plan)

					w.WriteHeader(http.StatusOK)
				})

				r.Post("/plans/{id}/stop", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if !strings.HasPrefix(id, "plan-") {
						id = "plan-" + id
					}

					// Stop the task via TaskManager
					tm.StopTask(id)

					// Force update plan status to stopped
					plan, err := plannerService.Store.GetPlan(id)
					if err == nil {
						plan.Status = "stopped"
						_ = plannerService.Store.SavePlan(plan)
					}

					w.WriteHeader(http.StatusOK)
				})

				r.Post("/plans/{id}/files", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if !strings.HasPrefix(id, "plan-") {
						id = "plan-" + id
					}

					// Update max upload size to 50MB
					r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
					if err := r.ParseMultipartForm(50 << 20); err != nil {
						http.Error(w, "File too big", http.StatusBadRequest)
						return
					}

					file, header, err := r.FormFile("file")
					if err != nil {
						http.Error(w, "Invalid file", http.StatusBadRequest)
						return
					}
					defer file.Close()

					rootDir, _ := findProjectRoot()
					if rootDir == "" {
						http.Error(w, "Project root not found", http.StatusInternalServerError)
						return
					}

					targetDir := filepath.Join(rootDir, ".druppie", "files", id)
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						http.Error(w, "Failed to create directory", http.StatusInternalServerError)
						return
					}

					filename := header.Filename
					dstPath := filepath.Join(targetDir, filename)
					dst, err := os.Create(dstPath)
					if err != nil {
						http.Error(w, "Failed to create file", http.StatusInternalServerError)
						return
					}
					defer dst.Close()

					if _, err := io.Copy(dst, file); err != nil {
						http.Error(w, "Failed to save file", http.StatusInternalServerError)
						return
					}

					// Update Plan in Store
					tm.mu.Lock()
					plan, err := plannerService.Store.GetPlan(id)
					if err == nil {
						exists := false
						for _, f := range plan.Files {
							if f == filename {
								exists = true
								break
							}
						}
						if !exists {
							plan.Files = append(plan.Files, filename)
							_ = plannerService.Store.SavePlan(plan)
						}
					}
					tm.mu.Unlock()

					_ = plannerService.Store.LogInteraction(id, "System", "File Upload", fmt.Sprintf("Uploaded file: %s", filename))

					w.WriteHeader(http.StatusOK)
					w.Write([]byte(fmt.Sprintf(`{"filename": "%s"}`, filename)))
				})

				r.Get("/logs/{id}", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if !strings.HasPrefix(id, "plan-") {
						id = "plan-" + id
					}
					logs, err := plannerService.Store.GetLogs(id)
					if err != nil {
						http.Error(w, "Logs not found", http.StatusNotFound)
						return
					}
					w.Header().Set("Content-Type", "text/plain")
					w.Write([]byte(logs))
				})

				// Alias for UI which requests /v1/tasks/{id}/output
				r.Get("/tasks/{id}/output", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					if !strings.HasPrefix(id, "plan-") {
						id = "plan-" + id
					}
					logs, err := plannerService.Store.GetLogs(id)
					if err != nil {
						// Return empty log instead of 404 to avoid console errors if just started
						w.Header().Set("Content-Type", "text/plain")
						w.Write([]byte(""))
						return
					}
					w.Header().Set("Content-Type", "text/plain")
					w.Write([]byte(logs))
				})

				r.Post("/tasks/{id}/message", func(w http.ResponseWriter, r *http.Request) {
					id := chi.URLParam(r, "id")
					var req struct {
						Input string `json:"input"`
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						http.Error(w, "Invalid request", http.StatusBadRequest)
						return
					}

					// Find the task
					// Note: TaskManager stores tasks by plan ID
					// We need to find the plan ID that matches. Actually TaskManager.tasks uses plan.ID.
					// Let's assume the ID passed is the plan ID.
					tm.mu.Lock()
					task, ok := tm.tasks[id]
					tm.mu.Unlock()

					if !ok {
						// Task not active in memory. Try to resume from store if plan exists.
						plan, err := plannerService.Store.GetPlan(id)
						if err == nil {
							// Plan exists! Resume it.
							tm.OutputChan <- fmt.Sprintf("[%s] Resuming inactive plan from store on input received...", id)

							// Force status update to valid running state before starting
							plan.Status = "running"
							_ = plannerService.Store.SavePlan(plan)

							// Start the task
							task = tm.StartTask(context.Background(), plan)

							// Proceed to use 'task' for input
						} else {
							// Task not in memory AND Plan not found in store

							// Task not started yet - handle stop command directly
							if req.Input == "/stop" {
								plan, err := plannerService.Store.GetPlan(id)
								if err == nil {
									plan.Status = "stopped"
									_ = plannerService.Store.SavePlan(plan)
									w.WriteHeader(http.StatusOK)
									return
								}
							}
							http.Error(w, "Task not found", http.StatusNotFound)
							return
						}
					}

					// Send to task (Buffered channel allows immediate return)
					select {
					case task.InputChan <- req.Input:
						w.WriteHeader(http.StatusOK)
					default:
						http.Error(w, "Task input buffer full", http.StatusServiceUnavailable)
					}
				})
			})

			// Serve static files (expecting 'ui' folder to be present in root)
			workDir, _ := os.Getwd()
			var staticRoot string

			// Check if ./ui exists (Docker / Production)
			if _, err := os.Stat(filepath.Join(workDir, "ui")); err == nil {
				staticRoot = workDir
			} else if _, err := os.Stat(filepath.Join(filepath.Dir(workDir), "ui")); err == nil {
				// Check if ../ui exists (Local Development from core/)
				// In this case, we serve the PARENT directory so that /ui/... URLs work correctly
				staticRoot = filepath.Dir(workDir)
			} else {
				fmt.Println("[Warning] Could not find 'ui' directory in ./ui or ../ui. Serving current directory.")
				staticRoot = workDir
			}

			fmt.Printf("Serving static files from root: %s\n", staticRoot)
			fs := http.FileServer(http.Dir(staticRoot))
			r.Handle("/*", fs)

			port := cfg.Server.Port
			if port == "" {
				port = "8080"
			}
			fmt.Printf("Starting server on port %s...\n", port)
			if err := http.ListenAndServe(":"+port, r); err != nil {
				fmt.Printf("Server failed: %v\n", err)
				os.Exit(1)
			}
		},
	}

	var registryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Dump the loaded registry",
		Run: func(cmd *cobra.Command, args []string) {
			_, reg, _, _, _, err := setup(cmd)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			stats := reg.Stats()
			fmt.Printf("Loaded: %d Building Blocks, %d Skills, %d MCP Servers, %d Agents\n",
				stats["building_blocks"], stats["skills"], stats["mcp_servers"], stats["agents"])

			for _, bb := range reg.ListBuildingBlocks() {
				fmt.Printf("- [Block] %s: %s\n", bb.ID, bb.Name)
			}

			// We access maps directly here for simplicity, or we could add ListSkills/ListMCPServers methods.
			// Since we are in a read-only CLI command initialized sequentially, direct access is okay,
			// but proper practice would be to use helper methods.
			// Let's rely on the fact that we can interact with the struct fields.

			for id, skill := range reg.Skills {
				fmt.Printf("- [Skill] %s: %s\n", id, skill.Name)
			}

			for id, mcp := range reg.MCPServers {
				fmt.Printf("- [MCP] %s: %s\n", id, mcp.Name)
			}

			// ListAgents was added in Phase 2
			for _, agent := range reg.ListAgents() {
				fmt.Printf("- [Agent] %s: %s\n", agent.ID, agent.Name)
			}
		},
	}

	// runInteractiveLoop Removed

	// TaskManager instance
	var tm *TaskManager

	var chatCmd = &cobra.Command{
		Use:   "chat",
		Short: "Start interactive chat",
		Run: func(cmd *cobra.Command, args []string) {
			cfgMgr, _, router, planner, _, err := setup(cmd)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			cfg := cfgMgr.Get()

			// Initialize TaskManager
			tm = NewTaskManager(planner)

			fmt.Println("--- Druppie Core Chat (Async) ---")
			fmt.Printf("LLM Provider: %s\n", cfg.LLM.DefaultProvider)
			fmt.Println("Commands: /exit, /list, /stop <id>, /switch <id>")

			// Resume plan if ID provided
			if planID != "" {
				fullID := planID
				if !strings.HasPrefix(fullID, "plan-") {
					fullID = "plan-" + fullID
				}
				router.PlanID = fullID
				fmt.Printf("[Chat] Resuming plan: %s\n", fullID)
				plan, err := planner.Store.GetPlan(fullID)
				if err != nil {
					fmt.Printf("[Error] Failed to load plan: %v\n", err)
				} else {
					tm.StartTask(context.Background(), plan)
				}
			}

			// Main Loop
			scanner := bufio.NewScanner(os.Stdin)
			fmt.Print("> ")

			// Input channel to decouple blocking read
			inputChan := make(chan string)
			go func() {
				for scanner.Scan() {
					inputChan <- scanner.Text()
				}
				close(inputChan)
			}()

			var activeTaskID string = ""

			for {
				select {
				case input, ok := <-inputChan:
					if !ok {
						return
					}

					// Handle Global Commands
					if input == "/exit" {
						return
					}
					if input == "/list" {
						tasks := tm.ListTasks()
						fmt.Println("Active Tasks:")
						for _, t := range tasks {
							fmt.Println(" - " + t)
						}
						fmt.Print("> ")
						continue
					}
					if strings.HasPrefix(input, "/stop ") {
						id := strings.TrimSpace(strings.TrimPrefix(input, "/stop "))
						tm.StopTask(id)
						fmt.Print("> ")
						continue
					}
					if strings.HasPrefix(input, "/switch ") {
						id := strings.TrimSpace(strings.TrimPrefix(input, "/switch "))
						activeTaskID = id
						fmt.Printf("Switched active task context to %s\n> ", id)
						continue
					}

					// Route Input
					// If activeTaskID is set and that task is waiting, send it there
					// Else if no active task, try to find one waiting

					targetTask := tm.GetSingleActiveTask()
					if activeTaskID != "" {
						if t, ok := tm.tasks[activeTaskID]; ok {
							targetTask = t
						}
					}

					if targetTask != nil && targetTask.Status == TaskStatusWaitingInput {
						// Send to Task
						targetTask.InputChan <- input
						// Reset prompt
						continue
					}

					// If not handled by task, treat as new Router Request
					if !strings.HasPrefix(input, "/") {
						fmt.Println("[Router - Analyzing]")
						intent, rawRouterResp, err := router.Analyze(context.Background(), input)
						if err != nil {
							fmt.Printf("[Error] Router failed: %v\n> ", err)
							continue
						}

						if intent.Action == "create_project" {
							plan, err := planner.CreatePlan(context.Background(), intent, "")
							if err != nil {
								fmt.Printf("[Error] Planner failed: %v\n> ", err)
								continue
							}
							_ = planner.Store.LogInteraction(plan.ID, "Router", input, rawRouterResp)

							task := tm.StartTask(context.Background(), plan)
							activeTaskID = task.ID
							fmt.Printf("[Planner] Started Plan %s\n", plan.ID)
						} else {
							fmt.Printf("[Router - %s] %s\n> ", intent.Action, intent.InitialPrompt)
						}
					} else {
						fmt.Println("Unknown command.")
						fmt.Print("> ")
					}

				case log := <-tm.OutputChan:
					// Clear current line if possible or just print
					// Simple print for now
					fmt.Printf("\r%s\n> ", log)

				case id := <-tm.TaskDoneChan:
					fmt.Printf("\r[Task Manager] Task %s Finished.\n> ", id)
				}
			}
		},
	}

	var planCmd = &cobra.Command{
		Use:   "plan [prompt]",
		Short: "Generate a plan for a given prompt",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			_, _, router, planner, _, err := setup(cmd)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			prompt := strings.Join(args, " ")

			// Load or Create Plan
			var currentPlan model.ExecutionPlan
			effectivePrompt := prompt
			isNewPlan := true

			if planID != "" {
				if p, err := planner.Store.GetPlan(planID); err == nil {
					currentPlan = p
					isNewPlan = false
					fmt.Printf("[Chat] Resuming plan: %s\n", planID)
				}
			}

			if isNewPlan {
				if planID == "" {
					// Generate a new ID if not provided
					planID = fmt.Sprintf("plan-%d", time.Now().Unix())
				}
				currentPlan = model.ExecutionPlan{
					ID: planID,
					Intent: model.Intent{
						InitialPrompt: prompt,
						Prompt:        prompt,
					},
					Status: "pending",
					Steps:  []model.Step{},
				}
			} else {
				// 0. Store User Input
				newStepID := 1
				if len(currentPlan.Steps) > 0 {
					newStepID = currentPlan.Steps[len(currentPlan.Steps)-1].ID + 1
				}
				currentPlan.Steps = append(currentPlan.Steps, model.Step{
					ID: newStepID, AgentID: "user", Action: "user_query", Status: "running", Result: prompt,
				})
				currentPlan.Status = "running"
				_ = planner.Store.SavePlan(currentPlan)

				// 1. Build Context
				history := ""
				startIdx := 0
				if len(currentPlan.Steps) > 20 {
					startIdx = len(currentPlan.Steps) - 20
				}
				for _, s := range currentPlan.Steps[startIdx:] {
					switch s.Action {
					case "user_query":
						history += fmt.Sprintf("User: %s\n", s.Result)
					case "general_chat":
						history += fmt.Sprintf("AI: %s\n", s.Result)
					}
				}
				effectivePrompt = fmt.Sprintf("History:\n%s\nRequest: %s", history, prompt)
				//fmt.Printf("[DEBUG] Constructed History Steps: %d\n", len(currentPlan.Steps))
			}

			// Initialize TaskManager early to unify output
			tm := NewTaskManager(planner)

			intent, rawRouterResp, err := router.Analyze(context.Background(), effectivePrompt)
			if err != nil {
				fmt.Printf("Router failed: %v\n", err)
				os.Exit(1)
			}

			var responseOutput string

			if intent.Action != "create_project" {
				_ = planner.Store.LogInteraction(currentPlan.ID, "Router", effectivePrompt, rawRouterResp)

				answer := intent.Answer
				if answer == "" {
					answer = rawRouterResp
				}

				// Append AI Step
				newStepID := 1
				if len(currentPlan.Steps) > 0 {
					newStepID = currentPlan.Steps[len(currentPlan.Steps)-1].ID + 1
				}
				currentPlan.Steps = append(currentPlan.Steps, model.Step{
					ID: newStepID, AgentID: "router", Action: "general_chat", Status: "completed", Result: answer,
				})
				currentPlan.Status = "completed"
				_ = planner.Store.SavePlan(currentPlan)

				if intent.Answer != "" {
					responseOutput = fmt.Sprintf("[%s]\n ** AI Response:**\n%s", currentPlan.ID, intent.Answer)
				} else {
					responseOutput = fmt.Sprintf("[%s] Intent was '%s', which doesn't trigger a planner in this CLI.\n", currentPlan.ID, intent.Action)
				}
			} else {
				// Update intent for planner
				intent.InitialPrompt = effectivePrompt
				intent.Prompt = effectivePrompt

				var err error
				currentPlan, err = planner.CreatePlan(context.Background(), intent, currentPlan.ID)
				if err != nil {
					fmt.Printf("[%s] Planner failed: %v\n", currentPlan.ID, err)
					os.Exit(1)
				}
				// Log router step to plan log
				_ = planner.Store.LogInteraction(currentPlan.ID, "Router", prompt, rawRouterResp)
				// Explicitly save the plan to disk so TaskManager can find it
				if err := planner.Store.SavePlan(currentPlan); err != nil {
					fmt.Printf("[%s] Failed to save plan: %v\n", currentPlan.ID, err)
					os.Exit(1)
				}
			}

			// Initialize TaskManager for execution
			task := tm.StartTask(context.Background(), currentPlan)
			fmt.Printf("[%s] Started Plan execution...\n", currentPlan.ID)

			// Inject the postponed AI response if exists
			if responseOutput != "" {
				tm.OutputChan <- responseOutput
			}

			// Execution Loop
			done := false
			for !done {
				select {
				case log := <-tm.OutputChan:
					fmt.Println(log)
				case <-time.After(500 * time.Millisecond):
					// Check status periodically
					// Note: Direct access is slightly racy but acceptable for this CLI tool
					switch task.Status {
					case TaskStatusWaitingInput:
						fmt.Println("[Auto-Pilot] Input required. Auto-accepting defaults...")
						// Simulate user verify/accept delay
						time.Sleep(1 * time.Second)
						task.InputChan <- "/accept"
					case TaskStatusCompleted:
						fmt.Println("[Auto-Pilot] Plan execution completed successfully.")
						done = true
					case TaskStatusError:
						fmt.Println("[Auto-Pilot] Plan execution failed.")
						done = true
					}
				}
			}

			// Fetch final state
			finalPlan, _ := planner.Store.GetPlan(currentPlan.ID)
			currentPlan = finalPlan
			// validJSON, _ := json.MarshalIndent(currentPlan, "", "  ")
			// fmt.Println(string(validJSON))
			fmt.Println("") // Add one more empty line on exit
		},
	}

	rootCmd.PersistentFlags().StringVar(&planID, "plan-id", "", "ID of an existing plan to resume or manage")
	rootCmd.PersistentFlags().StringVar(&llmProviderOverride, "llm-provider", "", "Override default LLM provider")
	rootCmd.PersistentFlags().StringVar(&buildProviderOverride, "build-provider", "", "Override default Build provider")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", true, "Enable debug mode (print raw LLM responses)")

	rootCmd.AddCommand(registryCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(planCmd)
	rootCmd.AddCommand(serveCmd)

	// Default to server mode
	rootCmd.Run = serveCmd.Run

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
