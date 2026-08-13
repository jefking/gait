package dashboard

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

const deliveryBootstrapSamples = 1000

type DeliveryCoverage struct {
	Organizations             int     `json:"organizations"`
	Repositories              int     `json:"repositories"`
	IndexEligibleRepositories int     `json:"index_eligible_repositories"`
	MergedPullRequests        int     `json:"merged_pull_requests"`
	AttributedPullRequests    int     `json:"attributed_pull_requests"`
	UnattributedPullRequests  int     `json:"unattributed_pull_requests"`
	DetailedPullRequests      int     `json:"detailed_pull_requests"`
	CompleteCommitEvidence    int     `json:"complete_commit_evidence_pull_requests"`
	TruncatedCommitEvidence   int     `json:"truncated_commit_evidence_pull_requests"`
	AttributionRate           float64 `json:"attribution_rate"`
	ActionsRuns               int     `json:"actions_runs"`
	ActionsCoveredPulls       int     `json:"actions_covered_pull_requests"`
	ActionsPermissionDenied   bool    `json:"actions_permission_denied"`
	ActionsTruncated          bool    `json:"actions_truncated"`
	DirectCommitCoverage      bool    `json:"direct_commit_coverage"`
	LowConfidenceBaseline     bool    `json:"low_confidence_baseline"`
}

type DeliveryScope struct {
	OrganizationID int64  `json:"organization_id,omitempty"`
	Organization   string `json:"organization,omitempty"`
	RepositoryID   int64  `json:"repository_id,omitempty"`
	Repository     string `json:"repository,omitempty"`
	ExcludeDead    bool   `json:"exclude_dead"`
}

type DeliveryMeta struct {
	AvailableFrom string              `json:"available_from,omitempty"`
	AvailableTo   string              `json:"available_to,omitempty"`
	From          string              `json:"from,omitempty"`
	To            string              `json:"to,omitempty"`
	Granularity   ActivityGranularity `json:"granularity,omitempty"`
	Scope         DeliveryScope       `json:"scope"`
	Coverage      DeliveryCoverage    `json:"coverage"`
	Unavailable   []string            `json:"unavailable,omitempty"`
}

type DeliveryRawMetrics struct {
	MergedPullRequests int `json:"merged_pull_requests"`
	Additions          int `json:"additions"`
	Deletions          int `json:"deletions"`
	ChangedLines       int `json:"changed_lines"`
	Commits            int `json:"commits"`
	DirectCommits      int `json:"direct_commits"`
}

type DeliveryModeMetrics struct {
	DeliveryRawMetrics
	Index float64 `json:"index"`
}

type DeliveryVelocityPoint struct {
	Date          string              `json:"date"`
	Human         DeliveryModeMetrics `json:"human"`
	Agent         DeliveryModeMetrics `json:"agent"`
	Collaborative DeliveryModeMetrics `json:"collaborative"`
	TotalIndex    float64             `json:"total_index"`
	Complete      bool                `json:"complete"`
}

type DeliveryRawTable struct {
	Human         DeliveryRawMetrics `json:"human"`
	Agent         DeliveryRawMetrics `json:"agent"`
	Collaborative DeliveryRawMetrics `json:"collaborative"`
	Total         DeliveryRawMetrics `json:"total"`
}

type DeliveryQualityPoint struct {
	Date                    string   `json:"date"`
	ActionsFailureIncidence *float64 `json:"actions_failure_incidence,omitempty"`
	ActionsPullSample       int      `json:"actions_pull_sample"`
	FailedActionsAttempts   int      `json:"failed_actions_attempts"`
	TotalActionsAttempts    int      `json:"total_actions_attempts"`
	RevertRate              *float64 `json:"revert_rate,omitempty"`
	CommitSample            int      `json:"commit_sample"`
	ReviewCoverage          *float64 `json:"review_coverage,omitempty"`
	ReviewSample            int      `json:"review_sample"`
	RetainedLineRate        *float64 `json:"retained_line_rate,omitempty"`
	RetentionSample         int      `json:"retention_sample"`
	MedianMergeHours        *float64 `json:"median_merge_hours,omitempty"`
	P90MergeHours           *float64 `json:"p90_merge_hours,omitempty"`
	MergeTimeSample         int      `json:"merge_time_sample"`
}

type DeliveryQualitySignal struct {
	Key       string   `json:"key"`
	Label     string   `json:"label"`
	Direction string   `json:"direction"`
	Delta     *float64 `json:"delta,omitempty"`
	Low       *float64 `json:"interval_low,omitempty"`
	High      *float64 `json:"interval_high,omitempty"`
	Sample    int      `json:"sample"`
}

type DeliveryQuality struct {
	Direction string                  `json:"direction"`
	Signals   []DeliveryQualitySignal `json:"signals"`
	Points    []DeliveryQualityPoint  `json:"points"`
}

type DeliveryFlowPoint struct {
	Date               string   `json:"date"`
	MergedPullRequests int      `json:"merged_pull_requests"`
	MedianChangedLines *float64 `json:"median_changed_lines,omitempty"`
	P90ChangedLines    *float64 `json:"p90_changed_lines,omitempty"`
	MedianCommits      *float64 `json:"median_commits,omitempty"`
	P90Commits         *float64 `json:"p90_commits,omitempty"`
	MedianAdditions    *float64 `json:"median_additions,omitempty"`
	P90Additions       *float64 `json:"p90_additions,omitempty"`
	MedianDeletions    *float64 `json:"median_deletions,omitempty"`
	P90Deletions       *float64 `json:"p90_deletions,omitempty"`
}

type DeliveryFlowSummary struct {
	AsOf                    string   `json:"as_of"`
	OpenPullRequests        int      `json:"open_pull_requests"`
	MedianOpenAgeDays       *float64 `json:"median_open_age_days,omitempty"`
	P90OpenAgeDays          *float64 `json:"p90_open_age_days,omitempty"`
	MedianChangedLines      *float64 `json:"median_changed_lines,omitempty"`
	P90ChangedLines         *float64 `json:"p90_changed_lines,omitempty"`
	MedianCommits           *float64 `json:"median_commits,omitempty"`
	P90Commits              *float64 `json:"p90_commits,omitempty"`
	MedianAdditions         *float64 `json:"median_additions,omitempty"`
	P90Additions            *float64 `json:"p90_additions,omitempty"`
	MedianDeletions         *float64 `json:"median_deletions,omitempty"`
	P90Deletions            *float64 `json:"p90_deletions,omitempty"`
	MergedPullRequestSample int      `json:"merged_pull_request_sample"`
}

type DeliveryFlow struct {
	Summary DeliveryFlowSummary `json:"summary"`
	Points  []DeliveryFlowPoint `json:"points"`
}

