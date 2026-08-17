package dashboard

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestDeliveryAttributionAndBaselineIndexInvariants(t *testing.T) {
	from := mustDeliveryDate("2025-01-06")
	to := mustDeliveryDate("2025-03-30")
	pulls := make([]PullRequest, 0)
	for week := range 12 {
		merged := from.AddDate(0, 0, week*7+6)
		author := Person{Login: "human", Type: "User"}
		pull := deliveryTestPull(int64(week+1), merged, author, 80+week*10, 20, 2)
		switch week {
		case 4:
			pull.Author = Person{Login: "agent[bot]", Type: "Bot"}
			pull.CommitEvidence[0].Author = pull.Author
		case 5:
			submitted := merged.Add(-time.Hour)
			pull.Reviews = []PullRequestReview{{ID: 1, State: "COMMENTED", SubmittedAt: &submitted, Author: Person{Login: "agent[bot]", Type: "Bot"}}}
			pull.CommitEvidence[0].Message = "ship\n\nCo-authored-by: agent[bot] <agent[bot]@users.noreply.github.com>"
		case 6:
			pull.CommitEvidence = append(pull.CommitEvidence, PullRequestCommit{SHA: "unknown", Author: Person{Name: "Unknown"}})
		case 7:
			pull.Author = Person{Name: "Unknown"}
			pull.CommitEvidence = nil
			pull.CommitEvidenceComplete = false
		}
		pulls = append(pulls, pull)
	}
	report := deliveryTestReport(1, 10, "Organization", pulls)
	personal := deliveryTestReport(2, 20, "User", []PullRequest{deliveryTestPull(99, from, Person{Login: "personal", Type: "User"}, 10000, 10000, 100)})
	manager := &Manager{reports: map[int64]RepositoryReport{1: report, 2: personal}, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{OwnerID: 10, From: &from, To: &to})
	if err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if response.Meta.Coverage.Repositories != 1 || response.Meta.Coverage.MergedPullRequests != 12 || response.Meta.Coverage.AttributedPullRequests != 10 || response.Meta.Coverage.UnattributedPullRequests != 2 || response.Meta.Coverage.IncompleteAuthorship != 1 || response.Meta.Coverage.UnknownAuthorship != 1 {
		t.Fatalf("unexpected scoped attribution coverage: %+v", response.Meta.Coverage)
	}
	if response.Raw.Human.MergedPullRequests != 8 || response.Raw.Agent.MergedPullRequests != 1 || response.Raw.Collaborative.MergedPullRequests != 1 || response.Raw.AuthorshipUnknown.MergedPullRequests != 2 || response.Raw.Total.MergedPullRequests != 12 {
		t.Fatalf("unexpected work-mode attribution: %+v", response.Raw)
	}
	if response.Performance.Overall.AuthorshipUnknown.MergedPullRequests != 2 {
		t.Fatalf("unknown authorship was not exposed in performance: %+v", response.Performance.Overall)
	}
	if response.Raw.Total.ChangedLines != response.Raw.Total.Additions+response.Raw.Total.Deletions {
		t.Fatalf("changed-line arithmetic double counted: %+v", response.Raw.Total)
	}
	opening := 0.0
	periods := 0
	for _, point := range response.Velocity {
		if !point.Complete || periods == 4 {
			continue
		}
		if math.Abs(point.TotalIndex-(point.Human.Index+point.Agent.Index+point.Collaborative.Index)) > 1e-9 {
			t.Fatalf("mode contributions do not add to total: %+v", point)
		}
		opening += point.TotalIndex
		periods++
	}
	if periods != 4 || math.Abs(opening/4-100) > 1e-9 {
		t.Fatalf("opening average = %f across %d periods, want 100", opening/float64(max(1, periods)), periods)
	}
	if response.Summary.Leader == "" {
		t.Fatal("single leader contract was not populated")
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"organization`) || !strings.Contains(string(encoded), `"owners":1`) || !strings.Contains(string(encoded), `"owner_id":10`) || !strings.Contains(string(encoded), `"owner_type":"Organization"`) {
		t.Fatalf("delivery JSON did not use the owner-only metadata contract: %s", encoded)
	}
}

