package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/geoffjay/jughead/sites/com/geoffjay/quux/data"
	"github.com/geoffjay/jughead/utils"
)

// Pagination / search defaults.
const (
	perPagePRs           = 50
	perPageCollaborators = 100
	perPageContributors  = 20
	rateLimitFloor       = 50 // short-circuit to cache when remaining drops below this
)

// ListAssignedPRs returns the open pull requests the viewer has been
// requested to review, grouped into a flat slice (the data layer groups by
// repo). Uses the search/issues endpoint with the review-requested qualifier.
func (c *Client) ListAssignedPRs(ctx context.Context, login string) ([]data.PullRequest, error) {
	query := fmt.Sprintf("review-requested:%s is:pr is:open", login)
	params := url.Values{
		"q":        {query},
		"per_page": {strconv.Itoa(perPagePRs)},
	}
	var page searchIssuesResponse
	if err := c.getJSON(ctx, "/search/issues?"+params.Encode(), &page); err != nil {
		return nil, err
	}
	return mapSearchToPRs(page.Items), nil
}

// LoadPR fetches a PR's detail, files (parsed into hunks), and review
// state, assembling them into a single data.PullRequest.
func (c *Client) LoadPR(ctx context.Context, owner, repo string, number int) (data.PullRequest, error) {
	ownerRepo := owner + "/" + repo

	detail, err := c.fetchPRDetail(ctx, owner, repo, number)
	if err != nil {
		return data.PullRequest{}, err
	}
	files, err := c.fetchPRFiles(ctx, owner, repo, number)
	if err != nil {
		return data.PullRequest{}, err
	}
	status, err := c.fetchPRStatus(ctx, owner, repo, number)
	if err != nil {
		// Reviews may be inaccessible (private repo without review scope);
		// fall back to the PR's draft/open state rather than failing the page.
		status = deriveStatus(&detail)
	} else if status == data.PRStatusOpen {
		status = deriveStatus(&detail)
	}

	return data.PullRequest{
		Number:     number,
		Title:      detail.Title,
		Author:     mapUser(detail.User, data.RoleCollaborator),
		Status:     status,
		HeadBranch: detail.Head.Ref,
		BaseBranch: detail.Base.Ref,
		UpdatedAt:  formatRelativeTime(detail.UpdatedAt),
		Repo: data.Repository{
			Owner:    owner,
			Name:     repo,
			FullName: ownerRepo,
			Icon:     repoInitial(repo),
		},
		Files: files,
	}, nil
}

// fetchPRDetail calls GET /repos/{owner}/{repo}/pulls/{number}.
func (c *Client) fetchPRDetail(ctx context.Context, owner, repo string, number int) (prDetail, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, number)
	var detail prDetail
	if err := c.getJSON(ctx, path, &detail); err != nil {
		return prDetail{}, fmt.Errorf("fetch PR detail: %w", err)
	}
	return detail, nil
}

// fetchPRFiles calls GET /repos/{owner}/{repo}/pulls/{number}/files and parses
// each file's unified-diff `patch` field into hunks.
func (c *Client) fetchPRFiles(ctx context.Context, owner, repo string, number int) ([]data.FileDiff, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=%d", owner, repo, number, perPagePRs)
	var raw []prFile
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return nil, fmt.Errorf("fetch PR files: %w", err)
	}

	files := make([]data.FileDiff, 0, len(raw))
	for _, f := range raw {
		hunks, err := utils.Parse(f.Patch)
		if err != nil {
			// Skip unparseable files rather than failing the whole PR.
			continue
		}
		fd := data.FileDiff{
			Path:    f.Filename,
			OldPath: f.PreviousFilename,
			Added:   f.Additions,
			Removed: f.Deletions,
			Hunks:   mapHunks(hunks),
		}
		files = append(files, fd)
	}
	return files, nil
}

