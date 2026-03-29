package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type GitRepo struct {
	Path string
	Repo *git.Repository
}

func OpenRepo(path string) (*GitRepo, error) {
	if path == "" || path == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		path = cwd
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for '%s': %w", path, err)
	}
	
	cleanPath := filepath.Clean(absPath)

	r, err := git.PlainOpenWithOptions(cleanPath, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open '%s' (resolved to '%s'): %w", path, cleanPath, err)
	}
	
	// Get the actual root of the repo
	w, _ := r.Worktree()
	actualPath := cleanPath
	if w != nil {
		actualPath = w.Filesystem.Root()
	}

	return &GitRepo{Path: actualPath, Repo: r}, nil
}

func (g *GitRepo) GetCurrentBranch() (string, error) {
	head, err := g.Repo.Head()
	if err != nil {
		return "", err
	}
	return head.Name().Short(), nil
}

func (g *GitRepo) GetBranches() ([]string, error) {
	branches, err := g.Repo.Branches()
	if err != nil {
		return nil, err
	}

	var names []string
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		names = append(names, ref.Name().Short())
		return nil
	})
	return names, err
}

func (g *GitRepo) GetTags() ([]string, error) {
	tags, err := g.Repo.Tags()
	if err != nil {
		return nil, err
	}

	var names []string
	err = tags.ForEach(func(ref *plumbing.Reference) error {
		names = append(names, ref.Name().Short())
		return nil
	})
	return names, err
}

func (g *GitRepo) GetGraph(limit int) (string, error) {
	args := []string{"log", "--graph", "--oneline", "--decorate", "--all", "--color=always"}
	if limit > 0 {
		args = append(args, "-n", fmt.Sprintf("%d", limit))
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get graph: %v (output: %s)", err, string(out))
	}
	return strings.ReplaceAll(string(out), "\r", "^M"), nil
}

type StatusItem struct {
	Path     string
	Staged   bool
	Status   string
	IsFile   bool
}

func (g *GitRepo) GetStatusItems() ([]StatusItem, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %v (output: %s)", err, string(out))
	}

	var items []StatusItem
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		
		staging := line[0]
		worktree := line[1]
		path := line[3:]
		
		// Handle renamed files: "R  old -> new"
		if staging == 'R' {
			parts := strings.Split(path, " -> ")
			if len(parts) > 1 {
				path = parts[1]
			}
		}

		staged := staging != ' ' && staging != '?'
		statusStr := string(staging)
		if staging == ' ' || staging == '?' {
			statusStr = string(worktree)
		}

		items = append(items, StatusItem{
			Path:   path,
			Staged: staged,
			Status: statusStr,
			IsFile: true,
		})
	}
	return items, nil
}

func (g *GitRepo) StageFile(path string) error {
	cmd := exec.Command("git", "add", path)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) UnstageFile(path string) error {
	cmd := exec.Command("git", "reset", "HEAD", path)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset HEAD failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) StageAll() error {
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add . failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) UnstageAll() error {
	cmd := exec.Command("git", "reset", "HEAD", ".")
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If it's a new repo with no commits, 'git reset' might fail. Try simple 'reset'
		cmd = exec.Command("git", "reset", ".")
		cmd.Dir = g.Path
		_, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git reset failed: %v (output: %s)", err, string(out))
		}
	}
	return nil
}

func (g *GitRepo) Fetch() (bool, error) {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("git fetch failed: %v (output: %s)", err, string(out))
	}
	return true, nil
}

func (g *GitRepo) Pull() (bool, error) {
	cmd := exec.Command("git", "pull", "origin")
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("git pull failed: %v (output: %s)", err, string(out))
	}
	return true, nil
}

func (g *GitRepo) Push() (bool, error) {
	cmd := exec.Command("git", "push", "origin")
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("git push failed: %v (output: %s)", err, string(out))
	}
	return true, nil
}

func (g *GitRepo) PushTags() (bool, error) {
	cmd := exec.Command("git", "push", "origin", "--tags")
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("git push tags failed: %v (output: %s)", err, string(out))
	}
	return true, nil
}

