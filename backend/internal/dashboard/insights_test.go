package dashboard

import (
	"fmt"
	"testing"
	"time"
)

func TestInsightsClassifyWorkBuildRelationshipsAndMeasureHandoffs(t *testing.T) {
	start := time.Date(2025, time.January, 1, 9, 0, 0, 0, time.UTC)
	human := ContributorMetrics{Key: "github:alice", Login: "alice", Name: "Alice"}
	agent := ContributorMetrics{Key: "github:helper[bot]", Login: "helper[bot]", Name: "Helper"}
	report := RepositoryReport{
		Repository: Repository{ID: 1, FullName: "org/repo", Owner: OwnerIdentity{ID: 2, Login: "org"}},
		Commits: CommitStats{Events: []CommitEvent{
			{Hash: "a", CommittedAt: start, Author: human, Message: "start", LinesAdded: 5},
			{Hash: "b", CommittedAt: start.Add(time.Hour), Author: agent, Message: "continue", LinesAdded: 8},
			{Hash: "c", CommittedAt: start.Add(2 * time.Hour), Author: human, Message: "finish\n\nCo-authored-by: Helper <helper[bot]@users.noreply.github.com>", LinesAdded: 3},
		}},
	}
	reports := map[int64]RepositoryReport{1: report}
	query, meta, err := prepareInsightQuery(reports, InsightQuery{SessionHours: 72, AdoptionDays: 30, SurvivalDays: 30})
	if err != nil {
		t.Fatalf("prepare query: %v", err)
	}
	overview := buildOverview(reports, nil, query, meta)
	if overview.Meta.Coverage.TotalCommits != 3 || overview.Meta.Coverage.UnknownCommits != 0 {
		t.Fatalf("unexpected coverage: %+v", overview.Meta.Coverage)
	}
	if len(overview.Timeline) != 1 || overview.Timeline[0].HumanOnly != 1 || overview.Timeline[0].AgentOnly != 1 || overview.Timeline[0].Mixed != 1 {
		t.Fatalf("work was not partitioned exactly once: %+v", overview.Timeline)
	}
	agentQuery := query
	agentQuery.ActorKind = ActorAgent
	agentOverview := buildOverview(reports, nil, agentQuery, meta)
	if agentOverview.Meta.Coverage.TotalCommits != 2 || agentOverview.Timeline[0].HumanOnly != 0 || agentOverview.Timeline[0].AgentOnly != 1 || agentOverview.Timeline[0].Mixed != 1 {
		t.Fatalf("agent participation filter did not preserve mixed work: %+v", agentOverview)
	}
	network := buildNetwork(reports, nil, query, meta)
	if len(network.Nodes) != 2 || len(network.Edges) != 1 {
		t.Fatalf("unexpected network: %+v", network)
	}
	edge := network.Edges[0]
	if edge.PairType != "human_agent" || edge.HumanToAgent != 1 || edge.Coauthorships != 1 || edge.InteractionDays != 1 {
		t.Fatalf("unexpected relationship evidence: %+v", edge)
	}
	rampQuery := query
	rampTo := start.AddDate(0, 0, 5)
	rampQuery.To = &rampTo
	ramps := buildRamps(reports, nil, rampQuery, meta)
	if len(ramps.Handoffs) != 1 || ramps.Handoffs[0].Episodes != 1 || ramps.Handoffs[0].Completed != 1 || ramps.Handoffs[0].InteractionDays != 1 || ramps.Handoffs[0].Baseline == 0 || ramps.Handoffs[0].After == 0 {
		t.Fatalf("unexpected handoff analysis: %+v", ramps.Handoffs)
	}
	if ramps.Meta.Coverage.TotalCommits != 3 {
		t.Fatalf("non-overview response did not include coverage: %+v", ramps.Meta.Coverage)
	}
}

func TestKnownAgentNamesRequireTrailerEvidence(t *testing.T) {
	plain := classifyIdentity(ContributorMetrics{Key: "git:claude", Name: "Claude"})
	if plain.Kind != ActorUnknown {
		t.Fatalf("a name alone must not identify an agent: %+v", plain)
	}
	coauthors := commitCoauthors("Co-authored-by: Claude <noreply@anthropic.com>")
	if len(coauthors) != 1 || classifyIdentity(coauthors[0]).Kind != ActorAgent {
		t.Fatalf("expected exact co-author signature evidence: %+v", coauthors)
	}
}

