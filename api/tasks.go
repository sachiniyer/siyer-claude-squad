package api

import (
	"fmt"

	"claude-squad/log"
	"claude-squad/task"

	"github.com/spf13/cobra"
)

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage tasks",
}

var tasksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks for a repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		tasks, err := task.LoadTasksForRepo(repo)
		if err != nil {
			return jsonError(fmt.Errorf("failed to load tasks: %w", err))
		}
		return jsonOut(tasks)
	},
}

var (
	taskAddTitleFlag    string
	taskAddStatusFlag   string
	taskAddInstanceFlag string
	taskLinkInstanceFlag string
	tasksMoveStatusFlag  string
)

var tasksAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a task",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		status := taskAddStatusFlag
		if status == "" {
			status = "backlog"
		}

		board, err := task.LoadBoardForRepo(repo)
		if err != nil {
			return jsonError(fmt.Errorf("failed to load board: %w", err))
		}
		t := board.AddTask(taskAddTitleFlag, status)
		if taskAddInstanceFlag != "" {
			board.LinkTask(t.ID, taskAddInstanceFlag)
			// Re-fetch so we output the linked version
			t.InstanceTitle = taskAddInstanceFlag
		}
		if err := task.SaveBoardForRepo(repo, board); err != nil {
			return jsonError(fmt.Errorf("failed to save board: %w", err))
		}
		return jsonOut(t)
	},
}

var tasksToggleCmd = &cobra.Command{
	Use:   "toggle <id>",
	Short: "Toggle a task's done status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		if err := task.ToggleTaskForRepo(repo, args[0]); err != nil {
			return jsonError(fmt.Errorf("failed to toggle task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var tasksRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		if err := task.DeleteTaskForRepo(repo, args[0]); err != nil {
			return jsonError(fmt.Errorf("failed to remove task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var tasksMoveCmd = &cobra.Command{
	Use:   "move <id>",
	Short: "Move a task to a different column",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		if tasksMoveStatusFlag == "" {
			return jsonError(fmt.Errorf("--status is required"))
		}

		if err := task.MoveTaskForRepo(repo, args[0], tasksMoveStatusFlag); err != nil {
			return jsonError(fmt.Errorf("failed to move task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var tasksLinkCmd = &cobra.Command{
	Use:   "link <id>",
	Short: "Link a task to a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		if taskLinkInstanceFlag == "" {
			return jsonError(fmt.Errorf("--instance is required"))
		}

		if err := task.LinkTaskForRepo(repo, args[0], taskLinkInstanceFlag); err != nil {
			return jsonError(fmt.Errorf("failed to link task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var tasksUnlinkCmd = &cobra.Command{
	Use:   "unlink <id>",
	Short: "Remove linkage from a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		if err := task.UnlinkTaskForRepo(repo, args[0]); err != nil {
			return jsonError(fmt.Errorf("failed to unlink task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var tasksBoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Get kanban board (columns + tasks grouped by status)",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		board, err := task.LoadBoardForRepo(repo)
		if err != nil {
			return jsonError(fmt.Errorf("failed to load board: %w", err))
		}

		// Group tasks by column for output
		grouped := make(map[string][]task.Task)
		for _, col := range board.Columns {
			grouped[col] = board.GetTasksByStatus(col)
			if grouped[col] == nil {
				grouped[col] = []task.Task{}
			}
		}

		return jsonOut(map[string]any{
			"columns": board.Columns,
			"tasks":   grouped,
		})
	},
}
