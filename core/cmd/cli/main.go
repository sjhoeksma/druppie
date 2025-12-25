package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drug-nl/druppie/core/internal/builder"
	"github.com/drug-nl/druppie/core/internal/config"
	"github.com/drug-nl/druppie/core/internal/llm"
	"github.com/drug-nl/druppie/core/internal/planner"
	"github.com/drug-nl/druppie/core/internal/registry"
	"github.com/drug-nl/druppie/core/internal/router"
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
	setup := func(cmd *cobra.Command) (*config.Config, *registry.Registry, *router.Router, *planner.Planner, error) {
		cwd, _ := os.Getwd()
		// Try to find the root directory (where bouwblokken resides)
		rootDir := filepath.Join(cwd, "..")
		if _, err := os.Stat(filepath.Join(rootDir, "blocks")); os.IsNotExist(err) {
			rootDir = cwd // fallback if running from root
		}

		fmt.Printf("Loading registry from: %s\n", rootDir)
		reg, err := registry.LoadRegistry(rootDir)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("registry load error: %w", err)
		}

		// Load Configuration
		// Use "config.yaml" which will auto-init from default if missing via our updated config manager
		cfgMgr, err := config.NewManager("config.yaml")
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("config load error: %w", err)
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
		_, err = builder.NewEngine(cfg.Build)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("builder init error: %w", err)
		}

		// Initialize LLM with Config
		llmManager, err := llm.NewManager(context.Background(), cfg.LLM)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("llm init error: %w", err)
		}

		r := router.NewRouter(llmManager, debug)
		p := planner.NewPlanner(llmManager, reg, debug)

		return &cfg, reg, r, p, nil
	}

	var registryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Dump the loaded registry",
		Run: func(cmd *cobra.Command, args []string) {
			_, reg, _, _, err := setup(cmd)
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

	var chatCmd = &cobra.Command{
		Use:   "chat",
		Short: "Start interactive chat",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, _, router, planner, err := setup(cmd)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}

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

					planJSON, _ := json.MarshalIndent(plan, "", "  ")
					fmt.Printf("[Planner] Plan Created:\n%s\n", string(planJSON))
				}
			}
		},
	}

	var planCmd = &cobra.Command{
		Use:   "plan [prompt]",
		Short: "Generate a plan for a given prompt",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			_, _, router, planner, err := setup(cmd)
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
