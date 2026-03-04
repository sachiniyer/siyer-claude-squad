package nanoclaw

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var nanoclawDirFlag string

// NanoClawCmd is the cobra command for the interactive nanoclaw TUI.
var NanoClawCmd = &cobra.Command{
	Use:   "nanoclaw",
	Short: "Interactive TUI for NanoClaw messaging",
	Long:  "Opens an interactive chat interface for sending and receiving messages through the NanoClaw bridge.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := nanoclawDirFlag
		if dir == "" {
			dir = os.Getenv("NANOCLAW_DIR")
		}
		bridge := NewBridge(dir)
		if !bridge.Available() {
			return fmt.Errorf("NanoClaw not available — set NANOCLAW_DIR or install at ~/nanoclaw")
		}

		groups, err := bridge.ListGroups()
		if err != nil {
			return fmt.Errorf("failed to list groups: %w", err)
		}
		if len(groups) == 0 {
			return fmt.Errorf("no NanoClaw groups found")
		}

		// Pick the main group
		group := groups[0]
		for _, g := range groups {
			if g.IsMain == 1 {
				group = g
				break
			}
		}

		// Build metadata from environment
		meta := &MessageMeta{}
		if cwd, err := os.Getwd(); err == nil {
			meta.RepoPath = cwd
		}

		return RunTUI(bridge, group, meta)
	},
}

func init() {
	NanoClawCmd.Flags().StringVar(&nanoclawDirFlag, "dir", "", "NanoClaw directory (defaults to NANOCLAW_DIR or ~/nanoclaw)")
}
