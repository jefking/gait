package dashboard

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var commitCSVHeader = []string{
	"datetime",
	"date",
	"commit_hash",
	"github_author_handle",
	"github_author_display_name",
	"text",
	"files_changed",
	"lines_added",
	"lines_deleted",
	"lines_changed",
}

func ParseCommitCSV(reader io.Reader) (CommitStats, error) {
	stats := CommitStats{
		Contributors: make(map[string]ContributorMetrics),
		Monthly:      make(map[string]map[string]int),
	}
	records := csv.NewReader(reader)
	header, err := records.Read()
	if err != nil {
		return stats, fmt.Errorf("read commit CSV header: %w", err)
	}
	if len(header) != len(commitCSVHeader) {
		return stats, fmt.Errorf("commit CSV has %d columns; expected %d", len(header), len(commitCSVHeader))
	}
	for index := range header {
		if header[index] != commitCSVHeader[index] {
			return stats, fmt.Errorf("commit CSV column %d is %q; expected %q", index+1, header[index], commitCSVHeader[index])
		}
	}

	for {
		record, readErr := records.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return stats, fmt.Errorf("read commit CSV: %w", readErr)
		}
		if len(record) != len(commitCSVHeader) {
			return stats, fmt.Errorf("commit CSV row has %d columns; expected %d", len(record), len(commitCSVHeader))
		}

		committedAt, parseErr := time.Parse(time.RFC3339, record[0])
		if parseErr != nil {
			return stats, fmt.Errorf("parse commit datetime %q: %w", record[0], parseErr)
		}
		filesChanged, parseErr := parseCSVInteger("files_changed", record[6])
		if parseErr != nil {
			return stats, parseErr
		}
		linesAdded, parseErr := parseCSVInteger("lines_added", record[7])
		if parseErr != nil {
			return stats, parseErr
		}
		linesDeleted, parseErr := parseCSVInteger("lines_deleted", record[8])
		if parseErr != nil {
			return stats, parseErr
		}

		contributor := commitContributor(record[3], record[4])
		contributor.Commits++
		contributor.FilesChanged += filesChanged
		contributor.LinesAdded += linesAdded
		contributor.LinesDeleted += linesDeleted
		if committedAt.After(contributor.LastActivityAt) {
			contributor.LastActivityAt = committedAt
		}
		stats.Contributors[contributor.Key] = contributor

		month := committedAt.UTC().Format("2006-01")
		if stats.Monthly[month] == nil {
			stats.Monthly[month] = make(map[string]int)
		}
		stats.Monthly[month][contributor.Key]++
		stats.Commits++
		stats.FilesChanged += filesChanged
		stats.LinesAdded += linesAdded
		stats.LinesDeleted += linesDeleted
		if committedAt.After(stats.LastAt) {
			stats.LastAt = committedAt
		}
	}

	return stats, nil
}

func parseCSVInteger(name, value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("parse commit %s %q", name, value)
	}
	return parsed, nil
}

func commitContributor(handle, displayName string) ContributorMetrics {
	handle = strings.TrimSpace(handle)
	displayName = strings.TrimSpace(displayName)
	if handle != "" {
		login := strings.ToLower(handle)
		if displayName == "" {
			displayName = handle
		}
		return ContributorMetrics{
			Key:       "github:" + login,
			Login:     handle,
			Name:      displayName,
			AvatarURL: "https://github.com/" + url.PathEscape(handle) + ".png?size=80",
		}
	}
	if displayName == "" {
		displayName = "Unknown contributor"
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(displayName), " "))
	return ContributorMetrics{Key: "git:" + normalized, Name: displayName}
}

