package git

import (
	"github.com/sachiniyer/agent-factory/config"
	"github.com/sachiniyer/agent-factory/log"
	"os/exec"
)

// RunPostWorktreeHooksAsync runs the per-repo post_worktree_commands in the
// background. Each command is executed sequentially via "sh -c" with the
// working directory set to worktreePath. Errors are logged but do not
// propagate — this is fire-and-forget.
func RunPostWorktreeHooksAsync(repoPath, worktreePath string) {
	repoID := config.RepoIDFromRoot(repoPath)
	repoCfg, err := config.LoadRepoConfig(repoID)
	if err != nil {
		log.WarningLog.Printf("failed to load repo config for hooks: %v", err)
		return
	}
	if len(repoCfg.PostWorktreeCommands) == 0 {
		return
	}

	cmds := repoCfg.PostWorktreeCommands
	go func() {
		for _, cmdStr := range cmds {
			log.InfoLog.Printf("running post-worktree hook in %s: %s", worktreePath, cmdStr)
			cmd := exec.Command("sh", "-c", cmdStr)
			cmd.Dir = worktreePath
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.ErrorLog.Printf("post-worktree hook %q failed: %v\n%s", cmdStr, err, string(output))
			} else {
				log.InfoLog.Printf("post-worktree hook %q completed successfully", cmdStr)
			}
		}
	}()
}
