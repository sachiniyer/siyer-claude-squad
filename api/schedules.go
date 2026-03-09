package api

import (
	"fmt"
	"path/filepath"
	"time"

	"claude-squad/config"
	"claude-squad/log"
	"claude-squad/schedule"

	"github.com/spf13/cobra"
)

var schedulesCmd = &cobra.Command{
	Use:   "schedules",
	Short: "Manage schedules",
}

var schedulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List schedules",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		schedules, err := schedule.LoadSchedules()
		if err != nil {
			return jsonError(fmt.Errorf("failed to load schedules: %w", err))
		}

		// Filter by repo if --repo is set
		if repoFlag != "" {
			absPath, err := filepath.Abs(repoFlag)
			if err != nil {
				return jsonError(fmt.Errorf("failed to resolve repo path: %w", err))
			}
			repo, err := config.RepoFromPath(absPath)
			if err != nil {
				return jsonError(fmt.Errorf("failed to get repo from path: %w", err))
			}
			var filtered []schedule.Schedule
			for _, s := range schedules {
				if s.ProjectPath == repo.Root {
					filtered = append(filtered, s)
				}
			}
			schedules = filtered
		}

		if schedules == nil {
			schedules = []schedule.Schedule{}
		}
		return jsonOut(schedules)
	},
}

var (
	schedAddNameFlag    string
	schedAddPromptFlag  string
	schedAddCronFlag    string
	schedAddProgramFlag string
)

var schedulesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new schedule",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		if err := schedule.ValidateCronExpr(schedAddCronFlag); err != nil {
			return jsonError(fmt.Errorf("invalid cron expression: %w", err))
		}

		program := schedAddProgramFlag
		if program == "" {
			program = config.LoadConfig().DefaultProgram
		}

		id := schedule.GenerateID()
		s := schedule.Schedule{
			ID:          id,
			Name:        schedAddNameFlag,
			Prompt:      schedAddPromptFlag,
			CronExpr:    schedAddCronFlag,
			ProjectPath: repo.Root,
			Program:     program,
			Enabled:     true,
			CreatedAt:   time.Now(),
		}

		if err := schedule.AddSchedule(s); err != nil {
			return jsonError(fmt.Errorf("failed to add schedule: %w", err))
		}

		if err := schedule.InstallSystemdTimer(s); err != nil {
			return jsonError(fmt.Errorf("failed to install systemd timer: %w", err))
		}

		return jsonOut(map[string]any{"id": id})
	},
}

var schedulesRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		s, err := schedule.GetSchedule(args[0])
		if err != nil {
			return jsonError(fmt.Errorf("failed to get schedule: %w", err))
		}

		if err := schedule.RemoveSystemdTimer(*s); err != nil {
			return jsonError(fmt.Errorf("failed to remove systemd timer: %w", err))
		}

		if err := schedule.RemoveSchedule(args[0]); err != nil {
			return jsonError(fmt.Errorf("failed to remove schedule: %w", err))
		}

		return jsonOut(map[string]bool{"ok": true})
	},
}