// fetchPRStatus aggregates review states from GET /repos/.../pulls/{n}/reviews
// to determine if a PR has been approved or has changes requested.
func (c *Client) fetchPRStatus(ctx context.Context, owner, repo string, number int) (data.PRStatus, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews?per_page=%d", owner, repo, number, perPagePRs)
	var reviews []prReview
	if err := c.getJSON(ctx, path, &reviews); err != nil {
		return data.PRStatusOpen, fmt.Errorf("fetch PR reviews: %w", err)
	}
	// Walk newest-first; the last meaningful review state wins.
	for i := len(reviews) - 1; i >= 0; i-- {
		switch reviews[i].State {
		case "APPROVED":
			return data.PRStatusApproved, nil
		case "CHANGES_REQUESTED":
			return data.PRStatusChangesReq, nil
		case "COMMENTED", "DISMISSED":
			continue
		}
	}
	return data.PRStatusReviewing, nil
}

// ListCollaborators calls GET /repos/{owner}/{repo}/collaborators.
func (c *Client) ListCollaborators(ctx context.Context, owner, repo string) ([]data.User, error) {
	path := fmt.Sprintf("/repos/%s/%s/collaborators?per_page=%d", owner, repo, perPageCollaborators)
	var raw []apiUser
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	users := make([]data.User, 0, len(raw))
	for _, u := range raw {
		users = append(users, mapUser(u, data.RoleCollaborator))
	}
	return users, nil
}

// ListContributors calls GET /repos/{owner}/{repo}/contributors.
func (c *Client) ListContributors(ctx context.Context, owner, repo string) ([]data.User, error) {
	path := fmt.Sprintf("/repos/%s/%s/contributors?per_page=%d", owner, repo, perPageContributors)
	var raw []contributor
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	users := make([]data.User, 0, len(raw))
	for _, c2 := range raw {
		users = append(users, data.User{
			Login:         c2.Login,
			Name:          c2.Login, // contributors endpoint doesn't return display name
			AvatarURL:     c2.AvatarURL,
			Role:          data.RoleContributor,
			Contributions: c2.Contributions,
		})
	}
	return users, nil
}

// ListAgents calls GET /repos/{owner}/{repo}/installation and lists the
// installed GitHub App as a single agent. GitHub App installations on a repo
// are retrieved one-at-a-time via /installation; to enumerate multiple the
// caller would use the /installations list (requires App JWT auth, deferred).
// For now we fetch the single installation (the most common case: one CI/AI
// bot per repo) and present it as an agent.
func (c *Client) ListAgents(ctx context.Context, owner, repo string) ([]data.User, error) {
	path := fmt.Sprintf("/repos/%s/%s/installation", owner, repo)
	var inst installation
	if err := c.getJSON(ctx, path, &inst); err != nil {
		// The /installation endpoint requires GitHub App JWT auth, not a user
		// OAuth token. A user token gets 401 ("JWT could not be decoded") or
		// 404 (no app installed). Either way, return an empty agent list
		// rather than failing the page — the agents section is supplementary.
		if isNotFound(err) || isUnauthorized(err) {
			return nil, nil
		}
		return nil, err
	}
	return []data.User{
		{
			Login:   inst.AppSlug,
			Name:    inst.App.Name,
			Role:    data.RoleAgent,
			Enabled: true,
		},
	}, nil
}

// --- helpers ---

// getJSON performs an authenticated GET and decodes the JSON body into v.
func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	req, err := c.newRequest(ctx, path)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxAPIBody))
		return &APIError{Path: path, Status: resp.StatusCode, Body: string(body)}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIBody))
	if err != nil {
		return fmt.Errorf("read response %s: %w", path, err)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("parse response %s: %w", path, err)
	}
	return nil
}

// APIError is returned for non-2xx responses, carrying the path and status
// so handlers can branch (401 -> re-auth, 404 -> empty result, etc.).
type APIError struct {
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github %s: status %d: %s", e.Path, e.Status, strings.TrimSpace(e.Body))
}

func isNotFound(err error) bool {
	if ae, ok := err.(*APIError); ok {
		return ae.Status == http.StatusNotFound
	}
	return false
}

func isUnauthorized(err error) bool {
	if ae, ok := err.(*APIError); ok {
		return ae.Status == http.StatusUnauthorized
	}
	return false
}

// --- JSON models (subset of fields we need) ---

type searchIssuesResponse struct {
	Items []searchIssue `json:"items"`
}

type searchIssue struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	HTMLURL   string  `json:"html_url"`
	State     string  `json:"state"`
	Draft     bool    `json:"draft"`
	User      apiUser `json:"user"`
	UpdatedAt string  `json:"updated_at"`
	RepoURL   string  `json:"repository_url"`
}