type DeliveryImpactQualityDelta struct {
	Key    string   `json:"key"`
	Delta  *float64 `json:"delta,omitempty"`
	Low    *float64 `json:"interval_low,omitempty"`
	High   *float64 `json:"interval_high,omitempty"`
	Sample int      `json:"sample"`
}

type DeliveryImpact struct {
	Tier                string                       `json:"tier"`
	Verdict             string                       `json:"verdict"`
	Estimate            *float64                     `json:"estimate,omitempty"`
	Low                 *float64                     `json:"interval_low,omitempty"`
	High                *float64                     `json:"interval_high,omitempty"`
	TreatedRepositories int                          `json:"treated_repositories"`
	ControlRepositories int                          `json:"control_repositories"`
	AdoptionCoverage    float64                      `json:"adoption_coverage"`
	PreWeeks            int                          `json:"pre_weeks"`
	PostWeeks           int                          `json:"post_weeks"`
	QualityDeltas       []DeliveryImpactQualityDelta `json:"quality_deltas"`
}

type DeliverySummary struct {
	Narrative            string   `json:"narrative"`
	VelocityVsBaseline   *float64 `json:"velocity_vs_baseline,omitempty"`
	AgentAssociatedShare float64  `json:"agent_associated_share"`
	QualityDirection     string   `json:"quality_direction"`
	Leader               string   `json:"leader"`
}

type DeliveryResponse struct {
	Meta     DeliveryMeta            `json:"meta"`
	Summary  DeliverySummary         `json:"summary"`
	Velocity []DeliveryVelocityPoint `json:"velocity"`
	Raw      DeliveryRawTable        `json:"raw"`
	Quality  DeliveryQuality         `json:"quality"`
	Flow     DeliveryFlow            `json:"flow"`
	Impact   DeliveryImpact          `json:"impact"`
}

type deliveryRepoData struct {
	report   RepositoryReport
	buckets  map[string]*deliveryBucket
	baseline deliveryBaseline
	adoption *time.Time
}

type deliveryBaseline struct {
	pulls float64
	lines float64
	valid bool
}

type deliveryBucket struct {
	modes   map[string]*DeliveryRawMetrics
	quality deliveryQualityAccumulator
	flow    deliveryFlowAccumulator
}

type deliveryQualityAccumulator struct {
	commits, reverts, reviewed, reviewSample int
	retained, added, retentionSample         int
	mergeHours                               []float64
	actionsPulls                             map[string]bool
	actionsFailed                            map[string]bool
	failedAttempts, totalAttempts            int
}

type deliveryFlowAccumulator struct {
	changed, commits, additions, deletions []float64
}

func (manager *Manager) InsightDelivery(query InsightQuery) (DeliveryResponse, error) {
	reports, overrides := manager.insightState()
	query, meta, err := prepareDeliveryQuery(reports, query)
	if err != nil {
		return DeliveryResponse{}, err
	}
	return buildDelivery(reports, overrides, query, meta), nil
}

func prepareDeliveryQuery(reports map[int64]RepositoryReport, query InsightQuery) (InsightQuery, DeliveryMeta, error) {
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return query, DeliveryMeta{}, errors.New("from must be on or before to")
	}
	var availableFrom, availableTo time.Time
	for _, report := range reports {
		if !deliveryReportMatches(report, query) {
			continue
		}
		if report.Pulls != nil {
			for _, pull := range report.Pulls.PullRequests {
				if pull.MergedAt == nil {
					continue
				}
				date := dayUTC(*pull.MergedAt)
				if availableFrom.IsZero() || date.Before(availableFrom) {
					availableFrom = date
				}
				if availableTo.IsZero() || date.After(availableTo) {
					availableTo = date
				}
			}
		}
	}
	meta := DeliveryMeta{Scope: DeliveryScope{OrganizationID: query.OwnerID, RepositoryID: query.RepositoryID, ExcludeDead: query.ExcludeDead}}
	if availableFrom.IsZero() {
		meta.Unavailable = []string{"merged_pull_request_evidence_unavailable"}
		return query, meta, nil
	}
	selectedFrom, selectedTo := availableFrom, availableTo
	if query.To != nil && query.To.Before(selectedTo) {
		selectedTo = dayUTC(*query.To)
	}
	if query.From != nil {
		if query.From.After(selectedFrom) {
			selectedFrom = dayUTC(*query.From)
		}
	} else {
		defaultFrom := addMonthsClamped(selectedTo, -defaultHistoryMonths)
		if defaultFrom.After(selectedFrom) {
			selectedFrom = defaultFrom
		}
	}
	if selectedFrom.After(selectedTo) {
		return query, DeliveryMeta{}, errors.New("selected date range does not overlap available merged pull requests")
	}
	query.From, query.To = &selectedFrom, &selectedTo
	meta.AvailableFrom, meta.AvailableTo = availableFrom.Format(time.DateOnly), availableTo.Format(time.DateOnly)
	meta.From, meta.To = selectedFrom.Format(time.DateOnly), selectedTo.Format(time.DateOnly)
	meta.Granularity = activityGranularity(selectedFrom, selectedTo)
	for _, report := range reports {
		if !deliveryReportMatches(report, query) {
			continue
		}
		if query.OwnerID != 0 && meta.Scope.Organization == "" {
			meta.Scope.Organization = report.Repository.Owner.Login
		}
		if query.RepositoryID != 0 && meta.Scope.Repository == "" {
			meta.Scope.Repository = report.Repository.FullName
			meta.Scope.OrganizationID = report.Repository.Owner.ID
			meta.Scope.Organization = report.Repository.Owner.Login
		}
	}
	return query, meta, nil
}

func deliveryReportMatches(report RepositoryReport, query InsightQuery) bool {
	return isOrganizationRepository(report.Repository) && insightReportMatches(report, query)
}

