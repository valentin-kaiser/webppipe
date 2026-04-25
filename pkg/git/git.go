// Package git wraps a small subset of the git CLI used by webppipe to commit
// and push converted images.
package git

import (
	"bytes"
	"os/exec"
	"strings"

	"github.com/valentin-kaiser/go-core/apperror"
)

// Client invokes the `git` binary in a fixed working directory.
type Client struct {
	Dir string
}

func (c *Client) run(args ...string) (string, error) {
	cmd := exec.Command("git", args...) //nolint:gosec // args are constructed internally
	cmd.Dir = c.Dir
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return out.String(), apperror.NewErrorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(errBuf.String())).AddError(err)
	}
	return out.String(), nil
}

// IsRepo reports whether Dir is inside a git working tree.
func (c *Client) IsRepo() bool {
	_, err := c.run("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// HasChanges returns true when the working tree has uncommitted changes.
func (c *Client) HasChanges() (bool, error) {
	out, err := c.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Configure sets user.name and user.email for subsequent commits in this repo.
func (c *Client) Configure(name, email string) error {
	if name != "" {
		if _, err := c.run("config", "user.name", name); err != nil {
			return err
		}
	}
	if email != "" {
		if _, err := c.run("config", "user.email", email); err != nil {
			return err
		}
	}
	return nil
}

// AddPaths stages the given repository-relative paths.
func (c *Client) AddPaths(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	_, err := c.run(args...)
	return err
}

// Commit creates a commit with the given message. Returns nil if there is
// nothing staged.
func (c *Client) Commit(message string) error {
	out, err := c.run("status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil
	}
	_, err = c.run("commit", "-m", message)
	return err
}

// CurrentBranch returns the name of the currently checked-out branch, or
// empty string in a detached-HEAD state.
func (c *Client) CurrentBranch() (string, error) {
	out, err := c.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(out)
	if b == "HEAD" {
		return "", nil
	}
	return b, nil
}

// Push pushes the given branch (or the current branch when empty) to origin.
func (c *Client) Push(branch string) error {
	if branch == "" {
		var err error
		branch, err = c.CurrentBranch()
		if err != nil {
			return err
		}
	}
	if branch == "" {
		return apperror.NewError("cannot push: detached HEAD and no branch specified")
	}
	_, err := c.run("push", "origin", "HEAD:"+branch)
	return err
}
