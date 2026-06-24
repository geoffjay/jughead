package data

// fixtures.go holds hardcoded sample data for the quux code-review demo. The
// shapes match GitHub REST so a real client can replace these later without
// touching templates. No I/O happens here.

var repos = []Repository{
	{Owner: "geoffjay", Name: "jughead", FullName: "geoffjay/jughead", Icon: "J"},
	{Owner: "geoffjay", Name: "2d", FullName: "geoffjay/2d", Icon: "2"},
	{Owner: "geoffjay", Name: "personal-site", FullName: "geoffjay/personal-site", Icon: "P"},
}

func repoByName(full string) Repository {
	for _, r := range repos {
		if r.FullName == full {
			return r
		}
	}
	return repos[0]
}

// goFileDiff builds a realistic multi-hunk Go diff used by the primary demo PR.
func goFileDiff() []FileDiff {
	return []FileDiff{
		{
			Path:    "handlers.go",
			Added:   9,
			Removed: 3,
			Hunks: []Hunk{
				{
					OldStart: 42,
					NewStart: 42,
					Lines: []DiffLine{
						{Kind: LineContext, OldLineNo: 42, NewLineNo: 42, Text: "func siteViewHandler(c *gin.Context) {"},
						{Kind: LineContext, OldLineNo: 43, NewLineNo: 43, Text: "\tsite := sites.GetSiteManager().GetSite(strings.TrimRight(c.Request.URL.Path, \"/\"))"},
						{Kind: LineRemoved, OldLineNo: 44, NewLineNo: 0, Text: "\tif site == nil {"},
						{Kind: LineRemoved, OldLineNo: 45, NewLineNo: 0, Text: "\t\tc.AbortWithStatus(http.StatusNotFound)"},
						{Kind: LineRemoved, OldLineNo: 46, NewLineNo: 0, Text: "\t\treturn"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 44, Text: "\tif site == nil || !site.Published {"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 45, Text: "\t\tc.AbortWithStatus(http.StatusNotFound)"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 46, Text: "\t\treturn"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 47, Text: "\t}"},
						{Kind: LineContext, OldLineNo: 47, NewLineNo: 48, Text: ""},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 49, Text: "\tmeta := pages.MetaTags(\"quux\", fmt.Sprintf(\"Welcome to %s\", site.Url))"},
						{Kind: LineContext, OldLineNo: 48, NewLineNo: 50, Text: "}"},
					},
				},
			},
		},
		{
			Path:    "server.go",
			Added:   4,
			Removed: 1,
			Hunks: []Hunk{
				{
					OldStart: 70,
					NewStart: 70,
					Lines: []DiffLine{
						{Kind: LineContext, OldLineNo: 70, NewLineNo: 70, Text: "\trouter.Use(middleware.ReverseProxy(map[string]string{"},
						{Kind: LineContext, OldLineNo: 71, NewLineNo: 71, Text: "\t\t\"quux.geoffjay.com\": fmt.Sprintf(\"http://localhost:%d/sites/quux.geoffjay.com\", port),"},
						{Kind: LineRemoved, OldLineNo: 72, NewLineNo: 0, Text: "\t}))"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 72, Text: "\t\t\"admin.geoffjay.com\": fmt.Sprintf(\"http://localhost:%d/admin\", port),"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 73, Text: "\t}))"},
						{Kind: LineContext, OldLineNo: 73, NewLineNo: 74, Text: ""},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 75, Text: "\trouter.GET(\"/health\", healthHandler)"},
						{Kind: LineContext, OldLineNo: 74, NewLineNo: 76, Text: "}"},
					},
				},
			},
		},
	}
}

