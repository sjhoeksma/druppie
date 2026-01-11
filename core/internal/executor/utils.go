package executor

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SaveAsset helper to store files (images, audio, video) in the plan's files directory
func SaveAsset(planID, filename, data string) error {
	basePath := fmt.Sprintf(".druppie/plans/%s/files", planID)
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return err
	}
	fullPath := filepath.Join(basePath, filename)

	var content []byte
	var err error

	if strings.HasPrefix(data, "base64,") {
		parts := strings.Split(data, ",")
		if len(parts) > 1 {
			data = parts[len(parts)-1]
		}
		content, err = base64.StdEncoding.DecodeString(data)
	} else if strings.HasPrefix(data, "http") {
		resp, hErr := http.Get(data)
		if hErr != nil {
			return hErr
		}
		defer resp.Body.Close()
		content, err = io.ReadAll(resp.Body)
	} else {
		// Try base64 anyway, but fallback to raw bytes for mocks/placeholders
		content, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			content = []byte(data)
			err = nil
		}
	}

	if err != nil {
		return err
	}

	return os.WriteFile(fullPath, content, 0644)
}
