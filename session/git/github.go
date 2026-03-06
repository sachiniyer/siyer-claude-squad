package git

import (
	"encoding/json"
	"os/exec"
)

// PRInfo holds information about a GitHub pull request associated with a branch.
type PRInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	State  string `json:"state"`
}

// FetchPRInfo runs `gh pr view` to look up a PR for the given branch.
// Returns nil with no error if no PR exists or gh is not installed.
func FetchPRInfo(repoPath, branchName string) (*PRInfo, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, nil
	}

	cmd := exec.Command("gh", "pr", "view", branchName, "--json", "number,title,url,state")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		// Non-zero exit means no PR found (or other error) — treat as no PR.
		return nil, nil
	}

	var info PRInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, nil
	}
	if info.Number == 0 {
		return nil, nil
	}
	return &info, nil
}
