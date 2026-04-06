package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/uuid"
)

// CanonBranch is the default branch name — the "official" story.
const CanonBranch = "canon"

// systemAuthor is used for all automated commits.
func systemAuthor() *object.Signature {
	return &object.Signature{
		Name:  "NexusTale",
		Email: "system@nexustale",
		When:  time.Now(),
	}
}

type GitService struct {
	reposPath string
}

func NewGitService(reposPath string) *GitService {
	return &GitService{reposPath: reposPath}
}

// InitRepo creates a non-bare git repo for the project with an initial commit
// on the "canon" branch. Safe to call on an already-initialised repo.
func (g *GitService) InitRepo(projectID uuid.UUID) (string, error) {
	repoPath := filepath.Join(g.reposPath, projectID.String())
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return "", fmt.Errorf("create repo dir: %w", err)
	}

	repo, err := git.PlainInit(repoPath, false)
	if errors.Is(err, git.ErrRepositoryAlreadyExists) {
		return repoPath, nil
	}
	if err != nil {
		return "", fmt.Errorf("init repo: %w", err)
	}

	// Point HEAD at "canon" before the first commit so the branch is created
	// with the right name from the start (otherwise go-git defaults to master).
	canonRef := plumbing.NewSymbolicReference(
		plumbing.HEAD,
		plumbing.NewBranchReferenceName(CanonBranch),
	)
	if err := repo.Storer.SetReference(canonRef); err != nil {
		return "", fmt.Errorf("set HEAD to canon: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get worktree: %w", err)
	}

	marker := filepath.Join(repoPath, ".nexustale")
	if err := os.WriteFile(marker, []byte("NexusTale project\n"), 0o644); err != nil {
		return "", fmt.Errorf("write marker: %w", err)
	}
	if _, err := wt.Add(".nexustale"); err != nil {
		return "", fmt.Errorf("stage marker: %w", err)
	}

	if _, err := wt.Commit("Initialize Canon", &git.CommitOptions{
		Author: systemAuthor(),
	}); err != nil {
		return "", fmt.Errorf("initial commit: %w", err)
	}

	return repoPath, nil
}

// Chronicle writes the provided file snapshot to the working tree and creates
// a commit on the current branch. Returns the full commit SHA.
// Returns ErrNothingToChronicle if the snapshot is identical to the last commit.
func (g *GitService) Chronicle(repoPath, note string, files map[string]string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("get worktree: %w", err)
	}

	for relPath, content := range files {
		full := filepath.Join(repoPath, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", relPath, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("write %s: %w", relPath, err)
		}
		if _, err := wt.Add(relPath); err != nil {
			return "", fmt.Errorf("stage %s: %w", relPath, err)
		}
	}

	hash, err := wt.Commit(note, &git.CommitOptions{
		Author:            systemAuthor(),
		AllowEmptyCommits: false,
	})
	if err != nil {
		// go-git returns an error when there is nothing to commit.
		if errors.Is(err, git.ErrEmptyCommit) {
			head, _ := repo.Head()
			return head.Hash().String(), ErrNothingToChronicle
		}
		return "", fmt.Errorf("commit: %w", err)
	}

	return hash.String(), nil
}

// Lore returns a paginated history of Chronicles on the current branch.
func (g *GitService) Lore(repoPath string, page, perPage int) ([]ChronicleEntry, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get head: %w", err)
	}

	iter, err := repo.Log(&git.LogOptions{From: head.Hash()})
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	defer iter.Close()

	skip := (page - 1) * perPage
	var entries []ChronicleEntry

	err = iter.ForEach(func(c *object.Commit) error {
		if skip > 0 {
			skip--
			return nil
		}
		if len(entries) >= perPage {
			return fmt.Errorf("stop") // sentinel; ForEach treats any error as stop
		}
		entries = append(entries, ChronicleEntry{
			SHA:       c.Hash.String(),
			ShortSHA:  c.Hash.String()[:7],
			Note:      c.Message,
			Author:    c.Author.Name,
			CreatedAt: c.Author.When,
		})
		return nil
	})
	if err != nil && err.Error() != "stop" {
		return nil, fmt.Errorf("iterate log: %w", err)
	}

	return entries, nil
}

// Echo returns a unified diff between two Chronicle SHAs.
func (g *GitService) Echo(repoPath, fromSHA, toSHA string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	fromCommit, err := repo.CommitObject(plumbing.NewHash(fromSHA))
	if err != nil {
		return "", fmt.Errorf("resolve from SHA %s: %w", fromSHA, err)
	}
	toCommit, err := repo.CommitObject(plumbing.NewHash(toSHA))
	if err != nil {
		return "", fmt.Errorf("resolve to SHA %s: %w", toSHA, err)
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("from tree: %w", err)
	}
	toTree, err := toCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("to tree: %w", err)
	}

	patch, err := fromTree.Patch(toTree)
	if err != nil {
		return "", fmt.Errorf("patch: %w", err)
	}

	return patch.String(), nil
}

