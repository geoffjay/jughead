package data

// This file defines the domain model for the quux code-review demo. The types
// mirror the subset of GitHub REST objects needed to render the review UI:
// repositories, pull requests assigned to the viewer, users (collaborators,
// contributors, and AI review agents), and structured file diffs.
//
// The shapes are deliberately simple so that a future GitHub API client can
// populate them without reshaping the templates. Nothing here performs I/O.

// Repository is a GitHub repository the viewer has assigned review work in.
type Repository struct {
	Owner string // login of the repo owner, e.g. "geoffjay"
	Name  string // repo name, e.g. "jughead"
	// FullName is "owner/name"; derived in fixtures but stored for convenience.
	FullName string
	Icon     string // optional emoji/short label for the sidebar grouping
}

// PRStatus is the review state of a pull request.
type PRStatus string

const (
	PRStatusOpen       PRStatus = "open"
	PRStatusDraft      PRStatus = "draft"
	PRStatusReviewing  PRStatus = "reviewing"
	PRStatusApproved   PRStatus = "approved"
	PRStatusChangesReq PRStatus = "changes-requested"
	PRStatusMerged     PRStatus = "merged"
	PRStatusClosed     PRStatus = "closed"
)

// PullRequest is a PR the viewer has been asked to review.
type PullRequest struct {
	Number     int        // PR number, e.g. 42
	Title      string     // PR title
	Author     User       // who opened the PR
	Status     PRStatus   // current review state
	HeadBranch string     // source branch, e.g. "feature/login"
	BaseBranch string     // target branch, e.g. "main"
	UpdatedAt  string     // human-readable relative time, e.g. "2h ago"
	Repo       Repository // repo the PR belongs to
	// Files is the changed files with hunks; the first file is shown by
	// default in the center diff pane.
	Files []FileDiff
}

// UserRole classifies a user for the right-hand sidebar grouping.
type UserRole string

const (
	RoleCollaborator UserRole = "collaborator"
	RoleContributor  UserRole = "contributor"
	RoleAgent        UserRole = "agent"
)

// User is a person or bot involved in the repository/PR. AvatarURL is left
// empty for fixture users so the UI falls back to avatar-placeholder (keeps
// the demo within the page's strict img-src CSP).
type User struct {
	Login     string   // GitHub login
	Name      string   // display name
	AvatarURL string   // optional avatar image URL
	Role      UserRole // collaborator | contributor | agent
	// Contributions counts commits/PRs touching the relevant file/area; used
	// by the "Contributors" section. Zero for collaborators/agents.
	Contributions int
	// Active reports whether the user is currently online/looking at the PR;
	// used by the "Active collaborators" section.
	Active bool
	// Enabled (agents only) indicates whether the AI agent is part of the
	// review process. Display-only for now; functionality is deferred.
	Enabled bool
}

// LineKind classifies a single line within a diff hunk.
type LineKind string

const (
	LineAdded   LineKind = "added"
	LineRemoved LineKind = "removed"
	LineContext LineKind = "context"
)

// DiffLine is a single line in a hunk. OldLineNo is the line number in the
// pre-change file (0 for added lines); NewLineNo is the line number in the
// post-change file (0 for removed lines).
type DiffLine struct {
	Kind      LineKind
	OldLineNo int
	NewLineNo int
	Text      string
}

// Hunk is a contiguous run of changed lines with surrounding context.
type Hunk struct {
	OldStart int        // starting line in the pre-change file
	NewStart int        // starting line in the post-change file
	Lines    []DiffLine // ordered lines
}

// FileDiff is one changed file in a PR.
type FileDiff struct {
	Path    string // repo-relative path, e.g. "handlers.go"
	OldPath string // pre-rename path; empty when unchanged
	Added   int    // number of added lines
	Removed int    // number of removed lines
	Hunks   []Hunk // hunks for this file
}

// ReviewState is the full payload the quux page renders: the list of assigned
// PRs grouped by repo (left pane), the currently selected PR (center pane),
// and the people/agents relevant to the selected PR (right pane).
type ReviewState struct {
	PRs           []PullRequest
	SelectedPR    *PullRequest
	Collaborators []User
	Contributors  []User
	Agents        []User
}