func buildDelivery(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery, meta DeliveryMeta) DeliveryResponse {
	response := DeliveryResponse{
		Meta:     meta,
		Velocity: []DeliveryVelocityPoint{},
		Quality: DeliveryQuality{
			Direction: "insufficient",
			Signals:   []DeliveryQualitySignal{},
			Points:    []DeliveryQualityPoint{},
		},
		Flow: DeliveryFlow{Points: []DeliveryFlowPoint{}},
		Impact: DeliveryImpact{
			Tier:          "insufficient_evidence",
			Verdict:       "insufficient evidence",
			PreWeeks:      8,
			PostWeeks:     8,
			QualityDeltas: []DeliveryImpactQualityDelta{},
		},
	}
	if query.From == nil || query.To == nil {
		response.Summary = buildDeliverySummary(response)
		return response
	}
	selectedReports := make(map[int64]RepositoryReport)
	organizations := make(map[int64]struct{})
	for id, report := range reports {
		if deliveryReportMatches(report, query) {
			selectedReports[id] = report
			organizations[report.Repository.Owner.ID] = struct{}{}
		}
	}
	response.Meta.Coverage.Organizations = len(organizations)
	response.Meta.Coverage.Repositories = len(selectedReports)
	response.Meta.Coverage.DirectCommitCoverage = true
	catalog := buildIdentityCatalog(selectedReports, overrides)
	aliases := identityAliasIndex(catalog)
	firstBucket := activityBucketStart(*query.From, meta.Granularity)
	lastBucket := activityBucketStart(*query.To, meta.Granularity)
	bucketDates := make([]time.Time, 0)
	for bucket := firstBucket; !bucket.After(lastBucket); bucket = nextActivityBucket(bucket, meta.Granularity) {
		bucketDates = append(bucketDates, bucket)
	}
	complete := deliveryCompleteBuckets(bucketDates, *query.From, *query.To, meta.Granularity)
	repositories := make([]*deliveryRepoData, 0, len(selectedReports))
	for _, report := range selectedReports {
		data := &deliveryRepoData{report: report, buckets: make(map[string]*deliveryBucket)}
		for _, date := range bucketDates {
			data.buckets[date.Format(time.DateOnly)] = newDeliveryBucket()
		}
		populateDeliveryRepository(data, catalog, aliases, overrides, query, &response.Meta.Coverage)
		data.baseline = calculateDeliveryBaseline(data.buckets, bucketDates, complete)
		if data.baseline.valid {
			response.Meta.Coverage.IndexEligibleRepositories++
		}
		repositories = append(repositories, data)
	}
	if response.Meta.Coverage.MergedPullRequests > 0 {
		response.Meta.Coverage.AttributionRate = float64(response.Meta.Coverage.AttributedPullRequests) / float64(response.Meta.Coverage.MergedPullRequests)
	}
	if response.Meta.Coverage.DetailedPullRequests < response.Meta.Coverage.MergedPullRequests {
		response.Meta.Unavailable = append(response.Meta.Unavailable, "pull_request_v3_enrichment_incomplete")
	}
	if response.Meta.Coverage.CompleteCommitEvidence+response.Meta.Coverage.TruncatedCommitEvidence < response.Meta.Coverage.MergedPullRequests {
		response.Meta.Unavailable = append(response.Meta.Unavailable, "pull_request_commit_evidence_incomplete")
	}
	if response.Meta.Coverage.ActionsPermissionDenied {
		response.Meta.Unavailable = append(response.Meta.Unavailable, "github_actions_permission_denied")
	} else if response.Meta.Coverage.MergedPullRequests > 0 && response.Meta.Coverage.ActionsCoveredPulls == 0 {
		response.Meta.Unavailable = append(response.Meta.Unavailable, "github_actions_evidence_unavailable")
	}
	if !response.Meta.Coverage.DirectCommitCoverage {
		response.Meta.Unavailable = append(response.Meta.Unavailable, "direct_commit_evidence_incomplete")
	}
	baselinePeriods := 0
	for _, date := range bucketDates {
		if complete[date.Format(time.DateOnly)] && baselinePeriods < 4 {
			baselinePeriods++
		}
	}
	response.Meta.Coverage.LowConfidenceBaseline = baselinePeriods < 4
	if response.Meta.Coverage.IndexEligibleRepositories == 0 {
		response.Meta.Unavailable = append(response.Meta.Unavailable, "velocity_index_requires_nonzero_opening_baseline")
	}
	for _, date := range bucketDates {
		key := date.Format(time.DateOnly)
		point := DeliveryVelocityPoint{Date: key, Complete: complete[key]}
		qualityAccumulator := deliveryQualityAccumulator{actionsPulls: make(map[string]bool), actionsFailed: make(map[string]bool)}
		flowAccumulator := deliveryFlowAccumulator{}
		for _, repository := range repositories {
			bucket := repository.buckets[key]
			addRawMetrics(&point.Human.DeliveryRawMetrics, *bucket.modes["human"])
			addRawMetrics(&point.Agent.DeliveryRawMetrics, *bucket.modes["agent"])
			addRawMetrics(&point.Collaborative.DeliveryRawMetrics, *bucket.modes["collaborative"])
			mergeQualityAccumulator(&qualityAccumulator, bucket.quality)
			mergeFlowAccumulator(&flowAccumulator, bucket.flow)
			if !repository.baseline.valid {
				continue
			}
			point.Human.Index += deliveryIndex(*bucket.modes["human"], repository.baseline)
			point.Agent.Index += deliveryIndex(*bucket.modes["agent"], repository.baseline)
			point.Collaborative.Index += deliveryIndex(*bucket.modes["collaborative"], repository.baseline)
		}
		eligible := float64(response.Meta.Coverage.IndexEligibleRepositories)
		if eligible > 0 {
			point.Human.Index /= eligible
			point.Agent.Index /= eligible
			point.Collaborative.Index /= eligible
		}
		point.TotalIndex = point.Human.Index + point.Agent.Index + point.Collaborative.Index
		response.Velocity = append(response.Velocity, point)
		response.Quality.Points = append(response.Quality.Points, qualityPoint(key, qualityAccumulator))
		response.Flow.Points = append(response.Flow.Points, flowPoint(key, flowAccumulator))
		addRawMetrics(&response.Raw.Human, point.Human.DeliveryRawMetrics)
		addRawMetrics(&response.Raw.Agent, point.Agent.DeliveryRawMetrics)
		addRawMetrics(&response.Raw.Collaborative, point.Collaborative.DeliveryRawMetrics)
	}
	addRawMetrics(&response.Raw.Total, response.Raw.Human)
	addRawMetrics(&response.Raw.Total, response.Raw.Agent)
	addRawMetrics(&response.Raw.Total, response.Raw.Collaborative)
	response.Quality.Direction, response.Quality.Signals = deliveryQualityDirection(response.Quality.Points, response.Velocity)
	response.Flow.Summary = deliveryFlowAt(repositories, *query.From, *query.To)
	response.Impact = buildDeliveryImpact(repositories, catalog, aliases, overrides, query)
	response.Summary = buildDeliverySummary(response)
	return response
}

func newDeliveryBucket() *deliveryBucket {
	return &deliveryBucket{
		modes:   map[string]*DeliveryRawMetrics{"human": {}, "agent": {}, "collaborative": {}},
		quality: deliveryQualityAccumulator{actionsPulls: make(map[string]bool), actionsFailed: make(map[string]bool)},
	}
}

func deliveryCompleteBuckets(dates []time.Time, from, to time.Time, granularity ActivityGranularity) map[string]bool {
	result := make(map[string]bool, len(dates))
	today := dayUTC(time.Now().UTC())
	selectedEnd := dayUTC(to).AddDate(0, 0, 1)
	if selectedEnd.After(today) {
		selectedEnd = today
	}
	for _, date := range dates {
		next := nextActivityBucket(date, granularity)
		result[date.Format(time.DateOnly)] = !date.Before(dayUTC(from)) && !next.After(selectedEnd)
	}
	return result
}

