package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestParseCommitCSVAndContributorIdentity(t *testing.T) {
	csvData := strings.Join([]string{
		strings.Join(commitCSVHeader, ","),
		"2024-01-02T03:04:05Z,2024-01-02,abc,octocat,The Octocat,first,2,10,3,13",
		"2024-02-03T04:05:06Z,2024-02-03,def,,Local Developer,second,1,4,2,6",
	}, "\n")

	stats, err := ParseCommitCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parse commit CSV: %v", err)
	}
	if stats.Commits != 2 || stats.FilesChanged != 3 || stats.LinesAdded != 14 || stats.LinesDeleted != 5 {
		t.Fatalf("unexpected totals: %+v", stats)
	}
	github := stats.Contributors["github:octocat"]
	if github.Login != "octocat" || github.Name != "The Octocat" || !strings.Contains(github.AvatarURL, "octocat") {
		t.Fatalf("unexpected GitHub contributor: %+v", github)
	}
	if _, ok := stats.Contributors["git:local developer"]; !ok {
		t.Fatalf("expected fallback Git display-name identity, got %#v", stats.Contributors)
	}
	if stats.Daily["2024-01-02"]["github:octocat"] != 1 || stats.Daily["2024-02-03"]["git:local developer"] != 1 {
		t.Fatalf("unexpected daily activity: %#v", stats.Daily)
	}
	if stats.FirstAt.Format(time.DateOnly) != "2024-01-02" || stats.LastAt.Format(time.DateOnly) != "2024-02-03" {
		t.Fatalf("unexpected commit activity bounds: %s to %s", stats.FirstAt, stats.LastAt)
	}
}

func TestBuildPullStatsUsesCreatedMonthAndExclusiveStates(t *testing.T) {
	created := time.Date(2024, time.March, 10, 0, 0, 0, 0, time.UTC)
	merged := created.Add(time.Hour)
	pulls := []PullRequest{
		{Number: 1, State: "open", CreatedAt: created, UpdatedAt: created, Author: Person{Login: "octocat"}},
		{Number: 2, State: "closed", CreatedAt: created, UpdatedAt: created, Author: Person{Login: "octocat"}},
		{Number: 3, State: "closed", MergedAt: &merged, CreatedAt: created, UpdatedAt: created, Author: Person{Login: "hubot", Type: "Bot"}},
	}

	stats := BuildPullStats(pulls)
	if stats.Totals != (PullRequestTotals{Opened: 3, Open: 1, Closed: 1, Merged: 1}) {
		t.Fatalf("unexpected pull request totals: %+v", stats.Totals)
	}
	if stats.Daily["2024-03-10"]["github:octocat"] != 2 {
		t.Fatalf("expected opened PRs to use creation day: %#v", stats.Daily)
	}
}

func TestBuildActivityFillsBucketsAndCollapsesAfterEightSeries(t *testing.T) {
	reports := make(map[int64]RepositoryReport)
	for index := int64(1); index <= 9; index++ {
		key := "github:user" + string(rune('a'+index-1))
		reports[index] = RepositoryReport{
			Repository: Repository{ID: index, Owner: OwnerIdentity{ID: index, Login: "owner" + string(rune('a'+index-1))}},
			Commits: CommitStats{
				Contributors: map[string]ContributorMetrics{key: {Key: key, Name: key}},
				Daily:        map[string]map[string]int{"2024-01-01": {key: int(index)}},
			},
		}
	}
	last := reports[9]
	last.Commits.Daily["2024-01-03"] = map[string]int{"github:useri": 1}
	reports[9] = last
	activity := BuildActivity(reports, ActivityQuery{Group: ActivityByContributor, Metric: ActivityCommits}, time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC))
	if len(activity.Series) != 9 {
		t.Fatalf("expected eight named series and Other, got %d", len(activity.Series))
	}
	other := activity.Series[8]
	if other.Key != "other" || other.Total != 1 {
		t.Fatalf("unexpected Other series: %+v", other)
	}
	if activity.Granularity != ActivityByDay || len(activity.Series[0].Points) != 3 || activity.Series[0].Points[1].Date != "2024-01-02" || activity.Series[0].Points[1].Value != 0 {
		t.Fatalf("expected zero-filled daily points: %+v", activity.Series[0].Points)
	}
	if activity.Series[0].Points[1].Month != "2024-01" {
		t.Fatalf("expected legacy month compatibility field: %+v", activity.Series[0].Points[1])
	}
}

