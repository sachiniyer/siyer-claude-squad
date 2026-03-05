package session

import (
	"claude-squad/config"
	"claude-squad/log"
	"encoding/json"
	"fmt"
	"time"
)

// InstanceData represents the serializable data of an Instance
type InstanceData struct {
	Title     string    `json:"title"`
	Path      string    `json:"path"`
	Branch    string    `json:"branch"`
	Status    Status    `json:"status"`
	Height    int       `json:"height"`
	Width     int       `json:"width"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	AutoYes   bool      `json:"auto_yes"`

	Program   string          `json:"program"`
	Worktree  GitWorktreeData `json:"worktree"`
	DiffStats DiffStatsData   `json:"diff_stats"`
}

// GitWorktreeData represents the serializable data of a GitWorktree
type GitWorktreeData struct {
	RepoPath         string `json:"repo_path"`
	WorktreePath     string `json:"worktree_path"`
	SessionName      string `json:"session_name"`
	BranchName       string `json:"branch_name"`
	BaseCommitSHA    string `json:"base_commit_sha"`
	ExternalWorktree bool   `json:"external_worktree,omitempty"`
}

// DiffStatsData represents the serializable data of a DiffStats
type DiffStatsData struct {
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
	Content string `json:"content"`
}

// Storage handles saving and loading instances using the state interface.
// When repoID is set (TUI mode), operations are scoped to that repo.
// When repoID is empty (daemon mode), operations span all repos.
type Storage struct {
	state  config.InstanceStorage
	repoID string
}

// NewStorage creates a new storage instance.
// Pass a non-empty repoID for TUI (repo-scoped) mode, or "" for daemon (all-repo) mode.
func NewStorage(state config.InstanceStorage, repoID string) (*Storage, error) {
	return &Storage{
		state:  state,
		repoID: repoID,
	}, nil
}

// SaveInstances saves the list of instances to disk.
func (s *Storage) SaveInstances(instances []*Instance) error {
	// Convert instances to InstanceData
	data := make([]InstanceData, 0)
	for _, instance := range instances {
		if instance.Started() {
			data = append(data, instance.ToInstanceData())
		}
	}

	if s.repoID != "" {
		// TUI mode: merge with on-disk state to preserve externally-created sessions
		// (e.g. sessions created by `cs api sessions create` while the TUI was running)
		merged, err := s.mergeWithDisk(data)
		if err != nil {
			return fmt.Errorf("failed to merge instances: %w", err)
		}
		jsonData, err := json.Marshal(merged)
		if err != nil {
			return fmt.Errorf("failed to marshal instances: %w", err)
		}
		return s.state.SaveInstances(s.repoID, jsonData)
	}

	// Daemon mode: group by repo and save each group separately
	grouped := make(map[string][]InstanceData)
	for _, d := range data {
		rid := config.RepoIDFromRoot(d.Worktree.RepoPath)
		grouped[rid] = append(grouped[rid], d)
	}
	for rid, group := range grouped {
		jsonData, err := json.Marshal(group)
		if err != nil {
			return fmt.Errorf("failed to marshal instances for repo %s: %w", rid, err)
		}
		if err := s.state.SaveInstances(rid, jsonData); err != nil {
			return err
		}
	}
	return nil
}

// LoadInstances loads the list of instances from disk.
func (s *Storage) LoadInstances() ([]*Instance, error) {
	var allJSON map[string]json.RawMessage
	if s.repoID != "" {
		// TUI mode: load just this repo
		raw := s.state.GetInstances(s.repoID)
		allJSON = map[string]json.RawMessage{s.repoID: raw}
	} else {
		// Daemon mode: load all repos
		allJSON = s.state.GetAllInstances()
	}

	var instances []*Instance
	for _, jsonData := range allJSON {
		if jsonData == nil || string(jsonData) == "[]" || string(jsonData) == "null" {
			continue
		}
		var instancesData []InstanceData
		if err := json.Unmarshal(jsonData, &instancesData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal instances: %w", err)
		}
		for _, data := range instancesData {
			instance, err := FromInstanceData(data)
			if err != nil {
				// Instance's tmux session or worktree may have been
				// destroyed externally. Log and skip rather than
				// failing the entire load.
				log.WarningLog.Printf("skipping instance %q: %v", data.Title, err)
				continue
			}
			instances = append(instances, instance)
		}
	}

	return instances, nil
}

// DeleteInstance removes an instance from storage
func (s *Storage) DeleteInstance(title string) error {
	instances, err := s.LoadInstances()
	if err != nil {
		return fmt.Errorf("failed to load instances: %w", err)
	}

	found := false
	newInstances := make([]*Instance, 0)
	for _, instance := range instances {
		data := instance.ToInstanceData()
		if data.Title != title {
			newInstances = append(newInstances, instance)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("instance not found: %s", title)
	}

	return s.SaveInstances(newInstances)
}


// mergeWithDisk reads the current on-disk instances and merges them with the
// in-memory set. Instances known to the TUI (by title) are replaced with the
// in-memory version. Instances that only exist on disk (created externally)
// are preserved.
func (s *Storage) mergeWithDisk(memoryData []InstanceData) ([]InstanceData, error) {
	raw := s.state.GetInstances(s.repoID)
	if raw == nil || string(raw) == "[]" || string(raw) == "null" {
		return memoryData, nil
	}

	var diskData []InstanceData
	if err := json.Unmarshal(raw, &diskData); err != nil {
		// Can't parse disk data, just use memory
		return memoryData, nil
	}

	// Build a set of titles the TUI knows about
	knownTitles := make(map[string]bool, len(memoryData))
	for _, d := range memoryData {
		knownTitles[d.Title] = true
	}

	// Keep disk-only instances (ones the TUI doesn't know about)
	merged := make([]InstanceData, 0, len(memoryData)+len(diskData))
	merged = append(merged, memoryData...)
	for _, d := range diskData {
		if !knownTitles[d.Title] {
			merged = append(merged, d)
		}
	}

	return merged, nil
}

// DeleteAllInstances removes all stored instances
func (s *Storage) DeleteAllInstances() error {
	return s.state.DeleteAllInstances()
}
