package iam

import (
	"os"
	"path/filepath"
	"strings"
)

// getClientTokenPath returns path to ~/.druppie/token
func getClientTokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".druppie")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "token"), nil
}

func SaveClientToken(token string) error {
	path, err := getClientTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0600)
}

func LoadClientToken() (string, error) {
	path, err := getClientTokenPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func ClearClientToken() error {
	path, err := getClientTokenPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}