func TestNetworkCapsCanvasPayloadAndReportsTotalIdentityCount(t *testing.T) {
	at := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	events := make([]CommitEvent, 0, 80)
	for index := range 80 {
		login := fmt.Sprintf("person-%02d", index)
		events = append(events, CommitEvent{Hash: login, CommittedAt: at.Add(time.Duration(index) * time.Minute), Author: ContributorMetrics{Key: "github:" + login, Login: login, Name: login}})
	}
	reports := map[int64]RepositoryReport{1: {Repository: Repository{ID: 1}, Commits: CommitStats{Events: events}}}
	query, meta, err := prepareInsightQuery(reports, InsightQuery{})
	if err != nil {
		t.Fatal(err)
	}
	network := buildNetwork(reports, nil, query, meta)
	if len(network.Nodes) != 75 || network.TotalIdentities != 80 || !network.Meta.Truncated || network.Meta.TotalResults != 80 {
		t.Fatalf("unexpected truncation metadata: nodes=%d total=%d meta=%+v", len(network.Nodes), network.TotalIdentities, network.Meta)
	}
}

func TestInsightsCanExcludeDeadRepositories(t *testing.T) {
	now := time.Now().UTC()
	old := now.AddDate(-2, 0, 0)
	reports := map[int64]RepositoryReport{
		1: {Repository: Repository{ID: 1, FullName: "org/dead", CreatedAt: old}, Commits: CommitStats{Commits: 2, FirstAt: old, LastAt: old.AddDate(0, 0, 2), Events: []CommitEvent{{Hash: "old", CommittedAt: old, Author: ContributorMetrics{Key: "github:old", Login: "old"}}}}},
		2: {Repository: Repository{ID: 2, FullName: "org/active", CreatedAt: old}, Commits: CommitStats{Commits: 2, FirstAt: old, LastAt: now, Events: []CommitEvent{{Hash: "current", CommittedAt: now, Author: ContributorMetrics{Key: "github:current", Login: "current"}}}}},
	}
	query, meta, err := prepareInsightQuery(reports, InsightQuery{ExcludeDead: true})
	if err != nil {
		t.Fatal(err)
	}
	overview := buildOverview(reports, nil, query, meta)
	if overview.Meta.Coverage.TotalCommits != 1 || len(overview.Repositories) != 1 || overview.Repositories[0].Name != "org/active" {
		t.Fatalf("dead repository contributed to filtered insights: %+v", overview)
	}
}

func TestLargeFiveYearPortfolioKeepsGraphAndPulsePayloadsBounded(t *testing.T) {
	start := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	reports := make(map[int64]RepositoryReport, 100)
	for repositoryIndex := range 100 {
		events := make([]CommitEvent, 0, 24)
		for eventIndex := range 24 {
			identityIndex := (repositoryIndex + eventIndex) % 80
			login := fmt.Sprintf("person-%02d", identityIndex)
			events = append(events, CommitEvent{Hash: fmt.Sprintf("%d-%d", repositoryIndex, eventIndex), CommittedAt: start.AddDate(0, eventIndex*3, repositoryIndex%27), Author: ContributorMetrics{Key: "github:" + login, Login: login, Name: login}})
		}
		id := int64(repositoryIndex + 1)
		reports[id] = RepositoryReport{Repository: Repository{ID: id, FullName: fmt.Sprintf("org/repo-%03d", repositoryIndex)}, Commits: CommitStats{Events: events}}
	}
	query, meta, err := prepareInsightQuery(reports, InsightQuery{})
	if err != nil {
		t.Fatal(err)
	}
	overview := buildOverview(reports, nil, query, meta)
	network := buildNetwork(reports, nil, query, meta)
	if len(overview.Repositories) != 100 || len(network.Nodes) != 75 || network.TotalIdentities != 80 || !network.Meta.Truncated {
		t.Fatalf("large fixture was not bounded as expected: repos=%d nodes=%d total=%d", len(overview.Repositories), len(network.Nodes), network.TotalIdentities)
	}
}

