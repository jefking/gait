package dashboard

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

func TestParseCommitCSVAndContributorIdentity(t *testing.T) {
	csvData := strings.Join([]string{
		strings.Join(authorEmailCommitCSVHeader, ","),
		"2024-01-02T03:04:05Z,2024-01-02,abc,1208574+octocat@users.noreply.github.com,octocat,The Octocat,first,2,10,3,13",
		"2024-02-03T04:05:06Z,2024-02-03,def,local@example.com,,Local Developer,second,1,4,2,6",
		"2024-03-04T05:06:07Z,2024-03-04,ghi,LOCAL@example.com,,Local Dev,third,1,2,1,3",
	}, "\n")

	stats, err := ParseCommitCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parse commit CSV: %v", err)
	}
	if stats.Commits != 3 || stats.FilesChanged != 4 || stats.LinesAdded != 16 || stats.LinesDeleted != 6 {
		t.Fatalf("unexpected totals: %+v", stats)
	}
	github := stats.Contributors["github:octocat"]
	if github.Login != "octocat" || github.Name != "The Octocat" || !strings.Contains(github.AvatarURL, "octocat") {
		t.Fatalf("unexpected GitHub contributor: %+v", github)
	}
	email := stats.Contributors["email:local@example.com"]
	if email.Commits != 2 || email.Name != "Local Developer" {
		t.Fatalf("expected stable, aggregated email identity, got %#v", email)
	}
	if stats.Daily["2024-01-02"]["github:octocat"] != 1 || stats.Daily["2024-02-03"]["email:local@example.com"] != 1 || stats.Daily["2024-03-04"]["email:local@example.com"] != 1 {
		t.Fatalf("unexpected daily activity: %#v", stats.Daily)
	}
	if stats.FirstAt.Format(time.DateOnly) != "2024-01-02" || stats.LastAt.Format(time.DateOnly) != "2024-03-04" {
		t.Fatalf("unexpected commit activity bounds: %s to %s", stats.FirstAt, stats.LastAt)
	}
}

func TestParseCommitCSVUsesStructuredCoauthors(t *testing.T) {
	var csvData bytes.Buffer
	writer := csv.NewWriter(&csvData)
	_ = writer.Write(commitCSVHeader)
	_ = writer.Write([]string{
		"2026-08-12T23:00:00Z", "2026-08-12", "abc", "human@example.com", "", "Human", "pair work",
		"2", "10", "3", "13",
		`["noreply@anthropic.com","260473928+moltenbot000@users.noreply.github.com","human@example.com"]`,
		`["","moltenbot000",""]`,
		`["Claude Code","Molten Bot 000","Human"]`,
	})
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatal(err)
	}

	stats, err := ParseCommitCSV(&csvData)
	if err != nil {
		t.Fatalf("parse structured co-authors: %v", err)
	}
	participants := stats.Events[0].Participants
	if len(participants) != 3 {
		t.Fatalf("participants = %+v, want author plus two distinct co-authors", participants)
	}
	if participants[0].Key != "email:human@example.com" || participants[1].Key != "email:noreply@anthropic.com" || participants[1].Type != "AgentSignature" {
		t.Fatalf("unexpected ordered participants: %+v", participants)
	}
	if participants[2].Key != "github:moltenbot000" || participants[2].Login != "moltenbot000" || participants[2].Type != "AgentSignature" {
		t.Fatalf("GitHub noreply identity was not preserved: %+v", participants[2])
	}
	if stats.Contributors[participants[1].Key].Commits != 1 || stats.Daily["2026-08-12"][participants[2].Key] != 1 {
		t.Fatalf("co-authors were not included in contributor/day evidence: contributors=%+v daily=%+v", stats.Contributors, stats.Daily)
	}
}

func TestParseCommitCSVRejectsMisalignedStructuredCoauthors(t *testing.T) {
	var csvData bytes.Buffer
	writer := csv.NewWriter(&csvData)
	_ = writer.Write(commitCSVHeader)
	_ = writer.Write([]string{
		"2026-08-12T23:00:00Z", "2026-08-12", "abc", "human@example.com", "", "Human", "pair work",
		"2", "10", "3", "13", `["one@example.com"]`, `[]`, `["One"]`,
	})
	writer.Flush()

	_, err := ParseCommitCSV(&csvData)
	if err == nil || !strings.Contains(err.Error(), "parallel co-author arrays") {
		t.Fatalf("error = %v, want mismatched-array validation", err)
	}
}

func TestParseCommitCSVSupportsLegacyCachedReports(t *testing.T) {
	csvData := strings.Join([]string{
		strings.Join(legacyCommitCSVHeader, ","),
		"2024-01-02T03:04:05Z,2024-01-02,abc,,Local Developer,first,2,10,3,13",
	}, "\n")

	stats, err := ParseCommitCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parse legacy commit CSV: %v", err)
	}
	if _, ok := stats.Contributors["git:local developer"]; !ok {
		t.Fatalf("expected legacy display-name identity, got %#v", stats.Contributors)
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

func TestBuildActivityExcludesDeadRepositories(t *testing.T) {
	evaluatedAt := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	reports := map[int64]RepositoryReport{
		1: {
			Repository: Repository{ID: 1, Owner: OwnerIdentity{ID: 1, Login: "org"}},
			Commits:    CommitStats{Daily: map[string]map[string]int{"2020-01-01": {"old": 3}}},
		},
		2: {
			Repository: Repository{ID: 2, Owner: OwnerIdentity{ID: 1, Login: "org"}},
			Commits:    CommitStats{Daily: map[string]map[string]int{"2025-01-10": {"current": 2}}},
		},
	}

	activity := BuildActivity(reports, ActivityQuery{Group: ActivityByOwner, Metric: ActivityCommits, ExcludeDead: true}, evaluatedAt)
	if activity.AvailableFrom != "2025-01-10" || len(activity.Series) != 1 || activity.Series[0].Total != 2 {
		t.Fatalf("expected only active repository data, got %+v", activity)
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
