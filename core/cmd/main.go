package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
		Short: "Druppie Core Helper CLI",
		Long:  `CLI for local testing and interacting with Druppie Core logic (Registry, Planner, Chat)`,
	}

	// CLI Flags
	var llmProviderOverride string
	var buildProviderOverride string
	var debug bool

	// Helper to bootstrap dependencies
	// Returns ConfigManager to allow updates, and Builder Engine
	// Helper to find project root
	findProjectRoot := func() (string, error) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}

		// Traverse up until we find .druppie or blocks
		for {
			if _, err := os.Stat(filepath.Join(cwd, ".druppie")); err == nil {
				return cwd, nil
			}
			if _, err := os.Stat(filepath.Join(cwd, "blocks")); err == nil {
				return cwd, nil
			}

			parent := filepath.Dir(cwd)
			if parent == cwd {
				// Reached root without finding
				return "", fmt.Errorf("project root not found (missing .druppie or blocks)")
			}
			cwd = parent
		}
	}

	// Helper to bootstrap dependencies
	// Returns ConfigManager to allow updates, and Builder Engine
	setup := func(cmd *cobra.Command) (*config.Manager, *registry.Registry, *router.Router, *planner.Planner, builder.BuildEngine, error) {
		rootDir, err := findProjectRoot()
		if err != nil {
			// Fallback to CWD just in case, or bubble error?
			// Let's print warning and use CWD, but likely it will fail later
			fmt.Printf("Warning: %v. Using CWD.\n", err)
			rootDir, _ = os.Getwd()
		}

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

		r := router.NewRouter(llmManager, debug)
		p := planner.NewPlanner(llmManager, reg, druppieStore, debug)

		return cfgMgr, reg, r, p, buildEngine, nil
	}

	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the Druppie Core API Server",
		Run: func(cmd *cobra.Command, args []string) {
			// Check if we are in 'core' and need to move up to project root
			cwd, _ := os.Getwd()
			if filepath.Base(cwd) == "core" {
				fmt.Println("Running in 'core', moving working directory up to project root.")
				os.Chdir("..")
			}

			cfgMgr, reg, routerService, plannerService, buildEngine, err := setup(cmd)
			if err != nil {
				fmt.Printf("Startup Error: %v\n", err)
				os.Exit(1)
			}
			cfg := cfgMgr.Get()

			r := chi.NewRouter()
			r.Use(middleware.Logger)
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
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						http.Error(w, "Invalid request body", http.StatusBadRequest)
						return
					}

					// 1. Analyze Intent
					intent, err := routerService.Analyze(r.Context(), req.Prompt)
					if err != nil {
						http.Error(w, fmt.Sprintf("Router failed: %v", err), http.StatusInternalServerError)
						return
					}

					// 2. Planning (if needed)
					var plan *model.ExecutionPlan
					if intent.Action == "create_project" {
						p, err := plannerService.CreatePlan(r.Context(), intent)
						if err != nil {
							http.Error(w, fmt.Sprintf("Planner failed: %v", err), http.StatusInternalServerError)
							return
						}
						plan = &p
					}

					// 3. Response
					resp := map[string]interface{}{
						"intent": intent,
						"plan":   plan,
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(resp)
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

				// MCP Endpoint
				r.Get("/mcps", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					mcpServers := reg.ListMCPServers()
					json.NewEncoder(w).Encode(mcpServers)
				})

				// Configuration Endpoint
				r.Get("/config", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(cfgMgr.Get().Sanitize())
				})

				r.Put("/config", func(w http.ResponseWriter, r *http.Request) {
					currentConfig := cfgMgr.Get()
					if err := json.NewDecoder(r.Body).Decode(&currentConfig); err != nil {
						http.Error(w, "Invalid config", http.StatusBadRequest)
						return
					}
					if err := cfgMgr.Update(currentConfig); err != nil {
						http.Error(w, fmt.Sprintf("Failed to update config: %v", err), http.StatusInternalServerError)
						return
					}
					w.WriteHeader(http.StatusOK)
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
			})

			// Serve static files from current directory
			workDir, _ := os.Getwd()
			fmt.Printf("Serving static files from: %s\n", workDir)
			fs := http.FileServer(http.Dir(workDir))
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

	runInteractiveLoop := func(ctx context.Context, p *planner.Planner, plan *model.ExecutionPlan) {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			// Check if we need to ask questions
			if len(plan.Steps) > 0 {
				lastStep := plan.Steps[len(plan.Steps)-1]
				if lastStep.Action == "ask_questions" {
					fmt.Printf("\n[Planner Needs Info]: %s\n", lastStep.Description)
					// Display questions if params exist
					// Display questions if params exist
					var assumptions []interface{}
					if as, ok := lastStep.Params["assumptions"]; ok {
						if listAs, isListAs := as.([]interface{}); isListAs {
							assumptions = listAs
						}
					}

					var questions []interface{}
					if qs, ok := lastStep.Params["questions"]; ok {
						if list, isList := qs.([]interface{}); isList {
							questions = list
						} else {
							questions = []interface{}{qs}
						}
					}

					if len(questions) > 0 {
						fmt.Println("")
						for i, q := range questions {
							assumption := "Unknown"
							if i < len(assumptions) {
								assumption = fmt.Sprintf("%v", assumptions[i])
							}
							fmt.Printf("  %d. %v (Default: %s)\n", i+1, q, assumption)
						}
					}

					fmt.Println("\nOptions: [Type your answer] | 'accept' (use defaults) | 'exit' (quit)")
					fmt.Print("> ")
					if !scanner.Scan() {
						return
					}
					answer := scanner.Text()

					if answer == "exit" || answer == "quit" {
						return
					}
					if answer == "accept" || answer == "skip" {
						// Build detailed answer from assumptions
						var details strings.Builder
						for i, q := range questions {
							val := "Unknown"
							if i < len(assumptions) {
								val = fmt.Sprintf("%v", assumptions[i])
							}
							details.WriteString(fmt.Sprintf("%v - %v\n", q, val))
						}
						answer = details.String()
					}

					fmt.Println("[Planner] Updating plan...")
					updatedPlan, err := p.UpdatePlan(ctx, plan, answer)
					if err != nil {
						fmt.Printf("[Error] Update failed: %v\n", err)
						return
					}
					plan = updatedPlan
					continue // Loop to check next state
				}
			}

			// Show current state
			planJSON, _ := json.MarshalIndent(plan, "", "  ")
			fmt.Printf("\n[Current Plan ID: %s]\n%s\n", plan.ID, string(planJSON))

			fmt.Println("\nPlan is autosaved. Options: [type feedback to refine] | 'exit' (quit)")
			fmt.Print("> ")
			if !scanner.Scan() {
				return
			}
			input := scanner.Text()

			if input == "exit" || input == "quit" {
				return
			}
			if input == "ok" {
				// Treat ok as "I'm done, thanks"
				return
			}

			// Refine
			updatedPlan, err := p.UpdatePlan(ctx, plan, input)
			if err != nil {
				fmt.Printf("[Error] Refine failed: %v\n", err)
				continue
			}
			plan = updatedPlan
		}
	}

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

			fmt.Println("--- Druppie Core Chat (Interactive) ---")
			fmt.Printf("LLM Provider: %s\n", cfg.LLM.DefaultProvider)
			// Helper to get active model
			activeModel := "unknown"
			if cfg.LLM.Providers != nil {
				if p, ok := cfg.LLM.Providers[cfg.LLM.DefaultProvider]; ok {
					activeModel = p.Model
				}
			}
			fmt.Printf("LLM Model: %s\n", activeModel)
			fmt.Printf("Build Provider: %s\n", cfg.Build.DefaultProvider)
			fmt.Println("Type 'exit' to quit.")
			scanner := bufio.NewScanner(os.Stdin)

			for {
				fmt.Print("> ")
				if !scanner.Scan() {
					break
				}
				input := scanner.Text()
				if input == "exit" {
					break
				}

				// 1. Route
				fmt.Println("[Router] Analyzing intent...")
				intent, err := router.Analyze(context.Background(), input)
				if err != nil {
					fmt.Printf("[Error] Router failed: %v\n", err)
					continue
				}
				fmt.Printf("[Router] Detected Intent: %+v\n", intent)

				// 2. Plan (if action is create_project)
				if intent.Action == "create_project" {
					fmt.Println("[Planner] Creating execution plan...")
					plan, err := planner.CreatePlan(context.Background(), intent)
					if err != nil {
						fmt.Printf("[Error] Planner failed: %v\n", err)
						continue
					}
					fmt.Printf("[Planner] Plan Created (ID: %s)\n", plan.ID)

					// Enter interactive loop
					runInteractiveLoop(context.Background(), planner, &plan)
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
			intent, err := router.Analyze(context.Background(), prompt)
			if err != nil {
				fmt.Printf("Router failed: %v\n", err)
				os.Exit(1)
			}

			if intent.Action != "create_project" {
				fmt.Printf("Intent was '%s', which doesn't trigger a planner in this CLI.\n", intent.Action)
				return
			}

			plan, err := planner.CreatePlan(context.Background(), intent)
			if err != nil {
				fmt.Printf("Planner failed: %v\n", err)
				os.Exit(1)
			}

			// Auto-resolve questions loop (Max 5 rounds)
			for i := 0; i < 5; i++ {
				if len(plan.Steps) == 0 {
					break
				}
				lastStep := plan.Steps[len(plan.Steps)-1]
				if lastStep.Action != "ask_questions" {
					break
				}

				// Extract Questions and Assumptions
				var assumptions []interface{}
				if as, ok := lastStep.Params["assumptions"]; ok {
					if listAs, isListAs := as.([]interface{}); isListAs {
						assumptions = listAs
					}
				}
				var questions []interface{}
				if qs, ok := lastStep.Params["questions"]; ok {
					if list, isList := qs.([]interface{}); isList {
						questions = list
					} else {
						questions = []interface{}{qs}
					}
				}

				// Build Answer
				var details strings.Builder
				for i, q := range questions {
					val := "Unknown"
					if i < len(assumptions) {
						val = fmt.Sprintf("%v", assumptions[i])
					}
					details.WriteString(fmt.Sprintf("%v - %v\n", q, val))
				}
				answer := details.String()
				if answer == "" {
					answer = "User accepted defaults."
				}

				fmt.Fprintf(os.Stderr, "[Auto-Plan] Accepting defaults for step %d...\n", lastStep.ID)

				// Update Plan
				updatedPlan, err := planner.UpdatePlan(context.Background(), &plan, answer)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[Error] Auto-resolving failed: %v\n", err)
					break
				}
				plan = *updatedPlan
			}

			validJSON, _ := json.MarshalIndent(plan, "", "  ")
			fmt.Println(string(validJSON))
		},
	}

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
