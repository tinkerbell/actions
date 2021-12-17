package git

import (
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// TinkerbellAction consists of a name and a major version.
type TinkerbellAction struct {
	Name    string
	Version string
}

func (a TinkerbellAction) String() string {
	return a.Name + "/" + a.Version
}

// NewTinkerbellAction creates a new Tinkerbell action.
func NewTinkerbellAction(file string) TinkerbellAction {
	fragments := strings.Split(file, "/")
	return TinkerbellAction{
		Name:    fragments[1],
		Version: fragments[2],
	}
}

// ModifiedActions analyses the Git commit history and determines which actions have been modified.
func ModifiedActions(modifiedActions *[]TinkerbellAction, actionsPath string, context string, gitRef string) error {
	// Check if Git is available in PATH.
	if out, err := exec.Command("git", "version").Output(); err != nil || strings.HasPrefix("git version", string(out)) {
		return errors.New("Failed to check git version. Do you have git installed?")
	}

	// Get all changes since the last commit. The "HEAD^@"
	// ensures that merge commits return correct changes.
	out, err := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", gitRef, context).Output()
	if err != nil {
		execErr := err.(*exec.ExitError)
		return errors.Wrap(err, strings.ReplaceAll(string(execErr.Stderr), "\n", " "))
	}

	// Use a map to deduplicate modified action entries.
	detected := make(map[string]bool)
	actionsDir := actionsPath + "/"
	for _, file := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(file, actionsDir) {
			action := NewTinkerbellAction(file)

			// Deduplicate entries by checking if a modification for this action was already detected.
			if !detected[action.String()] {
				detected[action.String()] = true
				*modifiedActions = append(*modifiedActions, action)
			}
		}
	}

	return nil
}
