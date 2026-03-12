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
	return string(out), nil
}

type StatusItem struct {
	Path     string
	Staged   bool
	Status   string
	IsFile   bool
}

func (g *GitRepo) GetStatusItems() ([]StatusItem, error) {
	w, err := g.Repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	var items []StatusItem
	for path, s := range status {
		staged := s.Staging != git.Unmodified && s.Staging != git.Untracked
		statusStr := string(s.Staging)
		if s.Staging == git.Unmodified || s.Staging == git.Untracked {
			statusStr = string(s.Worktree)
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
	w, err := g.Repo.Worktree()
	if err != nil {
		return err
	}
	_, err = w.Add(path)
	return err
}

func (g *GitRepo) UnstageFile(path string) error {
	w, err := g.Repo.Worktree()
	if err != nil {
		return err
	}
	// go-git doesn't have a direct "unstage" that matches 'git reset HEAD <file>' easily in one call
	// but we can use Reset with a specific file
	head, err := g.Repo.Head()
	if err != nil {
		// If no HEAD, we might be in an empty repo, just Reset might work
		return w.Reset(&git.ResetOptions{Files: []string{path}})
	}
	return w.Reset(&git.ResetOptions{Commit: head.Hash(), Files: []string{path}})
}

func (g *GitRepo) Fetch() error {
	return g.Repo.Fetch(&git.FetchOptions{RemoteName: "origin"})
}

func (g *GitRepo) Pull() error {
	w, err := g.Repo.Worktree()
	if err != nil {
		return err
	}
	return w.Pull(&git.PullOptions{RemoteName: "origin"})
}

func (g *GitRepo) Push() error {
	return g.Repo.Push(&git.PushOptions{RemoteName: "origin"})
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
	head, err := g.Repo.Head()
	if err != nil {
		return err
	}
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(name), head.Hash())
	return g.Repo.Storer.SetReference(ref)
}

func (g *GitRepo) DeleteBranch(name string) error {
	return g.Repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(name))
}

func (g *GitRepo) CreateTag(name string) error {
	head, err := g.Repo.Head()
	if err != nil {
		return err
	}
	_, err = g.Repo.CreateTag(name, head.Hash(), &git.CreateTagOptions{
		Message: name,
	})
	return err
}

func (g *GitRepo) DeleteTag(name string) error {
	return g.Repo.DeleteTag(name)
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
	return string(out), nil
}

func (g *GitRepo) GetDiff(path string) (string, error) {
	cmd := exec.Command("git", "diff", "--color=always", path)
	cmd.Dir = g.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (g *GitRepo) Commit(message string) error {
	w, err := g.Repo.Worktree()
	if err != nil {
		return err
	}
	_, err = w.Commit(message, &git.CommitOptions{})
	return err
}

func (g *GitRepo) Add(path string) error {
	w, err := g.Repo.Worktree()
	if err != nil {
		return err
	}
	_, err = w.Add(path)
	return err
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

func CloneRepo(url, path string) error {
	_, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	})
	return err
}

func InitRepo(path string) error {
	_, err := git.PlainInit(path, false)
	return err
}

func (g *GitRepo) Reset(path string) error {
	w, err := g.Repo.Worktree()
	if err != nil {
		return err
	}
	return w.Reset(&git.ResetOptions{
		Files: []string{path},
	})
}
