package registry

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/drug-nl/druppie/core/internal/model"
	"gopkg.in/yaml.v3"
)

// LoadRegistry scans the given root directory and populates a new Registry
func LoadRegistry(rootDir string) (*Registry, error) {
	reg := NewRegistry()

	// 1. Load Building Blocks (blocks -> mapped to BuildingBlocks)
	err := walkAndLoad(filepath.Join(rootDir, "blocks"), []string{".md"}, func(path string, fm []byte) error {
		var block model.BuildingBlock
		if err := yaml.Unmarshal(fm, &block); err != nil {
			return fmt.Errorf("failed to parse building block %s: %w", path, err)
		}
		// Fallback ID if missing
		if block.ID == "" {
			block.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		reg.mu.Lock()
		reg.BuildingBlocks[block.ID] = block
		reg.mu.Unlock()
		return nil
	})
	if err != nil {
		// Just log warning instead of failing hard? For now fail hard to ensure correctness
		// But directory might not exist yet
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading building blocks: %w", err)
		}
	}

	// 2. Load Skills
	err = walkAndLoad(filepath.Join(rootDir, "skills"), []string{".md"}, func(path string, fm []byte) error {
		var skill model.Skill
		if err := yaml.Unmarshal(fm, &skill); err != nil {
			return fmt.Errorf("failed to parse skill %s: %w", path, err)
		}
		if skill.ID == "" {
			skill.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		reg.mu.Lock()
		reg.Skills[skill.ID] = skill
		reg.mu.Unlock()
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error loading skills: %w", err)
	}

	// 3. Load Agents
	err = walkAndLoad(filepath.Join(rootDir, "agents"), []string{".yaml", ".yml", ".md"}, func(path string, fm []byte) error {
		var agent model.AgentDefinition
		if err := yaml.Unmarshal(fm, &agent); err != nil {
			return fmt.Errorf("failed to parse agent %s: %w", path, err)
		}
		if agent.ID == "" {
			agent.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		reg.mu.Lock()
		reg.Agents[agent.ID] = agent
		reg.mu.Unlock()
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error loading agents: %w", err)
	}

	// 4. Load MCP
	err = walkAndLoad(filepath.Join(rootDir, "mcp"), []string{".yaml", ".yml", ".md"}, func(path string, fm []byte) error {
		var mcp model.MCPServer
		if err := yaml.Unmarshal(fm, &mcp); err != nil {
			return fmt.Errorf("failed to parse mcp %s: %w", path, err)
		}
		if mcp.ID == "" {
			mcp.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		reg.mu.Lock()
		reg.MCPServers[mcp.ID] = mcp
		reg.mu.Unlock()
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error loading mcp: %w", err)
	}

	return reg, nil
}

// walkAndLoad walks a directory, finding files with allowed extensions, extracting data, and calling the handler.
// allowedExtensions: e.g. []string{".md", ".yaml"}
func walkAndLoad(dir string, allowedExtensions []string, handler func(path string, data []byte) error) error {
	// Check if dir exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return err
	}

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		isAllowed := false
		for _, allowed := range allowedExtensions {
			if ext == allowed {
				isAllowed = true
				break
			}
		}
		if !isAllowed {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var data []byte
		if ext == ".md" {
			// Parse Frontmatter
			fm, err := extractFrontmatter(content)
			if err != nil {
				// Error reading frontmatter
				return nil
			}
			if fm == nil {
				// No frontmatter found, skip
				return nil
			}
			data = fm
		} else if ext == ".yaml" || ext == ".yml" {
			// YAML file IS the data
			data = content
		}

		return handler(path, data)
	})
}

// extractFrontmatter peeks at the file content and extracts the YAML block between --- and ---
func extractFrontmatter(content []byte) ([]byte, error) {
	reader := bufio.NewReader(bytes.NewReader(content))

	// Check first line
	line, _, err := reader.ReadLine()
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(line, []byte("---")) {
		return nil, nil // No frontmatter
	}

	var fm bytes.Buffer
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			return nil, err
		}
		if bytes.Equal(line, []byte("---")) {
			break
		}
		fm.Write(line)
		fm.WriteByte('\n')
	}

	return fm.Bytes(), nil
}