func BuildPullStats(pulls []PullRequest) PullStats {
	stats := PullStats{
		Contributors: make(map[string]ContributorMetrics),
		Monthly:      make(map[string]map[string]int),
	}
	for _, pull := range pulls {
		login := strings.TrimSpace(pull.Author.Login)
		if login == "" {
			login = "unknown"
		}
		key := "github:" + strings.ToLower(login)
		name := strings.TrimSpace(pull.Author.Name)
		if name == "" {
			name = pull.Author.Login
		}
		if name == "" {
			name = "Unknown contributor"
		}
		contributor := stats.Contributors[key]
		contributor.Key = key
		contributor.Login = pull.Author.Login
		contributor.Name = name
		contributor.AvatarURL = pull.Author.AvatarURL
		contributor.Type = pull.Author.Type
		contributor.PullRequests++
		if pull.CreatedAt.After(contributor.LastActivityAt) {
			contributor.LastActivityAt = pull.CreatedAt
		}
		stats.Contributors[key] = contributor

		month := pull.CreatedAt.UTC().Format("2006-01")
		if stats.Monthly[month] == nil {
			stats.Monthly[month] = make(map[string]int)
		}
		stats.Monthly[month][key]++
		stats.Totals.Opened++
		if pull.MergedAt != nil {
			stats.Totals.Merged++
		} else if pull.State == "open" {
			stats.Totals.Open++
		} else {
			stats.Totals.Closed++
		}
		if pull.CreatedAt.After(stats.LastAt) {
			stats.LastAt = pull.CreatedAt
		}
	}
	return stats
}

type ownerAccumulator struct {
	summary      OwnerSummary
	contributors map[string]struct{}
}

type contributorAccumulator struct {
	summary ContributorSummary
	repos   map[int64]struct{}
	owners  map[int64]struct{}
}

func BuildSnapshot(viewer Viewer, repositories []Repository, reports map[int64]RepositoryReport, repoStates map[int64]string) *Snapshot {
	snapshot := &Snapshot{GeneratedAt: time.Now().UTC(), Viewer: viewer}
	owners := make(map[int64]*ownerAccumulator)
	contributors := make(map[string]*contributorAccumulator)

	for _, repository := range repositories {
		report, exists := reports[repository.ID]
		if !exists {
			report = RepositoryReport{Repository: repository, SyncStatus: "pending"}
		} else {
			report.Repository = repository
		}
		if state := repoStates[repository.ID]; state != "" {
			report.SyncStatus = state
		}
		if report.SyncStatus == "" {
			report.SyncStatus = "cached"
		}

		repositorySummary := summarizeRepository(report)
		snapshot.Repositories = append(snapshot.Repositories, repositorySummary)

		owner := owners[repository.Owner.ID]
		if owner == nil {
			owner = &ownerAccumulator{
				summary:      OwnerSummary{Owner: repository.Owner},
				contributors: make(map[string]struct{}),
			}
			owners[repository.Owner.ID] = owner
		}
		owner.summary.Repositories++
		owner.summary.Commits += report.Commits.Commits
		owner.summary.LinesAdded += report.Commits.LinesAdded
		owner.summary.LinesDeleted += report.Commits.LinesDeleted
		if report.Pulls != nil {
			owner.summary.PullRequestsOpened += report.Pulls.Totals.Opened
		}

		for key, metrics := range report.Commits.Contributors {
			mergeContributor(contributors, metrics, repository, key)
			owner.contributors[key] = struct{}{}
		}
		if report.Pulls != nil {
			for key, metrics := range report.Pulls.Contributors {
				mergeContributor(contributors, metrics, repository, key)
				owner.contributors[key] = struct{}{}
			}
		}

		snapshot.Totals.Commits += report.Commits.Commits
		snapshot.Totals.FilesChanged += report.Commits.FilesChanged
		snapshot.Totals.LinesAdded += report.Commits.LinesAdded
		snapshot.Totals.LinesDeleted += report.Commits.LinesDeleted
		if report.Pulls == nil {
			snapshot.Totals.RepositoriesWithoutPRAccess++
		} else {
			snapshot.Totals.PullRequestsOpened += report.Pulls.Totals.Opened
			snapshot.Totals.PullRequestsOpen += report.Pulls.Totals.Open
			snapshot.Totals.PullRequestsClosed += report.Pulls.Totals.Closed
			snapshot.Totals.PullRequestsMerged += report.Pulls.Totals.Merged
		}
	}

	for _, owner := range owners {
		owner.summary.Contributors = len(owner.contributors)
		snapshot.Owners = append(snapshot.Owners, owner.summary)
	}
	for _, contributor := range contributors {
		contributor.summary.Repositories = len(contributor.repos)
		contributor.summary.Owners = len(contributor.owners)
		snapshot.Contributors = append(snapshot.Contributors, contributor.summary)
	}
	snapshot.Totals.Owners = len(snapshot.Owners)
	snapshot.Totals.Repositories = len(snapshot.Repositories)
	snapshot.Totals.Contributors = len(snapshot.Contributors)

	sort.Slice(snapshot.Owners, func(left, right int) bool {
		return strings.ToLower(snapshot.Owners[left].Owner.Login) < strings.ToLower(snapshot.Owners[right].Owner.Login)
	})
	sort.Slice(snapshot.Repositories, func(left, right int) bool {
		return strings.ToLower(snapshot.Repositories[left].FullName) < strings.ToLower(snapshot.Repositories[right].FullName)
	})
	sort.Slice(snapshot.Contributors, func(left, right int) bool {
		leftTotal := snapshot.Contributors[left].Commits + snapshot.Contributors[left].PullRequests
		rightTotal := snapshot.Contributors[right].Commits + snapshot.Contributors[right].PullRequests
		if leftTotal == rightTotal {
			return strings.ToLower(snapshot.Contributors[left].Name) < strings.ToLower(snapshot.Contributors[right].Name)
		}
		return leftTotal > rightTotal
	})
	return snapshot
}

