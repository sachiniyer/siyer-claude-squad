package api

import (
	"fmt"

	"github.com/sachiniyer/agent-factory/board"
	"github.com/sachiniyer/agent-factory/log"

	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Manage board",
}

var boardListCmd = &cobra.Command{
	Use:   "list",
	Short: "List board items for a repo",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		tasks, err := board.LoadTasksForRepo(repo)
		if err != nil {
			return jsonError(fmt.Errorf("failed to load tasks: %w", err))
		}
		return jsonOut(tasks)
	},
}

var (
	boardAddTitleFlag     string
	boardAddStatusFlag    string
	boardAddInstanceFlag  string
	boardLinkInstanceFlag string
	boardMoveStatusFlag   string
)

var boardAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a task",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		status := boardAddStatusFlag
		if status == "" {
			status = "backlog"
		}

		b, err := board.LoadBoardForRepo(repo)
		if err != nil {
			return jsonError(fmt.Errorf("failed to load board: %w", err))
		}
		t := b.AddTask(boardAddTitleFlag, status)
		if boardAddInstanceFlag != "" {
			b.LinkTask(t.ID, boardAddInstanceFlag)
			// Re-fetch so we output the linked version
			t.InstanceTitle = boardAddInstanceFlag
		}
		if err := board.SaveBoardForRepo(repo, b); err != nil {
			return jsonError(fmt.Errorf("failed to save board: %w", err))
		}
		return jsonOut(t)
	},
}

var boardToggleCmd = &cobra.Command{
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

		if err := board.ToggleTaskForRepo(repo, args[0]); err != nil {
			return jsonError(fmt.Errorf("failed to toggle task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var boardRemoveCmd = &cobra.Command{
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

		if err := board.DeleteTaskForRepo(repo, args[0]); err != nil {
			return jsonError(fmt.Errorf("failed to remove task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var boardMoveCmd = &cobra.Command{
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

		if boardMoveStatusFlag == "" {
			return jsonError(fmt.Errorf("--status is required"))
		}

		if err := board.MoveTaskForRepo(repo, args[0], boardMoveStatusFlag); err != nil {
			return jsonError(fmt.Errorf("failed to move task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var boardLinkCmd = &cobra.Command{
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

		if boardLinkInstanceFlag == "" {
			return jsonError(fmt.Errorf("--instance is required"))
		}

		if err := board.LinkTaskForRepo(repo, args[0], boardLinkInstanceFlag); err != nil {
			return jsonError(fmt.Errorf("failed to link task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var boardUnlinkCmd = &cobra.Command{
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

		if err := board.UnlinkTaskForRepo(repo, args[0]); err != nil {
			return jsonError(fmt.Errorf("failed to unlink task: %w", err))
		}
		return jsonOut(map[string]bool{"ok": true})
	},
}

var boardViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Get kanban board (columns + tasks grouped by status)",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Initialize(false)
		defer log.Close()

		repo, err := resolveRepo()
		if err != nil {
			return jsonError(fmt.Errorf("--repo is required: %w", err))
		}

		b, err := board.LoadBoardForRepo(repo)
		if err != nil {
			return jsonError(fmt.Errorf("failed to load board: %w", err))
		}

		// Group tasks by column for output
		grouped := make(map[string][]board.Task)
		for _, col := range b.Columns {
			grouped[col] = b.GetTasksByStatus(col)
			if grouped[col] == nil {
				grouped[col] = []board.Task{}
			}
		}

		return jsonOut(map[string]any{
			"columns": b.Columns,
			"tasks":   grouped,
		})
	},
}