func populateDeliveryRepository(data *deliveryRepoData, catalog map[string]*resolvedIdentity, aliases map[string]*resolvedIdentity, overrides map[string]IdentityOverride, query InsightQuery, coverage *DeliveryCoverage) {
	prCommits := make(map[string]struct{})
	structuredCommitParticipants := make(map[string][]ContributorMetrics, len(data.report.Commits.Events))
	for _, event := range data.report.Commits.Events {
		if len(event.Participants) > 0 {
			structuredCommitParticipants[event.Hash] = event.Participants
		}
	}
	directCommitEvidenceComplete := true
	if data.report.Pulls != nil {
		for _, pull := range data.report.Pulls.PullRequests {
			for _, commit := range pull.CommitEvidence {
				prCommits[commit.SHA] = struct{}{}
			}
			if pull.MergedAt == nil {
				continue
			}
			mode := deliveryPullMode(pull, structuredCommitParticipants, catalog, aliases, overrides)
			if mode == "agent" || mode == "collaborative" {
				date := dayUTC(*pull.MergedAt)
				if data.adoption == nil || date.Before(*data.adoption) {
					data.adoption = &date
				}
			}
			if !pull.CommitEvidenceComplete {
				directCommitEvidenceComplete = false
			}
			if !inInsightRange(*pull.MergedAt, query) {
				continue
			}
			coverage.MergedPullRequests++
			if pull.DetailComplete {
				coverage.DetailedPullRequests++
			}
			if pull.CommitEvidenceComplete {
				coverage.CompleteCommitEvidence++
			} else if pull.Commits > len(pull.CommitEvidence) {
				coverage.TruncatedCommitEvidence++
			}
			if mode == "unattributed" {
				coverage.UnattributedPullRequests++
			} else {
				coverage.AttributedPullRequests++
			}
			bucket := data.buckets[activityBucketStart(*pull.MergedAt, activityGranularity(*query.From, *query.To)).Format(time.DateOnly)]
			if bucket == nil {
				continue
			}
			if mode != "unattributed" {
				raw := bucket.modes[mode]
				raw.MergedPullRequests++
				raw.Additions += pull.Additions
				raw.Deletions += pull.Deletions
				raw.ChangedLines += pull.Additions + pull.Deletions
				raw.Commits += pull.Commits
			}
			bucket.quality.reviewSample++
			if pullHasKnownReview(pull, catalog, overrides) {
				bucket.quality.reviewed++
			}
			bucket.quality.mergeHours = append(bucket.quality.mergeHours, pull.MergedAt.Sub(pull.CreatedAt).Hours())
			bucket.flow.changed = append(bucket.flow.changed, float64(pull.Additions+pull.Deletions))
			bucket.flow.commits = append(bucket.flow.commits, float64(pull.Commits))
			bucket.flow.additions = append(bucket.flow.additions, float64(pull.Additions))
			bucket.flow.deletions = append(bucket.flow.deletions, float64(pull.Deletions))
		}
	}
	coverage.DirectCommitCoverage = coverage.DirectCommitCoverage && data.report.Pulls != nil && directCommitEvidenceComplete
	granularity := activityGranularity(*query.From, *query.To)
	for _, event := range data.report.Commits.Events {
		if !inInsightRange(event.CommittedAt, query) {
			continue
		}
		bucket := data.buckets[activityBucketStart(event.CommittedAt, granularity).Format(time.DateOnly)]
		if bucket == nil {
			continue
		}
		bucket.quality.commits++
		if eventIsExplicitRevert(event) {
			bucket.quality.reverts++
		}
		if event.RetentionMeasured {
			bucket.quality.retentionSample++
			bucket.quality.retained += event.RetainedLines
			bucket.quality.added += event.LinesAdded
		}
		if _, belongsToPull := prCommits[event.Hash]; belongsToPull || !directCommitEvidenceComplete {
			continue
		}
		mode := deliveryIdentityMode(knownEventIdentities(event, catalog, aliases, overrides))
		if mode != "unattributed" {
			bucket.modes[mode].DirectCommits++
		}
	}
	populateActions(data, query, coverage)
}

func deliveryPullMode(pull PullRequest, structuredCommits map[string][]ContributorMetrics, catalog map[string]*resolvedIdentity, aliases map[string]*resolvedIdentity, overrides map[string]IdentityOverride) string {
	people := []ContributorMetrics{personMetrics(pull.Author)}
	for _, review := range pull.Reviews {
		if review.SubmittedAt == nil || pull.MergedAt != nil && review.SubmittedAt.After(*pull.MergedAt) {
			continue
		}
		people = append(people, personMetrics(review.Author))
	}
	for _, commit := range pull.CommitEvidence {
		if participants := structuredCommits[commit.SHA]; len(participants) > 0 {
			people = append(people, participants...)
		} else {
			people = append(people, personMetrics(commit.Author))
			people = append(people, commitCoauthors(commit.Message)...)
		}
	}
	identities := resolveDeliveryIdentities(people, catalog, aliases, overrides)
	return deliveryIdentityMode(knownIdentities(identities))
}

func pullHasKnownReview(pull PullRequest, catalog map[string]*resolvedIdentity, overrides map[string]IdentityOverride) bool {
	author := strings.ToLower(strings.TrimSpace(pull.Author.Login))
	for _, review := range pull.Reviews {
		if review.SubmittedAt == nil || pull.MergedAt != nil && review.SubmittedAt.After(*pull.MergedAt) || strings.EqualFold(strings.TrimSpace(review.Author.Login), author) {
			continue
		}
		identity := catalog[canonicalIdentityKey(personMetrics(review.Author).Key, overrides)]
		if isKnownIdentity(identity) {
			return true
		}
	}
	return false
}

func resolveDeliveryIdentities(people []ContributorMetrics, catalog map[string]*resolvedIdentity, aliases map[string]*resolvedIdentity, overrides map[string]IdentityOverride) []*resolvedIdentity {
	result := make([]*resolvedIdentity, 0, len(people))
	seen := make(map[string]struct{})
	for _, person := range people {
		if person.Key == "git:unknown" || person.Key == "github:" || person.Key == "" {
			continue
		}
		key := canonicalIdentityKey(person.Key, overrides)
		identity := catalog[key]
		if identity == nil {
			identity = aliases[person.Key]
		}
		if identity == nil {
			summary := classifyIdentity(person)
			summary.Key, summary.CanonicalKey = key, key
			applyIdentityOverride(&summary, overrides[person.Key])
			applyIdentityOverride(&summary, overrides[key])
			identity = &resolvedIdentity{IdentitySummary: summary}
		}
		if _, exists := seen[identity.Key]; exists {
			continue
		}
		seen[identity.Key] = struct{}{}
		result = append(result, identity)
	}
	return result
}

