package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const RepoConfigFileName = "config.json"

// RepoConfig holds per-repository configuration.
type RepoConfig struct {
	// PostWorktreeCommands are shell commands run asynchronously in the worktree
	// directory after a new worktree is created.
	PostWorktreeCommands []string `json:"post_worktree_commands,omitempty"`
}

// LoadRepoConfig loads the per-repo config for the given repo ID.
// Returns an empty config (not an error) if none exists.
func LoadRepoConfig(repoID string) (*RepoConfig, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return &RepoConfig{}, nil
	}
	path := filepath.Join(configDir, "repos", repoID, RepoConfigFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RepoConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read repo config: %w", err)
	}
	var cfg RepoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse repo config: %w", err)
	}
	return &cfg, nil
}

// SaveRepoConfig saves the per-repo config for the given repo ID.
func SaveRepoConfig(repoID string, cfg *RepoConfig) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config dir: %w", err)
	}
	dir := filepath.Join(configDir, "repos", repoID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create repo config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal repo config: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, RepoConfigFileName), data, 0644)
}