// Timelines lists all branches (Timelines) in the repo.
func (g *GitService) Timelines(repoPath string) ([]TimelineInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get head: %w", err)
	}

	iter, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	defer iter.Close()

	var timelines []TimelineInfo
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		commit, err := repo.CommitObject(ref.Hash())
		if err != nil {
			return nil // skip malformed ref
		}
		timelines = append(timelines, TimelineInfo{
			Name:      ref.Name().Short(),
			IsCanon:   ref.Name().Short() == CanonBranch,
			IsActive:  ref.Name() == head.Name(),
			HeadSHA:   ref.Hash().String(),
			CreatedAt: commit.Author.When,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("iterate branches: %w", err)
	}

	return timelines, nil
}

// CurrentTimeline returns the name of the currently active Timeline (branch).
func (g *GitService) CurrentTimeline(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("get head: %w", err)
	}

	return head.Name().Short(), nil
}

// Diverge creates a new Timeline (branch) from fromSHA (or HEAD if empty) and
// switches the working tree to it.
func (g *GitService) Diverge(repoPath, timelineName, fromSHA string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}

	var hash plumbing.Hash
	if fromSHA == "" {
		head, err := repo.Head()
		if err != nil {
			return fmt.Errorf("get head: %w", err)
		}
		hash = head.Hash()
	} else {
		hash = plumbing.NewHash(fromSHA)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	return wt.Checkout(&git.CheckoutOptions{
		Hash:   hash,
		Branch: plumbing.NewBranchReferenceName(timelineName),
		Create: true,
	})
}

// TravelTo switches the working tree to an existing Timeline (branch).
func (g *GitService) TravelTo(repoPath, timelineName string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	return wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(timelineName),
	})
}

// Canonize merges a Timeline into Canon via fast-forward.
// If the histories have diverged (a Paradox), it returns a result with
// HasParadox=true rather than an error — the caller surfaces this to the writer.
func (g *GitService) Canonize(repoPath, timelineName string) (*CanonizeResult, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	canonRef, err := repo.Reference(plumbing.NewBranchReferenceName(CanonBranch), true)
	if err != nil {
		return nil, fmt.Errorf("resolve canon: %w", err)
	}

	timelineRef, err := repo.Reference(plumbing.NewBranchReferenceName(timelineName), true)
	if err != nil {
		return nil, fmt.Errorf("resolve timeline %s: %w", timelineName, err)
	}

	canonCommit, err := repo.CommitObject(canonRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("canon commit: %w", err)
	}
	timelineCommit, err := repo.CommitObject(timelineRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("timeline commit: %w", err)
	}

	// Check whether the timeline's HEAD is already in the canon history
	// (already canonized / timeline is behind).
	isTimelineAncestor, err := timelineCommit.IsAncestor(canonCommit)
	if err != nil {
		return nil, fmt.Errorf("ancestor check: %w", err)
	}
	if isTimelineAncestor {
		// Timeline is behind or equal — nothing to merge.
		return &CanonizeResult{MergedSHA: canonRef.Hash().String()}, nil
	}

	// Check whether canon is an ancestor of the timeline (fast-forward possible).
	isCanonAncestor, err := canonCommit.IsAncestor(timelineCommit)
	if err != nil {
		return nil, fmt.Errorf("ancestor check: %w", err)
	}
	if !isCanonAncestor {
		return &CanonizeResult{
			HasParadox:  true,
			ParadoxNote: fmt.Sprintf("Timeline '%s' has diverged from Canon. Paradox resolution required — resolve manually with `git merge` from the repo at %s.", timelineName, repoPath),
		}, nil
	}

	// Fast-forward: advance canon to the timeline's HEAD.
	newRef := plumbing.NewHashReference(
		plumbing.NewBranchReferenceName(CanonBranch),
		timelineRef.Hash(),
	)
	if err := repo.Storer.SetReference(newRef); err != nil {
		return nil, fmt.Errorf("advance canon ref: %w", err)
	}

	// Update the working tree so HEAD matches the new canon tip.
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(CanonBranch),
		Force:  true,
	}); err != nil {
		return nil, fmt.Errorf("checkout canon: %w", err)
	}

	return &CanonizeResult{MergedSHA: timelineRef.Hash().String()}, nil
}

// ErrNothingToChronicle is returned by Chronicle when the working tree is
// identical to the last commit (no changes to record).
var ErrNothingToChronicle = errors.New("nothing to chronicle: no changes since last chronicle")