func deliveryIdentityMode(identities []*resolvedIdentity) string {
	switch workBucket(identities) {
	case "human_only":
		return "human"
	case "agent_only":
		return "agent"
	case "mixed":
		return "collaborative"
	default:
		return "unattributed"
	}
}

func populateActions(data *deliveryRepoData, query InsightQuery, coverage *DeliveryCoverage) {
	if data.report.Actions == nil {
		return
	}
	coverage.ActionsPermissionDenied = coverage.ActionsPermissionDenied || data.report.Actions.PermissionDenied
	coverage.ActionsTruncated = coverage.ActionsTruncated || data.report.Actions.Truncated
	byPull := make(map[int64][]WorkflowRun)
	for _, run := range data.report.Actions.Runs {
		if run.Event != "" && run.Event != "pull_request" && run.Event != "pull_request_target" {
			continue
		}
		for _, number := range run.PullNumbers {
			byPull[number] = append(byPull[number], run)
		}
	}
	if data.report.Pulls == nil {
		return
	}
	granularity := activityGranularity(*query.From, *query.To)
	coveredRuns := make(map[string]struct{})
	for _, pull := range data.report.Pulls.PullRequests {
		if pull.MergedAt == nil || !inInsightRange(*pull.MergedAt, query) {
			continue
		}
		bucket := data.buckets[activityBucketStart(*pull.MergedAt, granularity).Format(time.DateOnly)]
		conclusive, failed := false, false
		for _, run := range byPull[pull.Number] {
			if !isConclusiveAction(run.Conclusion) {
				continue
			}
			conclusive = true
			runKey := fmt.Sprintf("%d:%d", run.ID, run.Attempt)
			if _, exists := coveredRuns[runKey]; !exists {
				coveredRuns[runKey] = struct{}{}
				coverage.ActionsRuns++
			}
			bucket.quality.totalAttempts++
			if isFailedAction(run.Conclusion) {
				failed = true
				bucket.quality.failedAttempts++
			}
		}
		if conclusive {
			coverage.ActionsCoveredPulls++
			key := fmt.Sprintf("%d:%d", data.report.Repository.ID, pull.Number)
			bucket.quality.actionsPulls[key] = true
			if failed {
				bucket.quality.actionsFailed[key] = true
			}
		}
	}
}

func isConclusiveAction(conclusion string) bool {
	return strings.TrimSpace(conclusion) != ""
}

func isFailedAction(conclusion string) bool {
	switch strings.ToLower(strings.TrimSpace(conclusion)) {
	case "failure", "timed_out", "action_required", "startup_failure", "stale":
		return true
	default:
		return false
	}
}

func calculateDeliveryBaseline(buckets map[string]*deliveryBucket, dates []time.Time, complete map[string]bool) deliveryBaseline {
	periods, pulls, lines := 0, 0, 0
	for _, date := range dates {
		key := date.Format(time.DateOnly)
		if !complete[key] || periods >= 4 {
			continue
		}
		periods++
		for _, mode := range []string{"human", "agent", "collaborative"} {
			pulls += buckets[key].modes[mode].MergedPullRequests
			lines += buckets[key].modes[mode].ChangedLines
		}
	}
	if periods == 0 || pulls == 0 && lines == 0 {
		return deliveryBaseline{}
	}
	return deliveryBaseline{pulls: float64(pulls) / float64(periods), lines: float64(lines) / float64(periods), valid: true}
}

func deliveryIndex(raw DeliveryRawMetrics, baseline deliveryBaseline) float64 {
	if !baseline.valid {
		return 0
	}
	if baseline.pulls == 0 {
		return 100 * float64(raw.ChangedLines) / baseline.lines
	}
	if baseline.lines == 0 {
		return 100 * float64(raw.MergedPullRequests) / baseline.pulls
	}
	return 100 * (0.5*float64(raw.MergedPullRequests)/baseline.pulls + 0.5*float64(raw.ChangedLines)/baseline.lines)
}

func addRawMetrics(target *DeliveryRawMetrics, source DeliveryRawMetrics) {
	target.MergedPullRequests += source.MergedPullRequests
	target.Additions += source.Additions
	target.Deletions += source.Deletions
	target.ChangedLines += source.ChangedLines
	target.Commits += source.Commits
	target.DirectCommits += source.DirectCommits
}

func mergeQualityAccumulator(target *deliveryQualityAccumulator, source deliveryQualityAccumulator) {
	target.commits += source.commits
	target.reverts += source.reverts
	target.reviewed += source.reviewed
	target.reviewSample += source.reviewSample
	target.retained += source.retained
	target.added += source.added
	target.retentionSample += source.retentionSample
	target.mergeHours = append(target.mergeHours, source.mergeHours...)
	target.failedAttempts += source.failedAttempts
	target.totalAttempts += source.totalAttempts
	for pull := range source.actionsPulls {
		target.actionsPulls[pull] = true
		if source.actionsFailed[pull] {
			target.actionsFailed[pull] = true
		}
	}
}

func mergeFlowAccumulator(target *deliveryFlowAccumulator, source deliveryFlowAccumulator) {
	target.changed = append(target.changed, source.changed...)
	target.commits = append(target.commits, source.commits...)
	target.additions = append(target.additions, source.additions...)
	target.deletions = append(target.deletions, source.deletions...)
}

func qualityPoint(key string, value deliveryQualityAccumulator) DeliveryQualityPoint {
	point := DeliveryQualityPoint{Date: key, CommitSample: value.commits, ReviewSample: value.reviewSample, RetentionSample: value.retentionSample, MergeTimeSample: len(value.mergeHours), ActionsPullSample: len(value.actionsPulls), FailedActionsAttempts: value.failedAttempts, TotalActionsAttempts: value.totalAttempts}
	if len(value.actionsPulls) > 0 {
		failure := float64(len(value.actionsFailed)) / float64(len(value.actionsPulls))
		point.ActionsFailureIncidence = &failure
	}
	if value.commits > 0 {
		rate := float64(value.reverts) / float64(value.commits)
		point.RevertRate = &rate
	}
	if value.reviewSample > 0 {
		rate := float64(value.reviewed) / float64(value.reviewSample)
		point.ReviewCoverage = &rate
	}
	if value.retentionSample > 0 && value.added > 0 {
		rate := float64(value.retained) / float64(value.added)
		point.RetainedLineRate = &rate
	}
	if len(value.mergeHours) > 0 {
		medianValue, p90 := percentile(value.mergeHours, .5), percentile(value.mergeHours, .9)
		point.MedianMergeHours, point.P90MergeHours = &medianValue, &p90
	}
	return point
}

