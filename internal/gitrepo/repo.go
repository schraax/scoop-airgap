// Package gitrepo manages the local clone of the internal mirrored bucket repo.
package gitrepo

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repo represents the local clone of the internal bucket Git repository.
type Repo struct {
	remoteURL string // token-embedded URL used for git operations
	cleanURL  string // plain URL without credentials, used for PR API calls
	authToken string // raw token for PR API authentication
	branch    string
	localPath string
}

// New creates a Repo. If authToken is non-empty it is embedded into an HTTPS
// URL as https://token@host/path, which works with Gitea, GitLab, and GitHub.
func New(remoteURL, branch, localPath, authToken string) (*Repo, error) {
	cleanURL := remoteURL
	if authToken != "" {
		u, err := url.Parse(remoteURL)
		if err != nil {
			return nil, fmt.Errorf("parse repo URL: %w", err)
		}
		u.User = url.User(authToken)
		remoteURL = u.String()
	}
	if localPath == "" {
		tmp, err := os.MkdirTemp("", "scoop-airgap-bucket-*")
		if err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		localPath = tmp
	}
	return &Repo{
		remoteURL: remoteURL,
		cleanURL:  cleanURL,
		authToken: authToken,
		branch:    branch,
		localPath: localPath,
	}, nil
}

// EnsureReady clones the repo if it is not already present, or pulls the
// latest changes if it is.
func (r *Repo) EnsureReady() error {
	if _, err := os.Stat(filepath.Join(r.localPath, ".git")); os.IsNotExist(err) {
		return r.clone()
	}
	return r.pull()
}

// WriteManifest writes the rewritten manifest JSON into the repo at
// bucket/{appName}.json — the standard Scoop bucket layout, so that clients
// can add this repo with `scoop bucket add <name> <url>` unchanged.
func (r *Repo) WriteManifest(appName string, data []byte) error {
	dir := filepath.Join(r.localPath, "bucket")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	dest := filepath.Join(dir, appName+".json")
	return os.WriteFile(dest, data, 0o644)
}

// CommitAndPush stages all changes, creates a commit, and pushes to the remote.
// Returns nil if there is nothing to commit.
func (r *Repo) CommitAndPush(message string) error {
	// Stage everything
	if err := r.git("add", "--all"); err != nil {
		return err
	}

	// Check if there is anything to commit
	out, err := r.gitOutput("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil // nothing changed
	}

	if err := r.git("commit", "--message", message,
		"--author", "scoop-airgap <scoop-airgap@noreply>"); err != nil {
		return err
	}
	return r.git("push", "origin", r.branch)
}

// CommitAndPR is like CommitAndPush but instead of pushing to the configured
// branch it pushes the changes to prBranch and opens a pull request against
// the base branch. Returns nil without opening a PR if there is nothing to
// commit. The caller should choose a prBranch name that is unique per run
// (e.g. include a timestamp).
func (r *Repo) CommitAndPR(commitMsg, prBranch, prTitle, prBody string) error {
	if err := r.git("add", "--all"); err != nil {
		return err
	}
	out, err := r.gitOutput("status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil
	}

	if err := r.git("checkout", "-b", prBranch); err != nil {
		return fmt.Errorf("create PR branch: %w", err)
	}
	if err := r.git("commit", "--message", commitMsg,
		"--author", "scoop-airgap <scoop-airgap@noreply>"); err != nil {
		return err
	}
	if err := r.git("push", "origin", prBranch); err != nil {
		return fmt.Errorf("push PR branch: %w", err)
	}
	// Return to the base branch so EnsureReady works correctly on future runs.
	if err := r.git("checkout", r.branch); err != nil {
		return fmt.Errorf("checkout base branch after PR push: %w", err)
	}
	if err := CreatePR(r.cleanURL, r.authToken, r.branch, prBranch, prTitle, prBody); err != nil {
		return fmt.Errorf("open pull request (branch %s was pushed; open it manually if needed): %w", prBranch, err)
	}
	return nil
}

// LocalPath returns the local working directory of the clone.
func (r *Repo) LocalPath() string { return r.localPath }

func (r *Repo) clone() error {
	if err := os.MkdirAll(r.localPath, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", r.localPath, err)
	}
	return r.git("clone", "--branch", r.branch, "--single-branch", r.remoteURL, r.localPath)
}

func (r *Repo) pull() error {
	return r.git("-C", r.localPath, "pull", "--ff-only", "origin", r.branch)
}

func (r *Repo) git(args ...string) error {
	_, err := r.gitOutput(args...)
	return err
}

func (r *Repo) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec
	cmd.Dir = r.localPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return string(out), nil
}
