package config

import (
	"fmt"
	"os"
	"sync" // For thread-safe updates

	"gopkg.in/yaml.v3"
)

// Config holds the runtime configuration
type Config struct {
	LLM    LLMConfig    `yaml:"llm" json:"llm"`
	Server ServerConfig `yaml:"server" json:"server"`
	Build  BuildConfig  `yaml:"build" json:"build"`
	Git    GitConfig    `yaml:"git" json:"git"`
}

type GitConfig struct {
	Provider string `yaml:"provider" json:"provider"` // "gitea", "github", "gitlab"
	URL      string `yaml:"url" json:"url"`           // e.g. "http://gitea-http.gitea.svc.cluster.local:3000"
	User     string `yaml:"user" json:"user"`
	Token    string `yaml:"token" json:"token"`
}

type BuildConfig struct {
	DefaultProvider string                         `yaml:"default_provider" json:"default_provider"` // "tekton", "local"
	Providers       map[string]BuildProviderConfig `yaml:"providers" json:"providers"`
	// Legacy
	Provider  string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

type BuildProviderConfig struct {
	Type       string `yaml:"type" json:"type"` // "tekton", "local"
	Namespace  string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	WorkingDir string `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`
}

type LLMConfig struct {
	DefaultProvider string                    `yaml:"default_provider" json:"default_provider"` // "gemini", "ollama", "lmstudio"
	Providers       map[string]ProviderConfig `yaml:"providers" json:"providers"`
}

type ProviderConfig struct {
	Type         string `yaml:"type" json:"type"` // "gemini", "ollama", "lmstudio"
	APIKey       string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	Model        string `yaml:"model,omitempty" json:"model,omitempty"` // Default model for this provider
	URL          string `yaml:"url,omitempty" json:"url,omitempty"`     // For local LLMs
	ProjectID    string `yaml:"project_id,omitempty" json:"project_id,omitempty"`
	ClientID     string `yaml:"client_id,omitempty" json:"client_id,omitempty"`
	ClientSecret string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"`
}

type ServerConfig struct {
	Port string `yaml:"port" json:"port"`
}

// Manager handles concurrent access to the configuration
type Manager struct {
	mu     sync.RWMutex
	config *Config
	path   string
}

// NewManager creates a new config manager and loads the config from path
func NewManager(path string) (*Manager, error) {
	// Helper to create a provider based on type and details
	mgr := &Manager{
		path: path,
		config: &Config{
			LLM: LLMConfig{
				DefaultProvider: "ollama", // Default
				Providers: map[string]ProviderConfig{
					"ollama": {Type: "ollama", Model: "qwen3:8b", URL: "http://localhost:11434"},
				},
			},
			Server: ServerConfig{
				Port: "8080",
			},
			Build: BuildConfig{
				DefaultProvider: "local",
				Providers: map[string]BuildProviderConfig{
					"tekton": {
						Type:      "tekton",
						Namespace: "default",
					},
					"local": {
						Type:       "local",
						WorkingDir: ".",
					},
				},
			},
			Git: GitConfig{
				Provider: "gitea",
				URL:      "http://gitea-http.gitea.svc.cluster.local:3000",
			},
		},
	}

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try to copy from config_default.yaml
		defaultPath := "config_default.yaml"
		// If path has a directory, try to find default there too (ignoring complex logic for now, assuming cwd)
		if _, err := os.Stat(defaultPath); err == nil {
			if err := copyFile(defaultPath, path); err != nil {
				return nil, fmt.Errorf("failed to copy default config: %w", err)
			}
			fmt.Printf("Initialized config from %s\n", defaultPath)
		}
	}

	// Try to load if file exists (either existed or was just copied)
	if _, err := os.Stat(path); err == nil {
		if err := mgr.Load(); err != nil {
			return nil, err
		}
	} else {
		// If still no file (e.g. no default found), try Env Vars
		mgr.loadEnv()
	}

	return mgr, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// Load reads the config from disk
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, m.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	return nil
}

// Save writes the current config to disk
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// Clone creates a deep copy of the Config
func (c Config) Clone() Config {
	newCfg := c
	if c.LLM.Providers != nil {
		newCfg.LLM.Providers = make(map[string]ProviderConfig, len(c.LLM.Providers))
		for k, v := range c.LLM.Providers {
			newCfg.LLM.Providers[k] = v
		}
	}
	if c.Build.Providers != nil {
		newCfg.Build.Providers = make(map[string]BuildProviderConfig, len(c.Build.Providers))
		for k, v := range c.Build.Providers {
			newCfg.Build.Providers[k] = v
		}
	}
	return newCfg
}

// Sanitize returns a copy of the config with sensitive data redacted
func (c Config) Sanitize() Config {
	safe := c.Clone()
	safe.Git.Token = ""

	for name, p := range safe.LLM.Providers {
		p.APIKey = ""
		safe.LLM.Providers[name] = p
	}
	return safe
}

// Get returns specific config copy
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Clone()
}

// Update updates the configuration and saves it
func (m *Manager) Update(newConfig Config) error {
	m.mu.Lock()
	m.config = &newConfig
	m.mu.Unlock()
	return m.Save()
}

func (m *Manager) loadEnv() {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		// Update gemini provider in map if exists, or create it
		if m.config.LLM.Providers == nil {
			m.config.LLM.Providers = make(map[string]ProviderConfig)
		}
		p := m.config.LLM.Providers["gemini"]
		p.Type = "gemini"
		p.APIKey = key
		m.config.LLM.Providers["gemini"] = p

		// Set default if not set
		if m.config.LLM.DefaultProvider == "" {
			m.config.LLM.DefaultProvider = "gemini"
		}
	}
	if port := os.Getenv("PORT"); port != "" {
		m.config.Server.Port = port
	}
}