func flowPoint(key string, value deliveryFlowAccumulator) DeliveryFlowPoint {
	point := DeliveryFlowPoint{Date: key, MergedPullRequests: len(value.changed)}
	point.MedianChangedLines, point.P90ChangedLines = percentilePointers(value.changed)
	point.MedianCommits, point.P90Commits = percentilePointers(value.commits)
	point.MedianAdditions, point.P90Additions = percentilePointers(value.additions)
	point.MedianDeletions, point.P90Deletions = percentilePointers(value.deletions)
	return point
}

func percentilePointers(values []float64) (*float64, *float64) {
	if len(values) == 0 {
		return nil, nil
	}
	p50, p90 := percentile(values, .5), percentile(values, .9)
	return &p50, &p90
}

func percentile(values []float64, quantile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	position := quantile * float64(len(ordered)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return ordered[lower]
	}
	return ordered[lower] + (ordered[upper]-ordered[lower])*(position-float64(lower))
}

func deliveryFlowAt(repositories []*deliveryRepoData, from, end time.Time) DeliveryFlowSummary {
	result := DeliveryFlowSummary{AsOf: end.Format(time.DateOnly)}
	ages, changed, commits, additions, deletions := []float64{}, []float64{}, []float64{}, []float64{}, []float64{}
	for _, repository := range repositories {
		if repository.report.Pulls == nil {
			continue
		}
		for _, pull := range repository.report.Pulls.PullRequests {
			if pull.CreatedAt.After(end.AddDate(0, 0, 1)) {
				continue
			}
			resolvedAt := pull.ClosedAt
			if pull.MergedAt != nil {
				resolvedAt = pull.MergedAt
			}
			if resolvedAt == nil || resolvedAt.After(end.AddDate(0, 0, 1)) {
				result.OpenPullRequests++
				ages = append(ages, end.Sub(dayUTC(pull.CreatedAt)).Hours()/24)
			}
			if pull.MergedAt != nil && !dayUTC(*pull.MergedAt).Before(from) && !dayUTC(*pull.MergedAt).After(end) {
				changed = append(changed, float64(pull.Additions+pull.Deletions))
				commits = append(commits, float64(pull.Commits))
				additions = append(additions, float64(pull.Additions))
				deletions = append(deletions, float64(pull.Deletions))
			}
		}
	}
	result.MedianOpenAgeDays, result.P90OpenAgeDays = percentilePointers(ages)
	result.MedianChangedLines, result.P90ChangedLines = percentilePointers(changed)
	result.MedianCommits, result.P90Commits = percentilePointers(commits)
	result.MedianAdditions, result.P90Additions = percentilePointers(additions)
	result.MedianDeletions, result.P90Deletions = percentilePointers(deletions)
	result.MergedPullRequestSample = len(changed)
	return result
}

type qualitySeriesSpec struct {
	key, label    string
	lowerBetter   bool
	minimumSample int
	value         func(DeliveryQualityPoint) (*float64, int)
}

func deliveryQualityDirection(points []DeliveryQualityPoint, velocity []DeliveryVelocityPoint) (string, []DeliveryQualitySignal) {
	complete := make([]DeliveryQualityPoint, 0, len(points))
	for index, point := range points {
		if index < len(velocity) && velocity[index].Complete {
			complete = append(complete, point)
		}
	}
	specs := []qualitySeriesSpec{
		{key: "actions_failure_incidence", label: "GitHub Actions failure incidence", lowerBetter: true, minimumSample: 20, value: func(p DeliveryQualityPoint) (*float64, int) { return p.ActionsFailureIncidence, p.ActionsPullSample }},
		{key: "revert_rate", label: "Revert rate", lowerBetter: true, minimumSample: 20, value: func(p DeliveryQualityPoint) (*float64, int) { return p.RevertRate, p.CommitSample }},
		{key: "review_coverage", label: "Review coverage", minimumSample: 20, value: func(p DeliveryQualityPoint) (*float64, int) { return p.ReviewCoverage, p.ReviewSample }},
		{key: "retained_line_rate", label: "30-day retained-line rate", minimumSample: 20, value: func(p DeliveryQualityPoint) (*float64, int) { return p.RetainedLineRate, p.RetentionSample }},
		{key: "median_merge_hours", label: "Median merge time", lowerBetter: true, minimumSample: 16, value: func(p DeliveryQualityPoint) (*float64, int) { return p.MedianMergeHours, p.MergeTimeSample }},
	}
	signals := make([]DeliveryQualitySignal, 0, len(specs))
	improving, declining, eligible := 0, 0, 0
	for specIndex, spec := range specs {
		signal := DeliveryQualitySignal{Key: spec.key, Label: spec.label, Direction: "insufficient"}
		if len(complete) >= 8 {
			first, last := []float64{}, []float64{}
			sample := 0
			for _, point := range complete[:4] {
				value, n := spec.value(point)
				if value != nil && n > 0 {
					first = append(first, *value)
					sample += n
				}
			}
			for _, point := range complete[len(complete)-4:] {
				value, n := spec.value(point)
				if value != nil && n > 0 {
					last = append(last, *value)
					sample += n
				}
			}
			signal.Sample = sample
			if len(first) == 4 && len(last) == 4 && sample >= spec.minimumSample {
				delta, low, high := bootstrapDifference(first, last, spec.lowerBetter, uint64(4100+specIndex))
				signal.Delta, signal.Low, signal.High = &delta, &low, &high
				signal.Direction = "inconclusive"
				eligible++
				if low > 0 {
					signal.Direction = "improving"
					improving++
				} else if high < 0 {
					signal.Direction = "declining"
					declining++
				}
			}
		}
		signals = append(signals, signal)
	}
	if eligible < 2 {
		return "insufficient", signals
	}
	if improving > 0 && declining > 0 {
		return "mixed", signals
	}
	if improving >= 2 && declining == 0 {
		return "improving", signals
	}
	if declining >= 2 && improving == 0 {
		return "declining", signals
	}
	return "stable/inconclusive", signals
}