func (g *GitRepo) CreateRemote(name, url string) error {
	cmd := exec.Command("git", "remote", "add", name, url)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git remote add failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) DeleteRemote(name string) error {
	cmd := exec.Command("git", "remote", "remove", name)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git remote remove failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) Amend(message string) error {
	args := []string{"commit", "--amend", "--no-edit"}
	if message != "" {
		args = []string{"commit", "--amend", "-m", message}
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to amend: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) CherryPick(hash string) error {
	cmd := exec.Command("git", "cherry-pick", hash)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to cherry-pick: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) Merge(branch string) error {
	cmd := exec.Command("git", "merge", branch)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to merge: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) Checkout(branch string) error {
	// Using exec.Command for checkout to handle more cases (like new branches or remote tracking)
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to checkout: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) CreateBranch(name string) error {
	cmd := exec.Command("git", "branch", name)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) DeleteBranch(name string) error {
	// First try soft delete
	cmd := exec.Command("git", "branch", "-d", name)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If soft delete fails, try force delete
		cmd = exec.Command("git", "branch", "-D", name)
		cmd.Dir = g.Path
		out, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to delete branch: %v (output: %s)", err, string(out))
		}
	}
	return nil
}

func (g *GitRepo) CreateTag(name string) error {
	cmd := exec.Command("git", "tag", name)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git tag failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) DeleteTag(name string) error {
	cmd := exec.Command("git", "tag", "-d", name)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git tag -d failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) GetRemotes() ([]string, error) {
	remotes, err := g.Repo.Remotes()
	if err != nil {
		return nil, err
	}
	var details []string
	for _, r := range remotes {
		urls := r.Config().URLs
		urlStr := ""
		if len(urls) > 0 {
			urlStr = " (" + urls[0] + ")"
		}
		details = append(details, r.Config().Name+urlStr)
	}
	return details, err
}

func (g *GitRepo) GetCommitDiff(hash string) (string, error) {
	cmd := exec.Command("git", "show", "--color=always", hash)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(out), "\r", "^M"), nil
}

func (g *GitRepo) GetDiff(path string) (string, error) {
	cmd := exec.Command("git", "diff", "--color=always", "--ignore-submodules", "-a", path)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(out), "\r", "^M"), nil
}

func (g *GitRepo) GetStatusDiff(item StatusItem) (string, error) {
	var cmd *exec.Cmd
	if item.Status == "?" || item.Status == "??" {
		// For untracked files, show them as a new file diff
		cmd = exec.Command("git", "diff", "--no-index", "--color=always", "/dev/null", item.Path)
	} else if item.Staged {
		cmd = exec.Command("git", "diff", "--cached", "--color=always", "--ignore-submodules", "-a", item.Path)
	} else {
		cmd = exec.Command("git", "diff", "--color=always", "--ignore-submodules", "-a", item.Path)
	}

	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	
	// git diff --no-index exits with 1 if there is a diff
	if err != nil && cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != 1 {
		return "", fmt.Errorf("git diff failed: %v (output: %s)", err, string(out))
	}
	
	return strings.ReplaceAll(string(out), "\r", "^M"), nil
}

func (g *GitRepo) Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) Add(path string) error {
	cmd := exec.Command("git", "add", path)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) GetCommits(limit int) ([]string, error) {
	iter, err := g.Repo.Log(&git.LogOptions{Order: git.LogOrderCommitterTime})
	if err != nil {
		return nil, err
	}
	var commits []string
	count := 0
	err = iter.ForEach(func(c *object.Commit) error {
		if limit > 0 && count >= limit {
			return nil
		}
		msg := strings.Split(c.Message, "\n")[0]
		commits = append(commits, fmt.Sprintf("%s %s", c.Hash.String()[:7], msg))
		count++
		return nil
	})
	return commits, err
}

func CloneRepo(url, path string) (bool, error) {
	cmd := exec.Command("git", "clone", url, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return true, fmt.Errorf("git clone failed: %v (output: %s)", err, string(out))
	}
	return true, nil
}

func InitRepo(path string) error {
	cmd := exec.Command("git", "init", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init failed: %v (output: %s)", err, string(out))
	}
	return nil
}

func (g *GitRepo) Reset(path string) error {
	cmd := exec.Command("git", "reset", path)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %v (output: %s)", err, string(out))
	}
	return nil
}