func TestIdentityOverridesAndAliasesAreAppliedWithoutDoubleCounting(t *testing.T) {
	at := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	reports := map[int64]RepositoryReport{1: {
		Repository: Repository{ID: 1, Owner: OwnerIdentity{ID: 2}},
		Commits: CommitStats{Events: []CommitEvent{
			{Hash: "a", CommittedAt: at, Author: ContributorMetrics{Key: "git:alex", Name: "Alex"}},
			{Hash: "b", CommittedAt: at.Add(time.Hour), Author: ContributorMetrics{Key: "github:alex", Login: "alex", Name: "Alex"}},
		}},
	}}
	overrides := map[string]IdentityOverride{
		"git:alex":    {CanonicalKey: "github:alex"},
		"github:alex": {Kind: ActorAgent, DisplayName: "Alex Agent"},
	}
	catalog := buildIdentityCatalog(reports, overrides)
	identity := catalog["github:alex"]
	if len(catalog) != 1 || identity == nil || identity.Kind != ActorAgent || identity.Name != "Alex Agent" || identity.Commits != 2 {
		t.Fatalf("unexpected merged identity: %#v", catalog)
	}
	if len(identity.aliases) != 2 {
		t.Fatalf("aliases were not retained: %#v", identity.aliases)
	}
}

func TestInsightQueryValidatesAnalysisWindows(t *testing.T) {
	if _, _, err := prepareInsightQuery(nil, InsightQuery{SessionHours: 169}); err == nil {
		t.Fatal("expected invalid session window")
	}
	if _, _, err := prepareInsightQuery(nil, InsightQuery{AdoptionDays: 6}); err == nil {
		t.Fatal("expected invalid adoption window")
	}
	if _, _, err := prepareInsightQuery(nil, InsightQuery{SurvivalDays: 181}); err == nil {
		t.Fatal("expected invalid survival window")
	}
	if _, _, err := prepareInsightQuery(nil, InsightQuery{ActorKind: "machine"}); err == nil {
		t.Fatal("expected invalid actor kind")
	}
	at := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	reports := map[int64]RepositoryReport{1: {Commits: CommitStats{Events: []CommitEvent{{CommittedAt: at}}}}}
	after := at.AddDate(0, 0, 2)
	if _, _, err := prepareInsightQuery(reports, InsightQuery{From: &after}); err == nil {
		t.Fatal("expected a non-overlapping date range to fail")
	}
}

func TestIdentityOverridesPersistAtomically(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]IdentityOverride{"github:bot": {Kind: ActorAgent, DisplayName: "Build Agent"}}
	if err := store.SaveIdentityOverrides(want); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadIdentityOverrides()
	if err != nil {
		t.Fatal(err)
	}
	if got["github:bot"] != want["github:bot"] {
		t.Fatalf("unexpected overrides: %#v", got)
	}
}

func TestVersionedAnalysisEventsPersistBesideLegacyReport(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := []CommitEvent{{Hash: "abc", CommittedAt: time.Date(2025, time.January, 2, 3, 0, 0, 0, time.UTC), Message: "full message"}}
	if err := store.SaveAnalysisCache(42, want); err != nil {
		t.Fatal(err)
	}
	cache, err := store.LoadAnalysisCache(42)
	if err != nil {
		t.Fatal(err)
	}
	if cache.Version != 2 || len(cache.Events) != 1 || cache.Events[0].Message != want[0].Message {
		t.Fatalf("unexpected analysis cache: %+v", cache)
	}
}

