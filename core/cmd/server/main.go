package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/drug-nl/druppie/core/internal/builder"
	"github.com/drug-nl/druppie/core/internal/config"
	"github.com/drug-nl/druppie/core/internal/llm"
	"github.com/drug-nl/druppie/core/internal/model"
	"github.com/drug-nl/druppie/core/internal/planner"
	"github.com/drug-nl/druppie/core/internal/registry"
	"github.com/drug-nl/druppie/core/internal/router"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Setup Dependencies
	cwd, _ := os.Getwd()
	rootDir := filepath.Join(cwd, "..") // Fallback / assumption
	if _, err := os.Stat(filepath.Join(rootDir, "blocks")); os.IsNotExist(err) {
		rootDir = cwd
	}

	log.Printf("Loading Registry from: %s", rootDir)
	reg, err := registry.LoadRegistry(rootDir)
	if err != nil {
		log.Fatalf("Failed to load registry: %v", err)
	}

	// Load Configuration
	cfgMgr, err := config.NewManager("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	cfg := cfgMgr.Get()

	// Initialize Build Engine
	buildEngine, err := builder.NewEngine(cfg.Build)
	if err != nil {
		log.Fatalf("Failed to initialize Build Engine: %v", err)
	}

	// Initialize LLM with Config
	llmManager, err := llm.NewManager(context.Background(), cfg.LLM)
	if err != nil {
		log.Fatalf("Failed to initialize LLM manager: %v", err)
	}
	defer llmManager.Close()
	routerService := router.NewRouter(llmManager)
	plannerService := planner.NewPlanner(llmManager, reg)

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

		// Agent Endpoint (Stub)
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
			// Get current config (Deep Copy)
			currentConfig := cfgMgr.Get()

			// Decode JSON into current config to merge updates
			if err := json.NewDecoder(r.Body).Decode(&currentConfig); err != nil {
				http.Error(w, "Invalid config", http.StatusBadRequest)
				return
			}

			// Save updated config
			if err := cfgMgr.Update(currentConfig); err != nil {
				http.Error(w, fmt.Sprintf("Failed to update config: %v", err), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		// Build Trigger Endpoint (Debug/Manual)
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

	port := cfg.Server.Port
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Starting server on port %s\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