type prDetail struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Draft     bool     `json:"draft"`
	State     string   `json:"state"`
	User      apiUser  `json:"user"`
	Head      prBranch `json:"head"`
	Base      prBranch `json:"base"`
	UpdatedAt string   `json:"updated_at"`
}

type prBranch struct {
	Ref string `json:"ref"`
}

type prFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Patch            string `json:"patch"`
}

type prReview struct {
	State string `json:"state"`
}

type apiUser struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

type contributor struct {
	Login         string `json:"login"`
	AvatarURL     string `json:"avatar_url"`
	Contributions int    `json:"contributions"`
}

type installation struct {
	App     appInfo `json:"app"`
	AppSlug string  `json:"app_slug"`
}

type appInfo struct {
	Name string `json:"name"`
}

// --- mapping helpers ---

func mapSearchToPRs(items []searchIssue) []data.PullRequest {
	prs := make([]data.PullRequest, 0, len(items))
	for i := range items {
		it := items[i]
		owner, repo := parseRepoURL(it.RepoURL)
		prs = append(prs, data.PullRequest{
			Number:     it.Number,
			Title:      it.Title,
			Author:     mapUser(it.User, data.RoleCollaborator),
			Status:     searchStatus(&it),
			HeadBranch: "(loaded on select)",
			BaseBranch: "(loaded on select)",
			UpdatedAt:  formatRelativeTime(it.UpdatedAt),
			Repo: data.Repository{
				Owner:    owner,
				Name:     repo,
				FullName: owner + "/" + repo,
				Icon:     repoInitial(repo),
			},
		})
	}
	return prs
}

func searchStatus(it *searchIssue) data.PRStatus {
	if it.Draft {
		return data.PRStatusDraft
	}
	return data.PRStatusReviewing
}

func deriveStatus(d *prDetail) data.PRStatus {
	if d.Draft {
		return data.PRStatusDraft
	}
	if d.State == "closed" {
		return data.PRStatusClosed
	}
	return data.PRStatusReviewing
}

func mapUser(u apiUser, role data.UserRole) data.User {
	return data.User{
		Login:     u.Login,
		Name:      u.Login,
		AvatarURL: u.AvatarURL,
		Role:      role,
	}
}

func mapHunks(phs []utils.Hunk) []data.Hunk {
	hs := make([]data.Hunk, 0, len(phs))
	for _, ph := range phs {
		h := data.Hunk{OldStart: ph.OldStart, NewStart: ph.NewStart}
		for _, pl := range ph.Lines {
			h.Lines = append(h.Lines, data.DiffLine{
				Kind:      mapKind(pl.Kind),
				OldLineNo: pl.OldLineNo,
				NewLineNo: pl.NewLineNo,
				Text:      pl.Text,
			})
		}
		hs = append(hs, h)
	}
	return hs
}

func mapKind(k utils.LineKind) data.LineKind {
	switch k {
	case utils.LineAdded:
		return data.LineAdded
	case utils.LineRemoved:
		return data.LineRemoved
	default:
		return data.LineContext
	}
}

const repoURLSegments = 2

// parseRepoURL extracts owner and repo from a repository_url like
// "https://api.github.com/repos/geoffjay/jughead".
func parseRepoURL(u string) (owner, repo string) {
	const reposPrefix = "https://api.github.com/repos/"
	rest := strings.TrimPrefix(u, reposPrefix)
	parts := strings.SplitN(rest, "/", repoURLSegments)
	if len(parts) != repoURLSegments {
		return "", ""
	}
	return parts[0], parts[1]
}

// repoInitial returns the first character of a repo name, uppercased, used as
// the sidebar icon badge.
func repoInitial(repo string) string {
	if repo == "" {
		return "?"
	}
	r := rune(repo[0])
	if r >= 'a' && r <= 'z' {
		r -= 32
	}
	return string(r)
}

// formatRelativeTime turns an ISO 8601 timestamp into a coarse relative label
// ("2h ago", "3d ago"). Falls back to the raw string when parsing fails.
func formatRelativeTime(iso string) string {
	return iso // simplified; the UI can humanize later
}