func TestDeliveryEmptyScopeSerializesCollectionsAsArrays(t *testing.T) {
	report := deliveryTestReport(1, 10, "Organization", nil)
	manager := &Manager{reports: map[int64]RepositoryReport{1: report}, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{OwnerID: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.Velocity == nil || response.Performance.Daily == nil || response.Quality.Points == nil || response.Quality.Signals == nil || response.Flow.Points == nil || response.Impact.QualityDeltas == nil {
		t.Fatalf("empty delivery collections must be non-nil: %+v", response)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"velocity":[]`, `"daily":[]`, `"signals":[]`, `"points":[]`, `"quality_deltas":[]`} {
		if !strings.Contains(string(encoded), field) {
			t.Errorf("delivery JSON does not contain %s: %s", field, encoded)
		}
	}
}

func TestDeliveryAttributionPrefersStructuredCommitParticipants(t *testing.T) {
	merged := mustDeliveryDate("2025-02-10")
	pull := deliveryTestPull(1, merged, Person{Login: "human", Type: "User"}, 8, 2, 1)
	pull.CommitEvidence[0].Message = "message without co-author trailers"
	report := deliveryTestReport(1, 10, "Organization", []PullRequest{pull})
	report.Commits.Events = []CommitEvent{{
		Hash: pull.CommitEvidence[0].SHA, CommittedAt: merged,
		Author: ContributorMetrics{Key: "github:human", Login: "human", Name: "human"},
		Participants: []ContributorMetrics{
			{Key: "github:human", Login: "human", Name: "human"},
			{Key: "email:noreply@anthropic.com", Name: "Claude Code", Type: "AgentSignature"},
		},
	}}
	manager := &Manager{reports: map[int64]RepositoryReport{1: report}, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{From: &merged, To: &merged})
	if err != nil {
		t.Fatal(err)
	}
	if response.Raw.Collaborative.MergedPullRequests != 1 || response.Raw.Human.MergedPullRequests != 0 {
		t.Fatalf("structured commit participants did not drive PR attribution: %+v", response.Raw)
	}
}

func TestDeliveryAuthorshipIgnoresPullRequestOpenerAndReviewer(t *testing.T) {
	merged := mustDeliveryDate("2025-02-10")
	pull := deliveryTestPull(1, merged, Person{Login: "human", Type: "User"}, 8, 2, 1)
	pull.CommitEvidence[0].Author = Person{Login: "agent[bot]", Type: "Bot"}
	submitted := merged.Add(-time.Hour)
	pull.Reviews = []PullRequestReview{{ID: 1, State: "APPROVED", SubmittedAt: &submitted, Author: Person{Login: "reviewer", Type: "User"}}}
	manager := &Manager{reports: map[int64]RepositoryReport{1: deliveryTestReport(1, 10, "Organization", []PullRequest{pull})}, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{From: &merged, To: &merged})
	if err != nil {
		t.Fatal(err)
	}
	if response.Raw.Agent.MergedPullRequests != 1 || response.Raw.Human.MergedPullRequests != 0 || response.Raw.Collaborative.MergedPullRequests != 0 {
		t.Fatalf("workflow participants incorrectly changed code authorship: %+v", response.Raw)
	}
}

func TestDeliveryPerformanceSeparatesDailyParticipantCompositions(t *testing.T) {
	from := mustDeliveryDate("2025-01-06")
	to := mustDeliveryDate("2025-01-10")
	people := []struct {
		date     time.Time
		author   Person
		coauthor *Person
	}{
		{from, Person{Login: "alice", Type: "User"}, nil},
		{from.AddDate(0, 0, 1), Person{Login: "alice", Type: "User"}, &Person{Login: "bob", Type: "User"}},
		{from.AddDate(0, 0, 2), Person{Login: "alice", Type: "User"}, &Person{Login: "agent[bot]", Type: "Bot"}},
		{from.AddDate(0, 0, 3), Person{Login: "agent[bot]", Type: "Bot"}, nil},
		{from.AddDate(0, 0, 4), Person{Login: "agent[bot]", Type: "Bot"}, nil},
	}
	pulls := make([]PullRequest, 0, len(people))
	for index, participant := range people {
		pull := deliveryTestPull(int64(index+1), participant.date, participant.author, 8, 2, 1)
		if participant.coauthor != nil {
			pull.CommitEvidence = append(pull.CommitEvidence, PullRequestCommit{SHA: "coauthor", Author: *participant.coauthor})
			pull.Commits = len(pull.CommitEvidence)
		}
		pulls = append(pulls, pull)
	}
	manager := &Manager{reports: map[int64]RepositoryReport{1: deliveryTestReport(1, 10, "Organization", pulls)}, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Performance.Daily) != 5 {
		t.Fatalf("daily performance points = %d, want 5", len(response.Performance.Daily))
	}
	wantLeaders := []string{"human", "human_human", "human_agent", "agent", "agent"}
	for index, point := range response.Performance.Daily {
		if point.Leader != wantLeaders[index] || math.Abs(point.TotalIndex-100) > 1e-9 {
			t.Fatalf("daily point %d = %+v, want leader %q at index 100", index, point, wantLeaders[index])
		}
	}
	overall := response.Performance.Overall
	if overall.Leader != "agent" || overall.Agent.MergedPullRequests != 2 || overall.Human.MergedPullRequests != 1 || overall.HumanHuman.MergedPullRequests != 1 || overall.HumanAgent.MergedPullRequests != 1 {
		t.Fatalf("unexpected overall performance: %+v", overall)
	}
}

func TestDeliveryPortfolioEqualWeightsRepositoryIndices(t *testing.T) {
	from := mustDeliveryDate("2025-01-06")
	to := mustDeliveryDate("2025-03-16")
	makePulls := func(scale int, lastCount int) []PullRequest {
		pulls := make([]PullRequest, 0)
		for week := range 10 {
			count := 1
			if week == 9 {
				count = lastCount
			}
			for item := range count {
				merged := from.AddDate(0, 0, week*7+6)
				pulls = append(pulls, deliveryTestPull(int64(week*10+item+1), merged, Person{Login: "human", Type: "User"}, 8*scale, 2*scale, 1))
			}
		}
		return pulls
	}
	// The large repository stays at baseline (100); the small repository
	// doubles (200). Equal repository weighting therefore produces 150.
	reports := map[int64]RepositoryReport{
		1: deliveryTestReport(1, 10, "Organization", makePulls(1000, 1)),
		2: deliveryTestReport(2, 10, "Organization", makePulls(1, 2)),
	}
	manager := &Manager{reports: reports, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	last := response.Velocity[len(response.Velocity)-1]
	if math.Abs(last.TotalIndex-150) > 1e-9 {
		t.Fatalf("portfolio index %f, want equal-weight 150", last.TotalIndex)
	}
	if last.Human.Commits != 3 || last.TotalIndex != last.Human.Index {
		t.Fatalf("commits should remain raw context, not another index contribution: %+v", last)
	}
}

func TestDeliveryImpactUsesMatchedDifferenceInDifferencesWhenEligible(t *testing.T) {
	from := mustDeliveryDate("2025-01-06")
	to := mustDeliveryDate("2025-05-04")
	reports := make(map[int64]RepositoryReport)
	for repository := 1; repository <= 5; repository++ {
		pulls := make([]PullRequest, 0)
		number := int64(1)
		for week := range 8 {
			merged := from.AddDate(0, 0, week*7)
			pulls = append(pulls, deliveryTestPull(number, merged, Person{Login: "human", Type: "User"}, 8, 2, 1))
			number++
		}
		adoption := from.AddDate(0, 0, 62)
		if repository <= 3 {
			pulls = append(pulls, deliveryTestPull(number, adoption, Person{Login: "agent[bot]", Type: "Bot"}, 8, 2, 1))
			number++
		}
		for week := range 8 {
			count := 1
			if repository <= 3 {
				count = 2
			}
			for range count {
				merged := from.AddDate(0, 0, 69+week*7)
				pulls = append(pulls, deliveryTestPull(number, merged, Person{Login: "human", Type: "User"}, 8, 2, 1))
				number++
			}
		}
		reports[int64(repository)] = deliveryTestReport(int64(repository), 10, "Organization", pulls)
	}
	manager := &Manager{reports: reports, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	if response.Impact.Tier != "matched_difference_in_differences" || response.Impact.Verdict != "supported increase" || response.Impact.TreatedRepositories != 3 || response.Impact.ControlRepositories != 2 {
		t.Fatalf("controlled tier not selected: %+v", response.Impact)
	}
	if response.Impact.Estimate == nil || math.Abs(*response.Impact.Estimate-100) > 1e-9 || response.Impact.Low == nil || *response.Impact.Low <= 0 {
		t.Fatalf("unexpected controlled estimate: %+v", response.Impact)
	}
}

func TestScopedIdentitiesHonorOrganizationRepositoryAndDate(t *testing.T) {
	inside := mustDeliveryDate("2025-02-10")
	outside := mustDeliveryDate("2024-01-10")
	report := deliveryTestReport(1, 10, "Organization", []PullRequest{
		deliveryTestPull(1, inside, Person{Login: "inside", Type: "User"}, 1, 1, 1),
		deliveryTestPull(2, outside, Person{Login: "outside", Type: "User"}, 1, 1, 1),
	})
	manager := &Manager{reports: map[int64]RepositoryReport{1: report}, identityOverrides: map[string]IdentityOverride{}}
	from, to := mustDeliveryDate("2025-02-01"), mustDeliveryDate("2025-02-28")
	identities := manager.ScopedIdentities(InsightQuery{OwnerID: 10, RepositoryID: 1, From: &from, To: &to})
	if len(identities.Identities) != 1 || identities.Identities[0].Login != "inside" {
		t.Fatalf("identities leaked outside the global scope: %+v", identities)
	}
}

func TestOrganizationScopeIsolatesNetworkAndIdentityOverrides(t *testing.T) {
	at := mustDeliveryDate("2025-02-10")
	makeReport := func(id, organizationID int64, organization, login string) RepositoryReport {
		person := ContributorMetrics{Key: "github:" + login, Login: login, Name: login}
		return RepositoryReport{
			Repository: Repository{ID: id, Name: "repo", FullName: organization + "/repo", Owner: OwnerIdentity{ID: organizationID, Login: organization, Type: "Organization"}},
			Commits:    CommitStats{Events: []CommitEvent{{Hash: login, CommittedAt: at, Author: person, Participants: []ContributorMetrics{person}}}},
		}
	}
	reports := map[int64]RepositoryReport{
		1: makeReport(1, 10, "one", "alice"),
		2: makeReport(2, 20, "two", "bob"),
		3: makeReport(3, 10, "one", "carol"),
	}
	overrides := map[string]IdentityOverride{
		"github:alice": {Kind: ActorHuman},
		"github:bob":   {Kind: ActorHuman},
		"github:carol": {Kind: ActorHuman},
		"github:ghost": {Kind: ActorAgent, DisplayName: "Ghost"},
	}
	manager := &Manager{reports: reports, identityOverrides: overrides}
	from, to := at, at
	query := InsightQuery{OwnerID: 10, From: &from, To: &to}
	network, err := manager.InsightNetwork(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(network.Nodes) != 2 || network.TotalIdentities != 2 || !networkHasLogin(network, "alice") || !networkHasLogin(network, "carol") {
		t.Fatalf("network leaked identities outside organization scope: %+v", network.Nodes)
	}
	identities := manager.ScopedIdentities(query)
	if len(identities.Identities) != 2 || !identitiesHaveLogin(identities, "alice") || !identitiesHaveLogin(identities, "carol") {
		t.Fatalf("identity view leaked overrides outside organization scope: %+v", identities.Identities)
	}
	query.RepositoryID = 1
	repositoryNetwork, err := manager.InsightNetwork(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(repositoryNetwork.Nodes) != 1 || repositoryNetwork.Nodes[0].Login != "alice" {
		t.Fatalf("network leaked sibling-repository identities: %+v", repositoryNetwork.Nodes)
	}
	repositoryIdentities := manager.ScopedIdentities(query)
	if len(repositoryIdentities.Identities) != 1 || repositoryIdentities.Identities[0].Login != "alice" {
		t.Fatalf("identity view leaked sibling-repository identities: %+v", repositoryIdentities.Identities)
	}
}

func networkHasLogin(network NetworkResponse, login string) bool {
	for _, node := range network.Nodes {
		if node.Login == login {
			return true
		}
	}
	return false
}

func identitiesHaveLogin(response IdentityResponse, login string) bool {
	for _, identity := range response.Identities {
		if identity.Login == login {
			return true
		}
	}
	return false
}

func TestDeliveryQualityDirectionRefusesLowSamples(t *testing.T) {
	points := make([]DeliveryQualityPoint, 8)
	velocity := make([]DeliveryVelocityPoint, 8)
	for index := range points {
		failure, revert, review := .9, .8, .1
		if index >= 4 {
			failure, revert, review = .1, .1, .9
		}
		points[index] = DeliveryQualityPoint{Date: "period", ActionsFailureIncidence: &failure, ActionsPullSample: 1, RevertRate: &revert, CommitSample: 1, ReviewCoverage: &review, ReviewSample: 1}
		velocity[index] = DeliveryVelocityPoint{Complete: true}
	}
	direction, signals := deliveryQualityDirection(points, velocity)
	if direction != "insufficient" {
		t.Fatalf("low samples produced a confident quality verdict: %s %+v", direction, signals)
	}
}

func TestDeliveryQualityDirectionStates(t *testing.T) {
	tests := []struct {
		name                        string
		beforeFailure, afterFailure float64
		beforeReview, afterReview   float64
		want                        string
	}{
		{"improving", .8, .1, .2, .9, "improving"},
		{"declining", .1, .8, .9, .2, "declining"},
		{"mixed", .8, .1, .9, .2, "mixed"},
		{"stable", .4, .4, .6, .6, "stable/inconclusive"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			points := make([]DeliveryQualityPoint, 8)
			velocity := make([]DeliveryVelocityPoint, 8)
			for index := range points {
				failure, review := test.beforeFailure, test.beforeReview
				if index >= 4 {
					failure, review = test.afterFailure, test.afterReview
				}
				points[index] = DeliveryQualityPoint{ActionsFailureIncidence: &failure, ActionsPullSample: 3, ReviewCoverage: &review, ReviewSample: 3}
				velocity[index].Complete = true
			}
			direction, _ := deliveryQualityDirection(points, velocity)
			if direction != test.want {
				t.Fatalf("direction %q, want %q", direction, test.want)
			}
		})
	}
}

func TestDeliveryActionsFailureIncidenceCountsAttemptsAndCoveredPRs(t *testing.T) {
	merged := mustDeliveryDate("2025-02-10")
	pull := deliveryTestPull(7, merged, Person{Login: "human", Type: "User"}, 8, 2, 1)
	report := deliveryTestReport(1, 10, "Organization", []PullRequest{pull})
	report.Actions = &ActionsCache{Version: 1, Runs: []WorkflowRun{
		{ID: 100, Attempt: 1, Event: "pull_request", Conclusion: "failure", PullNumbers: []int64{7}},
		{ID: 100, Attempt: 2, Event: "pull_request", Conclusion: "success", PullNumbers: []int64{7}},
	}}
	manager := &Manager{reports: map[int64]RepositoryReport{1: report}, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{From: &merged, To: &merged})
	if err != nil {
		t.Fatal(err)
	}
	point := response.Quality.Points[0]
	if point.ActionsFailureIncidence == nil || *point.ActionsFailureIncidence != 1 || point.ActionsPullSample != 1 || point.FailedActionsAttempts != 1 || point.TotalActionsAttempts != 2 {
		t.Fatalf("unexpected GitHub Actions incidence: %+v", point)
	}
	if response.Meta.Coverage.ActionsCoveredPulls != 1 || response.Meta.Coverage.ActionsRuns != 2 {
		t.Fatalf("unexpected scoped Actions coverage: %+v", response.Meta.Coverage)
	}
}

func TestStoreReadsV2PullCacheButMarksDeliveryEnrichmentUnavailable(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	merged := mustDeliveryDate("2025-02-10")
	legacy := PullCache{Version: 2, Checkpoint: merged, PullRequests: []PullRequest{{
		Number: 1, State: "closed", CreatedAt: merged.Add(-time.Hour), MergedAt: &merged,
		Author: Person{Login: "human", Type: "User"},
	}}}
	if err := store.SavePullCache(1, legacy); err != nil {
		t.Fatal(err)
	}
	snapshot := &Snapshot{Repositories: []RepositorySummary{{ID: 1, Name: "repo", FullName: "org/repo", Owner: OwnerIdentity{ID: 10, Login: "org", Type: "Organization"}, PullRequests: &PullRequestTotals{Merged: 1}}}}
	reports, warnings := store.LoadReports(snapshot)
	if len(warnings) != 0 || reports[1].Pulls == nil || len(reports[1].Pulls.PullRequests) != 1 {
		t.Fatalf("v2 pull cache did not load compatibly: warnings=%v report=%+v", warnings, reports[1])
	}
	manager := &Manager{reports: reports, identityOverrides: map[string]IdentityOverride{}}
	response, err := manager.InsightDelivery(InsightQuery{From: &merged, To: &merged})
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(response.Meta.Unavailable, "pull_request_v3_enrichment_incomplete") || !containsString(response.Meta.Unavailable, "pull_request_commit_evidence_incomplete") {
		t.Fatalf("legacy enrichment gaps were not surfaced: %+v", response.Meta.Unavailable)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func deliveryTestPull(number int64, merged time.Time, author Person, additions, deletions, commits int) PullRequest {
	created := merged.Add(-24 * time.Hour)
	return PullRequest{Number: number, State: "closed", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged, Author: author, Additions: additions, Deletions: deletions, Commits: commits, DetailComplete: true, CommitEvidenceComplete: true, CommitEvidence: []PullRequestCommit{{SHA: "sha", Author: author}}}
}

func deliveryTestReport(id, organizationID int64, ownerType string, pulls []PullRequest) RepositoryReport {
	stats := BuildPullStats(pulls)
	return RepositoryReport{Repository: Repository{ID: id, Name: "repo", FullName: "org/repo", Owner: OwnerIdentity{ID: organizationID, Login: "org", Type: ownerType}}, Pulls: &stats}
}

func mustDeliveryDate(value string) time.Time {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		panic(err)
	}
	return date
}