func TestIdentityPatchPreservesClassificationAndCanUnmerge(t *testing.T) {
	manager, err := NewManager(ManagerConfig{DataDir: t.TempDir(), Runner: &fakeRepositoryRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.UpdateIdentity("git:helper", IdentityOverride{Kind: ActorAgent}); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.UpdateIdentity("git:helper", IdentityOverride{CanonicalKey: "github:helper"}); err != nil {
		t.Fatal(err)
	}
	if got := manager.identityOverrides["git:helper"]; got.Kind != ActorAgent || got.CanonicalKey != "github:helper" {
		t.Fatalf("patch fields were not merged: %+v", got)
	}
	if _, err := manager.UpdateIdentity("git:helper", IdentityOverride{Unmerge: true}); err != nil {
		t.Fatal(err)
	}
	if got := manager.identityOverrides["git:helper"]; got.Kind != ActorAgent || got.CanonicalKey != "" {
		t.Fatalf("unmerge removed classification: %+v", got)
	}
}

func TestManagerInsightMethodsExposeValidatedResults(t *testing.T) {
	manager, err := NewManager(ManagerConfig{DataDir: t.TempDir(), Runner: &fakeRepositoryRunner{}})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	at := time.Date(2025, time.January, 2, 12, 0, 0, 0, time.UTC)
	manager.reports = map[int64]RepositoryReport{1: {
		Repository: Repository{ID: 1, FullName: "org/repo"},
		Commits:    CommitStats{Events: []CommitEvent{{Hash: "abc", CommittedAt: at, Author: ContributorMetrics{Key: "github:alex", Login: "alex", Name: "Alex"}}}},
	}}
	manager.identityOverrides = map[string]IdentityOverride{"github:alex": {Kind: ActorHuman}}

	overview, err := manager.InsightOverview(InsightQuery{})
	if err != nil || overview.Meta.Coverage.TotalCommits != 1 {
		t.Fatalf("unexpected overview: %+v, %v", overview, err)
	}
	network, err := manager.InsightNetwork(InsightQuery{})
	if err != nil || len(network.Nodes) != 1 {
		t.Fatalf("unexpected network: %+v, %v", network, err)
	}
	ramps, err := manager.InsightRamps(InsightQuery{})
	if err != nil || ramps.Handoffs == nil {
		t.Fatalf("unexpected ramps: %+v, %v", ramps, err)
	}
	rankings, err := manager.InsightRankings(InsightQuery{Cohort: "humans", Metric: "commits"})
	if err != nil || len(rankings.Leaderboard) != 1 || rankings.Leaderboard[0].Label != "Alex" {
		t.Fatalf("unexpected rankings: %+v, %v", rankings, err)
	}
	for name, call := range map[string]func() error{
		"overview": func() error { _, err := manager.InsightOverview(InsightQuery{SessionHours: 999}); return err },
		"network":  func() error { _, err := manager.InsightNetwork(InsightQuery{SessionHours: 999}); return err },
		"ramps":    func() error { _, err := manager.InsightRamps(InsightQuery{SessionHours: 999}); return err },
		"rankings": func() error { _, err := manager.InsightRankings(InsightQuery{SessionHours: 999}); return err },
	} {
		t.Run(name+" validates query", func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("expected invalid query error")
			}
		})
	}
}

func TestRankingValidationOrderingAndStatistics(t *testing.T) {
	if _, err := buildRankings(nil, nil, InsightQuery{Cohort: "robots"}, InsightMeta{}); err == nil {
		t.Fatal("expected invalid cohort error")
	}
	if _, err := buildRankings(nil, nil, InsightQuery{Cohort: "humans", Metric: "handoffs"}, InsightMeta{}); err == nil {
		t.Fatal("expected invalid individual metric error")
	}
	entries := orderedRanks(
		map[string]float64{"b": 1, "a": 1, "c": 2},
		map[string]string{"a": "Alpha", "b": "Beta", "c": "Charlie"},
		map[string]ActorKind{"a": ActorHuman, "b": ActorHuman, "c": ActorAgent},
		map[string]map[string]float64{}, "higher",
	)
	if len(entries) != 3 || entries[0].Key != "c" || entries[1].Key != "a" || entries[2].Rank != 3 {
		t.Fatalf("unexpected ranking order: %+v", entries)
	}
	lower := orderedRanks(map[string]float64{"a": 2, "b": 1}, map[string]string{"a": "A", "b": "B"}, nil, nil, "lower")
	if lower[0].Key != "b" {
		t.Fatalf("lower-is-better order ignored: %+v", lower)
	}
	if median(nil) != 0 || median([]float64{9, 1, 5}) != 5 || median([]float64{8, 2}) != 5 {
		t.Fatal("median failed empty, odd, or even input")
	}
	values := []string{"one", "two", "three"}
	got := removeString(values, "two")
	if len(got) != 2 || got[0] != "one" || got[1] != "three" {
		t.Fatalf("unexpected removal: %v", got)
	}
}

func TestPullApprovalUsesLatestEligibleIndependentReview(t *testing.T) {
	merged := time.Date(2025, time.January, 3, 0, 0, 0, 0, time.UTC)
	before := merged.Add(-time.Hour)
	after := merged.Add(time.Hour)
	pull := PullRequest{Author: Person{Login: "author"}, MergedAt: &merged, Reviews: []PullRequestReview{
		{Author: Person{Login: "author"}, State: "APPROVED", SubmittedAt: &before},
		{Author: Person{Login: "reviewer"}, State: "CHANGES_REQUESTED", SubmittedAt: &before},
		{Author: Person{Login: "reviewer"}, State: "APPROVED", SubmittedAt: &after},
	}}
	if pullHasApproval(pull) {
		t.Fatal("post-merge and self approvals must not count")
	}
	pull.Reviews = append(pull.Reviews, PullRequestReview{Author: Person{Login: "second"}, State: " approved ", SubmittedAt: &before})
	if !pullHasApproval(pull) {
		t.Fatal("eligible independent approval was not detected")
	}
}
