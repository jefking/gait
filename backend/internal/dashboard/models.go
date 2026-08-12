package dashboard

import "time"

type SyncState string

const (
	SyncIdle                 SyncState = "idle"
	SyncDiscovering          SyncState = "discovering"
	SyncRunning              SyncState = "syncing"
	SyncWaitingRateLimit     SyncState = "waiting_rate_limit"
	SyncComplete             SyncState = "complete"
	SyncCompleteWithWarnings SyncState = "complete_with_warnings"
	SyncFailed               SyncState = "failed"
)

type SyncStatus struct {
	ID                 string     `json:"id,omitempty"`
	State              SyncState  `json:"state"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
	RateLimitResetAt   *time.Time `json:"rate_limit_reset_at,omitempty"`
	TotalRepositories  int        `json:"total_repositories"`
	CompletedRepos     int        `json:"completed_repositories"`
	FailedRepositories int        `json:"failed_repositories"`
	CurrentRepos       []string   `json:"current_repositories,omitempty"`
	Message            string     `json:"message,omitempty"`
	Warnings           []string   `json:"warnings,omitempty"`
}

func (status SyncStatus) Active() bool {
	return status.State == SyncDiscovering || status.State == SyncRunning || status.State == SyncWaitingRateLimit
}

type DashboardResponse struct {
	Snapshot *Snapshot  `json:"snapshot"`
	Sync     SyncStatus `json:"sync"`
}

type Snapshot struct {
	GeneratedAt  time.Time            `json:"generated_at"`
	Viewer       Viewer               `json:"viewer"`
	Totals       DashboardTotals      `json:"totals"`
	Owners       []OwnerSummary       `json:"owners"`
	Contributors []ContributorSummary `json:"contributors"`
	Repositories []RepositorySummary  `json:"repositories"`
}

type Viewer struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

type DashboardTotals struct {
	Owners                      int `json:"owners"`
	Repositories                int `json:"repositories"`
	Contributors                int `json:"contributors"`
	Commits                     int `json:"commits"`
	FilesChanged                int `json:"files_changed"`
	LinesAdded                  int `json:"lines_added"`
	LinesDeleted                int `json:"lines_deleted"`
	PullRequestsOpened          int `json:"pull_requests_opened"`
	PullRequestsOpen            int `json:"pull_requests_open"`
	PullRequestsClosed          int `json:"pull_requests_closed"`
	PullRequestsMerged          int `json:"pull_requests_merged"`
	RepositoriesWithoutPRAccess int `json:"repositories_without_pr_access"`
}

type OwnerIdentity struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Type      string `json:"type"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
}

type OwnerSummary struct {
	Owner              OwnerIdentity `json:"owner"`
	Repositories       int           `json:"repositories"`
	Contributors       int           `json:"contributors"`
	Commits            int           `json:"commits"`
	LinesAdded         int           `json:"lines_added"`
	LinesDeleted       int           `json:"lines_deleted"`
	PullRequestsOpened int           `json:"pull_requests_opened"`
}