func TestBuildActivityAdaptsGranularityToSelectedRange(t *testing.T) {
	key := "github:user"
	reports := map[int64]RepositoryReport{1: {
		Repository: Repository{ID: 1, Owner: OwnerIdentity{ID: 2, Login: "org"}},
		Commits: CommitStats{
			Contributors: map[string]ContributorMetrics{key: {Key: key, Name: "User"}},
			Daily: map[string]map[string]int{
				"2020-01-01": {key: 1},
				"2024-01-01": {key: 1},
				"2024-04-01": {key: 1},
				"2025-01-01": {key: 1},
			},
		},
	}}

	full := BuildActivity(reports, ActivityQuery{Group: ActivityByOwner, Metric: ActivityCommits}, time.Now())
	if full.Granularity != ActivityByMonth || full.AvailableFrom != "2020-01-01" || full.AvailableTo != "2025-01-01" {
		t.Fatalf("expected monthly full history with bounds, got %+v", full)
	}
	dayFrom, _ := time.Parse(time.DateOnly, "2024-01-01")
	dayTo, _ := time.Parse(time.DateOnly, "2024-01-31")
	daily := BuildActivity(reports, ActivityQuery{Group: ActivityByOwner, Metric: ActivityCommits, From: &dayFrom, To: &dayTo}, time.Now())
	if daily.Granularity != ActivityByDay || daily.From != "2024-01-01" || daily.To != "2024-01-31" || len(daily.Series[0].Points) != 31 {
		t.Fatalf("expected daily filtered history, got %+v", daily)
	}
	weekTo, _ := time.Parse(time.DateOnly, "2024-04-01")
	weekly := BuildActivity(reports, ActivityQuery{Group: ActivityByOwner, Metric: ActivityCommits, From: &dayFrom, To: &weekTo}, time.Now())
	if weekly.Granularity != ActivityByWeek {
		t.Fatalf("expected weekly filtered history, got %+v", weekly)
	}
}

func TestBuildSnapshotMarksUnavailablePullRequests(t *testing.T) {
	repository := Repository{ID: 1, Name: "repo", FullName: "org/repo", Owner: OwnerIdentity{ID: 2, Login: "org"}}
	report := RepositoryReport{
		Repository: repository,
		Commits: CommitStats{
			Commits:      1,
			Contributors: map[string]ContributorMetrics{"github:user": {Key: "github:user", Name: "User", Commits: 1}},
			Daily:        map[string]map[string]int{"2024-01-01": {"github:user": 1}},
		},
	}
	snapshot := BuildSnapshot(Viewer{Login: "viewer"}, []Repository{repository}, map[int64]RepositoryReport{1: report}, nil)
	if snapshot.Repositories[0].PullRequests != nil || snapshot.Totals.RepositoriesWithoutPRAccess != 1 {
		t.Fatalf("expected unavailable PR data, got %+v", snapshot)
	}
}

func TestRepositoryLivenessUsesQuarterOfWorkingLifespan(t *testing.T) {
	first := time.Date(2020, time.January, 1, 12, 0, 0, 0, time.UTC)
	last := time.Date(2024, time.January, 1, 12, 0, 0, 0, time.UTC)
	evaluated := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)
	liveness := BuildRepositoryLiveness(CommitStats{FirstAt: first, LastAt: last}, time.Time{}, evaluated)

	if !liveness.IsDead || liveness.State != RepositoryDead || liveness.Scale != "year" {
		t.Fatalf("expected a dead, yearly-scale repository: %+v", liveness)
	}
	if liveness.ActiveSpanDays != 1462 || liveness.ThresholdDays != 366 || liveness.InactiveDays != 366 || liveness.ThresholdValue != 1 {
		t.Fatalf("unexpected lifespan-relative threshold: %+v", liveness)
	}
}

func TestRepositoryLivenessAdaptsScaleAndHandlesEmptyRepositories(t *testing.T) {
	evaluated := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	weekly := BuildRepositoryLiveness(CommitStats{
		FirstAt: time.Date(2024, time.December, 1, 0, 0, 0, 0, time.UTC),
		LastAt:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
	}, time.Time{}, evaluated)
	if weekly.Scale != "week" || weekly.ThresholdDays != 8 || weekly.IsDead != true {
		t.Fatalf("expected weekly liveness threshold: %+v", weekly)
	}

	empty := BuildRepositoryLiveness(CommitStats{}, evaluated.AddDate(0, 0, -3), evaluated)
	if !empty.IsDead || empty.Reason != "no_default_branch_commits" || empty.ThresholdDays != 1 {
		t.Fatalf("expected an old empty repository to be dead: %+v", empty)
	}
	unknown := BuildRepositoryLiveness(CommitStats{}, time.Time{}, evaluated)
	if unknown.State != RepositoryUnknown || unknown.IsDead {
		t.Fatalf("expected missing commit and creation metadata to remain unknown: %+v", unknown)
	}
}

func TestBuildSnapshotCountsDeadRepositories(t *testing.T) {
	created := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	repository := Repository{ID: 1, FullName: "org/dead", CreatedAt: created, Owner: OwnerIdentity{ID: 2, Login: "org"}}
	snapshot := BuildSnapshot(Viewer{}, []Repository{repository}, map[int64]RepositoryReport{
		1: {Repository: repository},
	}, nil)
	if snapshot.Totals.DeadRepositories != 1 || snapshot.Owners[0].DeadRepositories != 1 || !snapshot.Repositories[0].Liveness.IsDead {
		t.Fatalf("expected dead metadata at repository, owner, and dashboard levels: %+v", snapshot)
	}
}
