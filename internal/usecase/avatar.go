package usecase

import (
	"os"
	"path/filepath"
	"strings"

	"chat-backend/internal/domain"
)

func findLatestByPrefix(dir, prefix string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", domain.ErrNotFound
	}

	var latestFile string
	var latestTime int64

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().UnixNano() > latestTime {
			latestTime = info.ModTime().UnixNano()
			latestFile = filepath.Join(dir, entry.Name())
		}
	}

	if latestFile == "" {
		return "", domain.ErrNotFound
	}
	return latestFile, nil
}