type ContributorSummary struct {
	Key            string     `json:"key"`
	Login          string     `json:"login,omitempty"`
	Name           string     `json:"name"`
	AvatarURL      string     `json:"avatar_url,omitempty"`
	Type           string     `json:"type,omitempty"`
	Commits        int        `json:"commits"`
	PullRequests   int        `json:"pull_requests"`
	Repositories   int        `json:"repositories"`
	Owners         int        `json:"owners"`
	LinesAdded     int        `json:"lines_added"`
	LinesDeleted   int        `json:"lines_deleted"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
}

type PullRequestTotals struct {
	Opened int `json:"opened"`
	Open   int `json:"open"`
	Closed int `json:"closed"`
	Merged int `json:"merged"`
}

type RepositorySummary struct {
	ID             int64              `json:"id"`
	Name           string             `json:"name"`
	FullName       string             `json:"full_name"`
	HTMLURL        string             `json:"html_url"`
	Description    string             `json:"description,omitempty"`
	DefaultBranch  string             `json:"default_branch"`
	Private        bool               `json:"private"`
	Archived       bool               `json:"archived"`
	Fork           bool               `json:"fork"`
	Owner          OwnerIdentity      `json:"owner"`
	Commits        int                `json:"commits"`
	Contributors   int                `json:"contributors"`
	FilesChanged   int                `json:"files_changed"`
	LinesAdded     int                `json:"lines_added"`
	LinesDeleted   int                `json:"lines_deleted"`
	PullRequests   *PullRequestTotals `json:"pull_requests"`
	LastActivityAt *time.Time         `json:"last_activity_at,omitempty"`
	SyncStatus     string             `json:"sync_status"`
	SyncMessage    string             `json:"sync_message,omitempty"`
}

type ActivityGroup string

const (
	ActivityByOwner       ActivityGroup = "owner"
	ActivityByContributor ActivityGroup = "contributor"
)

type ActivityMetric string

const (
	ActivityCommits      ActivityMetric = "commits"
	ActivityPullRequests ActivityMetric = "pull_requests"
)

type ActivityGranularity string

const (
	ActivityByDay   ActivityGranularity = "day"
	ActivityByWeek  ActivityGranularity = "week"
	ActivityByMonth ActivityGranularity = "month"
)

type ActivityQuery struct {
	Group        ActivityGroup
	Metric       ActivityMetric
	OwnerID      int64
	RepositoryID int64
	From         *time.Time
	To           *time.Time
}

type ActivityResponse struct {
	Group         ActivityGroup       `json:"group_by"`
	Metric        ActivityMetric      `json:"metric"`
	Granularity   ActivityGranularity `json:"granularity,omitempty"`
	AvailableFrom string              `json:"available_from,omitempty"`
	AvailableTo   string              `json:"available_to,omitempty"`
	From          string              `json:"from,omitempty"`
	To            string              `json:"to,omitempty"`
	Series        []ActivitySeries    `json:"series"`
}

type ActivitySeries struct {
	Key       string          `json:"key"`
	Label     string          `json:"label"`
	AvatarURL string          `json:"avatar_url,omitempty"`
	Total     int             `json:"total"`
	Points    []ActivityPoint `json:"points"`
}

type ActivityPoint struct {
	Date  string `json:"date"`
	Value int    `json:"value"`
}

type Repository struct {
	ID            int64
	Name          string
	FullName      string
	CloneURL      string
	HTMLURL       string
	Description   string
	DefaultBranch string
	Private       bool
	Archived      bool
	Fork          bool
	Owner         OwnerIdentity
}

type PullRequest struct {
	Number    int64      `json:"number"`
	State     string     `json:"state"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Author    Person     `json:"author"`
}

type Person struct {
	Login     string `json:"login"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Type      string `json:"type,omitempty"`
}

type PullCache struct {
	Checkpoint   time.Time     `json:"checkpoint"`
	PullRequests []PullRequest `json:"pull_requests"`
}

type ContributorMetrics struct {
	Key            string
	Login          string
	Name           string
	AvatarURL      string
	Type           string
	Commits        int
	PullRequests   int
	FilesChanged   int
	LinesAdded     int
	LinesDeleted   int
	LastActivityAt time.Time
}

type CommitStats struct {
	Commits      int
	FilesChanged int
	LinesAdded   int
	LinesDeleted int
	LastAt       time.Time
	Contributors map[string]ContributorMetrics
	Daily        map[string]map[string]int
}

type PullStats struct {
	Totals       PullRequestTotals
	LastAt       time.Time
	Contributors map[string]ContributorMetrics
	Daily        map[string]map[string]int
}

type RepositoryReport struct {
	Repository  Repository
	Commits     CommitStats
	Pulls       *PullStats
	SyncStatus  string
	SyncMessage string
}