func bootstrapDifference(first, last []float64, lowerBetter bool, seed uint64) (float64, float64, float64) {
	direction := 1.0
	if lowerBetter {
		direction = -1
	}
	observed := direction * (mean(last) - mean(first))
	random := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	samples := make([]float64, deliveryBootstrapSamples)
	for index := range samples {
		before, after := 0.0, 0.0
		for range len(first) {
			before += first[random.IntN(len(first))]
		}
		for range len(last) {
			after += last[random.IntN(len(last))]
		}
		samples[index] = direction * (after/float64(len(last)) - before/float64(len(first)))
	}
	return observed, percentile(samples, .025), percentile(samples, .975)
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func buildDeliverySummary(response DeliveryResponse) DeliverySummary {
	summary := DeliverySummary{QualityDirection: response.Quality.Direction, Leader: "No clear leader"}
	complete := make([]DeliveryVelocityPoint, 0, len(response.Velocity))
	totals := map[string]float64{"Human": 0, "Agent": 0, "Collaborative": 0}
	for _, point := range response.Velocity {
		if point.Complete {
			complete = append(complete, point)
		}
		totals["Human"] += point.Human.Index
		totals["Agent"] += point.Agent.Index
		totals["Collaborative"] += point.Collaborative.Index
	}
	if len(complete) > 0 {
		start := max(0, len(complete)-3)
		values := make([]float64, 0, len(complete)-start)
		for _, point := range complete[start:] {
			values = append(values, point.TotalIndex)
		}
		delta := mean(values)/100 - 1
		summary.VelocityVsBaseline = &delta
	}
	totalIndex := totals["Human"] + totals["Agent"] + totals["Collaborative"]
	if totalIndex > 0 {
		summary.AgentAssociatedShare = (totals["Agent"] + totals["Collaborative"]) / totalIndex
	}
	maxValue := math.Inf(-1)
	tied := false
	for _, label := range []string{"Human", "Agent", "Collaborative"} {
		value := totals[label]
		if value > maxValue+1e-9 {
			maxValue, summary.Leader, tied = value, label, false
		} else if math.Abs(value-maxValue) <= 1e-9 {
			tied = true
		}
	}
	if tied || maxValue <= 0 {
		summary.Leader = "No clear leader"
	}
	velocitySentence := "Team velocity is unavailable until a complete opening baseline exists"
	if summary.VelocityVsBaseline != nil {
		velocitySentence = fmt.Sprintf("Team velocity is %.0f%% %s its opening pace; agents participate in %.0f%% of indexed shipped work", math.Abs(*summary.VelocityVsBaseline)*100, map[bool]string{true: "above", false: "below"}[*summary.VelocityVsBaseline >= 0], summary.AgentAssociatedShare*100)
	}
	qualitySentence := "Quality evidence is insufficient"
	switch summary.QualityDirection {
	case "improving":
		qualitySentence = "Quality is improving across the eligible guardrails"
	case "declining":
		qualitySentence = "Quality is declining across the eligible guardrails"
	case "mixed":
		qualitySentence = "Quality is mixed: some guardrails improved while others declined"
	case "stable/inconclusive":
		qualitySentence = "Quality is stable or inconclusive at the available sample size"
	}
	summary.Narrative = velocitySentence + ". " + qualitySentence + "."
	return summary
}

func buildDeliveryImpact(repositories []*deliveryRepoData, catalog map[string]*resolvedIdentity, aliases map[string]*resolvedIdentity, overrides map[string]IdentityOverride, query InsightQuery) DeliveryImpact {
	result := DeliveryImpact{Tier: "insufficient_evidence", Verdict: "insufficient evidence", PreWeeks: 8, PostWeeks: 8, QualityDeltas: []DeliveryImpactQualityDelta{}}
	adopted := 0
	treated := make([]impactRepositoryEffect, 0)
	for _, repository := range repositories {
		if repository.adoption == nil {
			continue
		}
		adopted++
		adoption := activityBucketStart(*repository.adoption, ActivityByWeek)
		preStart, postEnd := adoption.AddDate(0, 0, -56), adoption.AddDate(0, 0, 63)
		if preStart.Before(*query.From) || postEnd.After(query.To.AddDate(0, 0, 1)) {
			continue
		}
		effect, ok := deliveryImpactEffect(repository, catalog, aliases, overrides, preStart, adoption, adoption.AddDate(0, 0, 7), postEnd)
		if !ok {
			continue
		}
		effect.adoption = adoption
		treated = append(treated, effect)
	}
	if len(repositories) > 0 {
		result.AdoptionCoverage = float64(adopted) / float64(len(repositories))
	}
	result.TreatedRepositories = len(treated)
	if len(treated) == 0 {
		return result
	}

	controlledEffects := make([]impactRepositoryEffect, 0, len(treated))
	usedControls := make(map[int64]struct{})
	if len(treated) >= 3 {
		for _, treatment := range treated {
			matches := make([]impactRepositoryEffect, 0)
			for _, candidate := range repositories {
				if candidate.adoption != nil || candidate.report.Repository.Owner.ID != treatment.repository.report.Repository.Owner.ID || candidate.report.Repository.ID == treatment.repository.report.Repository.ID {
					continue
				}
				preStart, postEnd := treatment.adoption.AddDate(0, 0, -56), treatment.adoption.AddDate(0, 0, 63)
				control, ok := deliveryImpactEffect(candidate, catalog, aliases, overrides, preStart, treatment.adoption, treatment.adoption.AddDate(0, 0, 7), postEnd)
				if !ok {
					continue
				}
				control.distance = math.Abs(math.Log1p(control.prePulls)-math.Log1p(treatment.prePulls)) + math.Abs(math.Log1p(control.preLines)-math.Log1p(treatment.preLines))
				matches = append(matches, control)
			}
			sort.Slice(matches, func(i, j int) bool {
				if matches[i].distance == matches[j].distance {
					return matches[i].repository.report.Repository.ID < matches[j].repository.report.Repository.ID
				}
				return matches[i].distance < matches[j].distance
			})
			if len(matches) < 2 {
				controlledEffects = nil
				break
			}
			matches = matches[:2]
			adjusted := treatment
			adjusted.effect -= (matches[0].effect + matches[1].effect) / 2
			adjusted.quality = adjustedQualityEffects(treatment.quality, matches)
			controlledEffects = append(controlledEffects, adjusted)
			usedControls[matches[0].repository.report.Repository.ID] = struct{}{}
			usedControls[matches[1].repository.report.Repository.ID] = struct{}{}
		}
	}
	if len(controlledEffects) == len(treated) && len(controlledEffects) >= 3 {
		values := impactValues(controlledEffects)
		estimate, low, high := bootstrapMean(values, 8701)
		result.Tier = "matched_difference_in_differences"
		result.Verdict = "inconclusive"
		if low > 0 {
			result.Verdict = "supported increase"
		} else if high < 0 {
			result.Verdict = "supported decrease"
		}
		result.Estimate, result.Low, result.High = &estimate, &low, &high
		result.ControlRepositories = len(usedControls)
		result.QualityDeltas = impactQualityDeltas(controlledEffects)
		return result
	}
	result.Tier = "paired_pre_post"
	result.Verdict = "observed pre/post association"
	estimate, low, high := bootstrapMean(impactValues(treated), 8701)
	result.Estimate, result.Low, result.High = &estimate, &low, &high
	result.QualityDeltas = impactQualityDeltas(treated)
	return result
}

type impactRepositoryEffect struct {
	repository *deliveryRepoData
	adoption   time.Time
	effect     float64
	prePulls   float64
	preLines   float64
	distance   float64
	quality    map[string]float64
}

func deliveryImpactEffect(repository *deliveryRepoData, catalog map[string]*resolvedIdentity, aliases map[string]*resolvedIdentity, overrides map[string]IdentityOverride, preStart, preEnd, postStart, postEnd time.Time) (impactRepositoryEffect, bool) {
	weekly := make(map[string]DeliveryRawMetrics)
	structuredCommitParticipants := make(map[string][]ContributorMetrics, len(repository.report.Commits.Events))
	for _, event := range repository.report.Commits.Events {
		if len(event.Participants) > 0 {
			structuredCommitParticipants[event.Hash] = event.Participants
		}
	}
	if repository.report.Pulls != nil {
		for _, pull := range repository.report.Pulls.PullRequests {
			if pull.MergedAt == nil || pull.MergedAt.Before(preStart) || !pull.MergedAt.Before(postEnd) || deliveryPullMode(pull, structuredCommitParticipants, catalog, aliases, overrides) == "unattributed" {
				continue
			}
			key := activityBucketStart(*pull.MergedAt, ActivityByWeek).Format(time.DateOnly)
			raw := weekly[key]
			raw.MergedPullRequests++
			raw.ChangedLines += pull.Additions + pull.Deletions
			weekly[key] = raw
		}
	}
	collect := func(start, end time.Time) []DeliveryRawMetrics {
		values := make([]DeliveryRawMetrics, 0, 8)
		for week := start; week.Before(end); week = week.AddDate(0, 0, 7) {
			values = append(values, weekly[week.Format(time.DateOnly)])
		}
		return values
	}
	before, after := collect(preStart, preEnd), collect(postStart, postEnd)
	if len(before) != 8 || len(after) != 8 {
		return impactRepositoryEffect{}, false
	}
	pulls, lines := 0, 0
	for _, raw := range before {
		pulls += raw.MergedPullRequests
		lines += raw.ChangedLines
	}
	baseline := deliveryBaseline{pulls: float64(pulls) / 8, lines: float64(lines) / 8, valid: pulls > 0 || lines > 0}
	if !baseline.valid {
		return impactRepositoryEffect{}, false
	}
	postIndices := make([]float64, 0, len(after))
	for _, raw := range after {
		postIndices = append(postIndices, deliveryIndex(raw, baseline))
	}
	quality := impactQualityChange(repository.report, catalog, overrides, preStart, preEnd, postStart, postEnd)
	return impactRepositoryEffect{repository: repository, effect: mean(postIndices) - 100, prePulls: baseline.pulls, preLines: baseline.lines, quality: quality}, true
}

func impactQualityChange(report RepositoryReport, catalog map[string]*resolvedIdentity, overrides map[string]IdentityOverride, preStart, preEnd, postStart, postEnd time.Time) map[string]float64 {
	before := impactQualityWindow(report, catalog, overrides, preStart, preEnd)
	after := impactQualityWindow(report, catalog, overrides, postStart, postEnd)
	result := make(map[string]float64)
	for key, value := range before {
		if next, exists := after[key]; exists {
			result[key] = next - value
		}
	}
	return result
}

func impactQualityWindow(report RepositoryReport, catalog map[string]*resolvedIdentity, overrides map[string]IdentityOverride, start, end time.Time) map[string]float64 {
	commits, reverts, retained, added := 0, 0, 0, 0
	mergeHours := make([]float64, 0)
	reviewed, pulls := 0, 0
	for _, event := range report.Commits.Events {
		if event.CommittedAt.Before(start) || !event.CommittedAt.Before(end) {
			continue
		}
		commits++
		if eventIsExplicitRevert(event) {
			reverts++
		}
		if event.RetentionMeasured {
			retained += event.RetainedLines
			added += event.LinesAdded
		}
	}
	actionsByPull := make(map[int64][]WorkflowRun)
	if report.Actions != nil {
		for _, run := range report.Actions.Runs {
			for _, number := range run.PullNumbers {
				actionsByPull[number] = append(actionsByPull[number], run)
			}
		}
	}
	actionsPulls, failedPulls := 0, 0
	if report.Pulls != nil {
		for _, pull := range report.Pulls.PullRequests {
			if pull.MergedAt == nil || pull.MergedAt.Before(start) || !pull.MergedAt.Before(end) {
				continue
			}
			pulls++
			mergeHours = append(mergeHours, pull.MergedAt.Sub(pull.CreatedAt).Hours())
			if pullHasKnownReview(pull, catalog, overrides) {
				reviewed++
			}
			conclusive, failed := false, false
			for _, run := range actionsByPull[pull.Number] {
				conclusive = conclusive || isConclusiveAction(run.Conclusion)
				failed = failed || isFailedAction(run.Conclusion)
			}
			if conclusive {
				actionsPulls++
				if failed {
					failedPulls++
				}
			}
		}
	}
	result := make(map[string]float64)
	if actionsPulls > 0 {
		result["actions_failure_incidence"] = float64(failedPulls) / float64(actionsPulls)
	}
	if commits > 0 {
		result["revert_rate"] = float64(reverts) / float64(commits)
	}
	if pulls > 0 {
		result["review_coverage"] = float64(reviewed) / float64(pulls)
	}
	if added > 0 {
		result["retained_line_rate"] = float64(retained) / float64(added)
	}
	if len(mergeHours) > 0 {
		result["median_merge_hours"] = median(mergeHours)
	}
	return result
}

func adjustedQualityEffects(treatment map[string]float64, controls []impactRepositoryEffect) map[string]float64 {
	result := make(map[string]float64)
	for key, value := range treatment {
		left, leftOK := controls[0].quality[key]
		right, rightOK := controls[1].quality[key]
		if leftOK && rightOK {
			result[key] = value - (left+right)/2
		}
	}
	return result
}

func impactValues(effects []impactRepositoryEffect) []float64 {
	values := make([]float64, 0, len(effects))
	for _, effect := range effects {
		values = append(values, effect.effect)
	}
	return values
}

func impactQualityDeltas(effects []impactRepositoryEffect) []DeliveryImpactQualityDelta {
	byKey := make(map[string][]float64)
	for _, effect := range effects {
		for key, value := range effect.quality {
			byKey[key] = append(byKey[key], value)
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]DeliveryImpactQualityDelta, 0, len(keys))
	for index, key := range keys {
		values := byKey[key]
		if len(values) < 2 {
			continue
		}
		delta, low, high := bootstrapMean(values, uint64(9300+index))
		result = append(result, DeliveryImpactQualityDelta{Key: key, Delta: &delta, Low: &low, High: &high, Sample: len(values)})
	}
	return result
}

func bootstrapMean(values []float64, seed uint64) (float64, float64, float64) {
	random := rand.New(rand.NewPCG(seed, seed^0xa0761d6478bd642f))
	samples := make([]float64, deliveryBootstrapSamples)
	for index := range samples {
		total := 0.0
		for range len(values) {
			total += values[random.IntN(len(values))]
		}
		samples[index] = total / float64(len(values))
	}
	return mean(values), percentile(samples, .025), percentile(samples, .975)
}