// cssFileDiff builds a small CSS diff for the secondary demo PR.
func cssFileDiff() []FileDiff {
	return []FileDiff{
		{
			Path:    "assets/styles.css",
			Added:   5,
			Removed: 2,
			Hunks: []Hunk{
				{
					OldStart: 12,
					NewStart: 12,
					Lines: []DiffLine{
						{Kind: LineContext, OldLineNo: 12, NewLineNo: 12, Text: ".diff-line {"},
						{Kind: LineRemoved, OldLineNo: 13, NewLineNo: 0, Text: "  padding: 0 0.5rem;"},
						{Kind: LineRemoved, OldLineNo: 14, NewLineNo: 0, Text: "  font-family: monospace;"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 13, Text: "  padding: 0 0.75rem;"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 14, Text: "  font-family: ui-monospace, monospace;"},
						{Kind: LineAdded, OldLineNo: 0, NewLineNo: 15, Text: "  line-height: 1.5;"},
						{Kind: LineContext, OldLineNo: 15, NewLineNo: 16, Text: "}"},
					},
				},
			},
		},
	}
}

var allPRs = []PullRequest{
	{
		Number:     42,
		Title:      "Gate unpublished sites behind the published flag",
		Author:     User{Login: "alice", Name: "Alice Park", Role: RoleCollaborator, Active: true},
		Status:     PRStatusReviewing,
		HeadBranch: "fix/unpublished-gate",
		BaseBranch: "main",
		UpdatedAt:  "2h ago",
		Repo:       repoByName("geoffjay/jughead"),
		Files:      goFileDiff(),
	},
	{
		Number:     39,
		Title:      "Add /health endpoint for fly.io checks",
		Author:     User{Login: "bob", Name: "Bob Lin", Role: RoleCollaborator},
		Status:     PRStatusChangesReq,
		HeadBranch: "feat/health",
		BaseBranch: "main",
		UpdatedAt:  "1d ago",
		Repo:       repoByName("geoffjay/jughead"),
		Files:      goFileDiff()[1:],
	},
	{
		Number:     17,
		Title:      "Tighten diff line spacing for dark themes",
		Author:     User{Login: "cara", Name: "Cara Ng", Role: RoleCollaborator, Active: true},
		Status:     PRStatusOpen,
		HeadBranch: "style/diff-spacing",
		BaseBranch: "main",
		UpdatedAt:  "3h ago",
		Repo:       repoByName("geoffjay/2d"),
		Files:      cssFileDiff(),
	},
	{
		Number:     8,
		Title:      "Draft: avatar group component for review pane",
		Author:     User{Login: "dan", Name: "Dan Reyes", Role: RoleCollaborator},
		Status:     PRStatusDraft,
		HeadBranch: "feat/avatar-group",
		BaseBranch: "main",
		UpdatedAt:  "5h ago",
		Repo:       repoByName("geoffjay/personal-site"),
		Files:      cssFileDiff(),
	},
}

// collaboratorsFor returns the active reviewers for a PR. In the fixtures we
// keep a stable set per repo so the right pane feels consistent.
func collaboratorsFor(pr *PullRequest) []User {
	base := []User{
		{Login: "geoffjay", Name: "Geoff Jay", Role: RoleCollaborator, Active: true},
		{Login: pr.Author.Login, Name: pr.Author.Name, Role: RoleCollaborator, Active: pr.Author.Active},
		{Login: "eve", Name: "Eve Sato", Role: RoleCollaborator, Active: false},
	}
	return base
}

// contributorsFor returns people who have touched the file/area in the past.
func contributorsFor(_ *PullRequest) []User {
	return []User{
		{Login: "alice", Name: "Alice Park", Role: RoleContributor, Contributions: 34},
		{Login: "bob", Name: "Bob Lin", Role: RoleContributor, Contributions: 21},
		{Login: "cara", Name: "Cara Ng", Role: RoleContributor, Contributions: 12},
		{Login: "eve", Name: "Eve Sato", Role: RoleContributor, Contributions: 7},
	}
}

// agentsFor returns the AI review agents attached to a PR. They are display-
// only for now; Enabled hints whether the agent is part of the flow.
func agentsFor(_ *PullRequest) []User {
	return []User{
		{Login: "claude-reviewer", Name: "Claude Reviewer", Role: RoleAgent, Enabled: true},
		{Login: "copilot", Name: "Copilot Reviewer", Role: RoleAgent, Enabled: false},
		{Login: "codeql", Name: "CodeQL Security", Role: RoleAgent, Enabled: true},
	}
}