func summarizeRepository(report RepositoryReport) RepositorySummary {
	repository := report.Repository
	contributorKeys := make(map[string]struct{})
	for key := range report.Commits.Contributors {
		contributorKeys[key] = struct{}{}
	}
	var pulls *PullRequestTotals
	lastActivity := report.Commits.LastAt
	if report.Pulls != nil {
		copyTotals := report.Pulls.Totals
		pulls = &copyTotals
		for key := range report.Pulls.Contributors {
			contributorKeys[key] = struct{}{}
		}
		if report.Pulls.LastAt.After(lastActivity) {
			lastActivity = report.Pulls.LastAt
		}
	}

	result := RepositorySummary{
		ID:            repository.ID,
		Name:          repository.Name,
		FullName:      repository.FullName,
		HTMLURL:       repository.HTMLURL,
		Description:   repository.Description,
		DefaultBranch: repository.DefaultBranch,
		Private:       repository.Private,
		Archived:      repository.Archived,
		Fork:          repository.Fork,
		Owner:         repository.Owner,
		Commits:       report.Commits.Commits,
		Contributors:  len(contributorKeys),
		FilesChanged:  report.Commits.FilesChanged,
		LinesAdded:    report.Commits.LinesAdded,
		LinesDeleted:  report.Commits.LinesDeleted,
		PullRequests:  pulls,
		SyncStatus:    report.SyncStatus,
		SyncMessage:   report.SyncMessage,
	}
	if !lastActivity.IsZero() {
		last := lastActivity.UTC()
		result.LastActivityAt = &last
	}
	return result
}

func mergeContributor(target map[string]*contributorAccumulator, metrics ContributorMetrics, repository Repository, key string) {
	contributor := target[key]
	if contributor == nil {
		contributor = &contributorAccumulator{
			summary: ContributorSummary{Key: key},
			repos:   make(map[int64]struct{}),
			owners:  make(map[int64]struct{}),
		}
		target[key] = contributor
	}
	contributor.summary.Login = preferNonEmpty(metrics.Login, contributor.summary.Login)
	contributor.summary.Name = preferNonEmpty(metrics.Name, contributor.summary.Name)
	contributor.summary.AvatarURL = preferNonEmpty(metrics.AvatarURL, contributor.summary.AvatarURL)
	contributor.summary.Type = preferNonEmpty(metrics.Type, contributor.summary.Type)
	contributor.summary.Commits += metrics.Commits
	contributor.summary.PullRequests += metrics.PullRequests
	contributor.summary.LinesAdded += metrics.LinesAdded
	contributor.summary.LinesDeleted += metrics.LinesDeleted
	if metrics.LastActivityAt.After(dereferenceTime(contributor.summary.LastActivityAt)) {
		last := metrics.LastActivityAt.UTC()
		contributor.summary.LastActivityAt = &last
	}
	contributor.repos[repository.ID] = struct{}{}
	contributor.owners[repository.Owner.ID] = struct{}{}
}

func preferNonEmpty(candidate, current string) string {
	if strings.TrimSpace(candidate) != "" {
		return candidate
	}
	return current
}

func dereferenceTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

type activityAccumulator struct {
	key       string
	label     string
	avatarURL string
	total     int
	monthly   map[string]int
}

func BuildActivity(reports map[int64]RepositoryReport, query ActivityQuery, now time.Time) ActivityResponse {
	result := ActivityResponse{Group: query.Group, Metric: query.Metric, Series: []ActivitySeries{}}
	series := make(map[string]*activityAccumulator)
	for _, report := range reports {
		if query.RepositoryID != 0 && report.Repository.ID != query.RepositoryID {
			continue
		}
		if query.OwnerID != 0 && report.Repository.Owner.ID != query.OwnerID {
			continue
		}
		if query.Metric == ActivityPullRequests && report.Pulls == nil {
			continue
		}

		if query.Group == ActivityByOwner {
			key := strconv.FormatInt(report.Repository.Owner.ID, 10)
			entry := ensureActivity(series, key, report.Repository.Owner.Login, report.Repository.Owner.AvatarURL)
			monthly := report.Commits.Monthly
			if query.Metric == ActivityPullRequests {
				monthly = report.Pulls.Monthly
			}
			for month, contributors := range monthly {
				for _, count := range contributors {
					entry.monthly[month] += count
					entry.total += count
				}
			}
			continue
		}

		monthly := report.Commits.Monthly
		metadata := report.Commits.Contributors
		if query.Metric == ActivityPullRequests {
			monthly = report.Pulls.Monthly
			metadata = report.Pulls.Contributors
		}
		for month, contributors := range monthly {
			for key, count := range contributors {
				person := metadata[key]
				label := person.Name
				if label == "" {
					label = person.Login
				}
				entry := ensureActivity(series, key, label, person.AvatarURL)
				entry.monthly[month] += count
				entry.total += count
			}
		}
	}

	ordered := make([]*activityAccumulator, 0, len(series))
	for _, entry := range series {
		if entry.total > 0 {
			ordered = append(ordered, entry)
		}
	}
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].total == ordered[right].total {
			return strings.ToLower(ordered[left].label) < strings.ToLower(ordered[right].label)
		}
		return ordered[left].total > ordered[right].total
	})
	if len(ordered) > 8 {
		other := &activityAccumulator{key: "other", label: "Other", monthly: make(map[string]int)}
		for _, entry := range ordered[8:] {
			other.total += entry.total
			for month, count := range entry.monthly {
				other.monthly[month] += count
			}
		}
		ordered = append(ordered[:8], other)
	}
	if len(ordered) == 0 {
		return result
	}

	firstMonth, lastMonth := activityRange(ordered, now.UTC())
	result.From = firstMonth.Format("2006-01")
	result.To = lastMonth.Format("2006-01")
	for _, entry := range ordered {
		activitySeries := ActivitySeries{
			Key:       entry.key,
			Label:     entry.label,
			AvatarURL: entry.avatarURL,
			Total:     entry.total,
		}
		for month := firstMonth; !month.After(lastMonth); month = month.AddDate(0, 1, 0) {
			monthKey := month.Format("2006-01")
			activitySeries.Points = append(activitySeries.Points, ActivityPoint{Month: monthKey, Value: entry.monthly[monthKey]})
		}
		result.Series = append(result.Series, activitySeries)
	}
	return result
}

func ensureActivity(series map[string]*activityAccumulator, key, label, avatarURL string) *activityAccumulator {
	entry := series[key]
	if entry == nil {
		entry = &activityAccumulator{key: key, label: label, avatarURL: avatarURL, monthly: make(map[string]int)}
		series[key] = entry
	}
	if entry.label == "" {
		entry.label = label
	}
	if entry.avatarURL == "" {
		entry.avatarURL = avatarURL
	}
	return entry
}

func activityRange(series []*activityAccumulator, now time.Time) (time.Time, time.Time) {
	var first time.Time
	last := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for _, entry := range series {
		for month := range entry.monthly {
			parsed, err := time.Parse("2006-01", month)
			if err != nil {
				continue
			}
			if first.IsZero() || parsed.Before(first) {
				first = parsed
			}
			if parsed.After(last) {
				last = parsed
			}
		}
	}
	if first.IsZero() {
		first = last
	}
	return first, last
}
