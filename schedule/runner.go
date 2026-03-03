package schedule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"claude-squad/config"
	"claude-squad/daemon"
	"claude-squad/log"
	"claude-squad/session"
	"claude-squad/session/git"
)

// RunScheduledTask executes a scheduled task by creating a new instance,
// sending the prompt, and registering it in the application state.
func RunScheduledTask(scheduleID string) error {
	log.Initialize(false)
	defer log.Close()

	// Create lock file to prevent overlapping runs.
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}
	lockDir := filepath.Join(configDir, "locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}
	lockPath := filepath.Join(lockDir, "schedule-"+scheduleID+".lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("another run is already active for schedule %s: %w", scheduleID, err)
	}
	defer os.Remove(lockPath)
	defer lockFile.Close()

	// Load the schedule.
	s, err := GetSchedule(scheduleID)
	if err != nil {
		return fmt.Errorf("failed to load schedule: %w", err)
	}

	if !s.Enabled {
		return fmt.Errorf("schedule %s is disabled", scheduleID)
	}

	// Validate project path.
	if !git.IsGitRepo(s.ProjectPath) {
		return fmt.Errorf("project path %s is not a valid git repository", s.ProjectPath)
	}

	cfg := config.LoadConfig()

	title := fmt.Sprintf("sched-%s-%s", s.ID, time.Now().Format("20060102-150405"))

	instance, err := session.NewInstance(session.InstanceOptions{
		Title:   title,
		Path:    s.ProjectPath,
		Program: s.Program,
	})
	if err != nil {
		return fmt.Errorf("failed to create instance: %w", err)
	}

	if err := instance.Start(true); err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	if err := instance.SendPrompt(s.Prompt); err != nil {
		return fmt.Errorf("failed to send prompt: %w", err)
	}

	// Append instance to state.json directly, without calling LoadInstances
	// which tries to restore tmux sessions and would fail headlessly.
	state := config.LoadState()
	var existingInstances []session.InstanceData
	if err := json.Unmarshal(state.GetInstances(), &existingInstances); err != nil {
		existingInstances = []session.InstanceData{}
	}
	existingInstances = append(existingInstances, instance.ToInstanceData())
	newJSON, err := json.Marshal(existingInstances)
	if err != nil {
		return fmt.Errorf("failed to marshal instances: %w", err)
	}
	if err := state.SaveInstances(newJSON); err != nil {
		return fmt.Errorf("failed to save instances: %w", err)
	}

	// Launch daemon for autoyes if configured.
	if cfg.AutoYes {
		instance.AutoYes = true
		if err := daemon.LaunchDaemon(); err != nil {
			log.ErrorLog.Printf("failed to launch daemon: %v", err)
		}
	}

	// Update schedule status.
	now := time.Now()
	s.LastRunAt = &now
	s.LastRunStatus = "started"
	if err := UpdateSchedule(*s); err != nil {
		log.ErrorLog.Printf("failed to update schedule status: %v", err)
	}

	log.InfoLog.Printf("scheduled task %s started successfully as instance %s", scheduleID, title)
	return nil
}
