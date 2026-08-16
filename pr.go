package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PR is the state of the pull request a branch belongs to.
type PR struct {
	Number int
	State  string // open, draft, merged or closed
	URL    string
}

// Label is what the list shows for the pull request.
func (p PR) Label() string {
	if p.Number == 0 {
		return p.State
	}
	return fmt.Sprintf("%s #%d", p.State, p.Number)
}

// pullRequests maps branch name to the pull request that says most about it.
// It asks the GitHub CLI, and falls back to a local ancestry check when gh is
// missing, unauthenticated or offline — that still catches merges, as long as
// they were not squashed or rebased.
func pullRequests(branches []string) map[string]PR {
	if prs, err := ghPullRequests(); err == nil {
		return prs
	}
	return mergedBranches(branches)
}

func ghPullRequests() (map[string]PR, error) {
	out, err := command("gh", "pr", "list", "--state", "all", "--limit", "200",
		"--json", "number,state,isDraft,headRefName,url")
	if err != nil {
		return nil, err
	}
	var list []struct {
		Number      int
		State       string
		IsDraft     bool
		HeadRefName string
		URL         string
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, err
	}
	prs := make(map[string]PR, len(list))
	for _, p := range list {
		state := strings.ToLower(p.State)
		if state == "open" && p.IsDraft {
			state = "draft"
		}
		// A branch can carry several pull requests; keep the loudest one.
		if cur, ok := prs[p.HeadRefName]; ok && rank(cur.State) >= rank(state) {
			continue
		}
		prs[p.HeadRefName] = PR{Number: p.Number, State: state, URL: p.URL}
	}
	return prs, nil
}

// openPR is one entry of the pull request picker.
type openPR struct {
	Number      int
	Title       string
	HeadRefName string
	UpdatedAt   string
}

// Date is the day the pull request last moved, as a bare yyyy-mm-dd.
func (p openPR) Date() string {
	day, _, _ := strings.Cut(p.UpdatedAt, "T")
	return day
}

// openPRs lists the open pull requests, most recently updated first.
func openPRs() ([]openPR, error) {
	out, err := command("gh", "pr", "list", "--state", "open", "--limit", "200",
		"--json", "number,title,headRefName,updatedAt")
	if err != nil {
		return nil, errors.New(firstLine(err.Error()))
	}
	list := []openPR{}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return nil, err
	}
	// gh timestamps are RFC 3339 in UTC, so they sort as plain strings.
	sort.Slice(list, func(i, j int) bool { return list[i].UpdatedAt > list[j].UpdatedAt })
	return list, nil
}

// createFromPR checks the head branch of a pull request out in a new worktree.
// The branch is fetched from the pull request ref when it is not already local,
// which also works for pull requests opened from a fork.
func createFromPR(number string) (string, error) {
	number = strings.TrimPrefix(strings.TrimSpace(number), "#")
	if n, err := strconv.Atoi(number); err != nil || n <= 0 {
		return "", fmt.Errorf("invalid pull request number %q", number)
	}
	branch, err := prBranch(number)
	if err != nil {
		return "", err
	}
	if !branchExists(branch) {
		// ponytail: assumes the remote is "origin"; read it off gh if that bites.
		if _, err := git("fetch", "origin", "pull/"+number+"/head:"+branch); err != nil {
			return "", err
		}
	}
	return createWorktree(branch, "")
}

// prBranch is the head branch of a pull request, as gh reports it.
func prBranch(number string) (string, error) {
	out, err := command("gh", "pr", "view", number, "--json", "headRefName", "-q", ".headRefName")
	if err != nil {
		return "", fmt.Errorf("pull request #%s: %s", number, firstLine(err.Error()))
	}
	if out == "" {
		return "", fmt.Errorf("pull request #%s has no head branch", number)
	}
	return out, nil
}

func rank(state string) int {
	switch state {
	case "merged":
		return 3
	case "open", "draft":
		return 2
	}
	return 1
}

// mergedBranches reports the branches already contained in the default branch.
func mergedBranches(branches []string) map[string]PR {
	base := defaultBranch()
	if base == "" {
		return nil
	}
	prs := map[string]PR{}
	for _, b := range branches {
		if b == "" || b == strings.TrimPrefix(base, "origin/") {
			continue
		}
		if _, err := git("merge-base", "--is-ancestor", b, base); err == nil {
			prs[b] = PR{State: "merged"}
		}
	}
	return prs
}

// defaultBranch is the ref merges land on, preferring the remote's own answer.
func defaultBranch() string {
	if head, err := git("symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return head
	}
	for _, ref := range []string{"origin/main", "origin/master", "main", "master"} {
		if _, err := git("rev-parse", "--verify", "--quiet", ref); err == nil {
			return ref
		}
	}
	return ""
}
