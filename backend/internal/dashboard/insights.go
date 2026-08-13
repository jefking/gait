package dashboard

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultSessionHours  = 72
	defaultWindowDays    = 30
	defaultHistoryMonths = 6
	maximumNetworkNodes  = 75
)

type ActorKind string

const (
	ActorHuman   ActorKind = "human"
	ActorAgent   ActorKind = "agent"
	ActorUnknown ActorKind = "unknown"
)

type IdentityOverride struct {
	Kind         ActorKind `json:"kind,omitempty"`
	DisplayName  string    `json:"display_name,omitempty"`
	CanonicalKey string    `json:"canonical_key,omitempty"`
	Unmerge      bool      `json:"unmerge,omitempty"`
}

type IdentitySummary struct {
	Key          string    `json:"key"`
	CanonicalKey string    `json:"canonical_key"`
	Name         string    `json:"name"`
	Login        string    `json:"login,omitempty"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Kind         ActorKind `json:"kind"`
	Evidence     string    `json:"evidence"`
	Confidence   string    `json:"confidence"`
	Aliases      []string  `json:"aliases,omitempty"`
	Commits      int       `json:"commits"`
	PullRequests int       `json:"pull_requests"`
	Reviews      int       `json:"reviews"`
}

type IdentityResponse struct {
	Identities []IdentitySummary `json:"identities"`
}

type InsightQuery struct {
	OwnerID        int64
	RepositoryID   int64
	ActorKind      ActorKind
	ExcludeDead    bool
	From           *time.Time
	To             *time.Time
	SessionHours   int
	AdoptionDays   int
	SurvivalDays   int
	Cohort         string
	Metric         string
	maturityCutoff time.Time
}

type InsightCoverage struct {
	TotalCommits       int     `json:"total_commits"`
	ClassifiedCommits  int     `json:"classified_commits"`
	UnknownCommits     int     `json:"unknown_commits"`
	ClassificationRate float64 `json:"classification_rate"`
	MatureCommits      int     `json:"mature_commits"`
	EligiblePulls      int     `json:"eligible_pull_requests"`
	ReviewedPulls      int     `json:"reviewed_pull_requests"`
}

type InsightMeta struct {
	AvailableFrom string              `json:"available_from,omitempty"`
	AvailableTo   string              `json:"available_to,omitempty"`
	From          string              `json:"from,omitempty"`
	To            string              `json:"to,omitempty"`
	Granularity   ActivityGranularity `json:"granularity,omitempty"`
	SessionHours  int                 `json:"session_hours"`
	AdoptionDays  int                 `json:"adoption_days"`
	SurvivalDays  int                 `json:"survival_days"`
	Coverage      InsightCoverage     `json:"coverage"`
	Unavailable   []string            `json:"unavailable,omitempty"`
	Truncated     bool                `json:"truncated,omitempty"`
	TotalResults  int                 `json:"total_results,omitempty"`
}

type InsightSummary struct {
	AgentParticipation float64  `json:"agent_participation"`
	HandoffLift        *float64 `json:"handoff_lift,omitempty"`
	HandoffEpisodes    int      `json:"handoff_episodes"`
	QualityDirection   *float64 `json:"quality_direction,omitempty"`
	StrongestPair      string   `json:"strongest_pair,omitempty"`
	StrongestPairDays  int      `json:"strongest_pair_days"`
}

type TimelinePoint struct {
	Date      string `json:"date"`
	HumanOnly int    `json:"human_only"`
	AgentOnly int    `json:"agent_only"`
	Mixed     int    `json:"mixed"`
	Unknown   int    `json:"unknown"`
	Pulls     int    `json:"pull_requests"`
}

type QualityPoint struct {
	Date              string   `json:"date"`
	RevertRate        *float64 `json:"revert_rate,omitempty"`
	MergeRate         *float64 `json:"merge_rate,omitempty"`
	MedianMergeHours  *float64 `json:"median_merge_hours,omitempty"`
	ReviewCoverage    *float64 `json:"review_coverage,omitempty"`
	RetainedLineRate  *float64 `json:"retained_line_rate,omitempty"`
	CommitSample      int      `json:"commit_sample"`
	PullRequestSample int      `json:"pull_request_sample"`
	RetentionSample   int      `json:"retention_sample"`
}

type RepositoryPulse struct {
	RepositoryID int64        `json:"repository_id"`
	Name         string       `json:"name"`
	Total        int          `json:"total"`
	Points       []PulsePoint `json:"points"`
}

type PulsePoint struct {
	Date      string `json:"date"`
	HumanOnly int    `json:"human_only"`
	AgentOnly int    `json:"agent_only"`
	Mixed     int    `json:"mixed"`
	Unknown   int    `json:"unknown"`
}

type OverviewResponse struct {
	Meta         InsightMeta       `json:"meta"`
	Summary      InsightSummary    `json:"summary"`
	Timeline     []TimelinePoint   `json:"timeline"`
	Quality      []QualityPoint    `json:"quality"`
	Repositories []RepositoryPulse `json:"repositories"`
}

type NetworkNode struct {
	IdentitySummary
	Activity int `json:"activity"`
}

type NetworkEdge struct {
	Source             string   `json:"source"`
	Target             string   `json:"target"`
	PairType           string   `json:"pair_type"`
	InteractionDays    int      `json:"interaction_days"`
	Coauthorships      int      `json:"coauthorships"`
	ReviewInteractions int      `json:"review_interactions"`
	Handoffs           int      `json:"handoffs"`
	HumanToAgent       int      `json:"human_to_agent"`
	Repositories       []string `json:"repositories"`
	Periods            []string `json:"periods,omitempty"`
}

type NetworkResponse struct {
	Meta            InsightMeta   `json:"meta"`
	Nodes           []NetworkNode `json:"nodes"`
	Edges           []NetworkEdge `json:"edges"`
	TotalIdentities int           `json:"total_identities"`
}

type RampPoint struct {
	Key             string          `json:"key"`
	Human           IdentitySummary `json:"human"`
	Agent           IdentitySummary `json:"agent"`
	Episodes        int             `json:"episodes"`
	Completed       int             `json:"completed_episodes"`
	InteractionDays int             `json:"interaction_days"`
	Baseline        float64         `json:"baseline"`
	After           float64         `json:"after"`
	AbsoluteChange  float64         `json:"absolute_change"`
	ObservedLift    *float64        `json:"observed_lift,omitempty"`
	QualityDelta    *float64        `json:"quality_delta,omitempty"`
	Mature          bool            `json:"mature"`
	RankEligible    bool            `json:"rank_eligible"`
}

type AdoptionPoint struct {
	RepositoryID   int64    `json:"repository_id"`
	Repository     string   `json:"repository"`
	AdoptedAt      string   `json:"adopted_at"`
	Baseline       float64  `json:"baseline"`
	After          float64  `json:"after"`
	AbsoluteChange float64  `json:"absolute_change"`
	ObservedLift   *float64 `json:"observed_lift,omitempty"`
	QualityDelta   *float64 `json:"quality_delta,omitempty"`
	Mature         bool     `json:"mature"`
}

type RampResponse struct {
	Meta      InsightMeta     `json:"meta"`
	Handoffs  []RampPoint     `json:"handoffs"`
	Adoptions []AdoptionPoint `json:"adoptions"`
}

type RankEntry struct {
	Key      string             `json:"key"`
	Label    string             `json:"label"`
	Kind     ActorKind          `json:"kind,omitempty"`
	Rank     int                `json:"rank"`
	Value    float64            `json:"value"`
	Eligible bool               `json:"eligible"`
	Metrics  map[string]float64 `json:"metrics"`
}

type RankPoint struct {
	Date  string  `json:"date"`
	Rank  int     `json:"rank"`
	Value float64 `json:"value"`
}

type RankSeries struct {
	Key    string      `json:"key"`
	Label  string      `json:"label"`
	Points []RankPoint `json:"points"`
}

type RankingResponse struct {
	Meta               InsightMeta  `json:"meta"`
	Cohort             string       `json:"cohort"`
	Metric             string       `json:"metric"`
	FavorableDirection string       `json:"favorable_direction"`
	Leaderboard        []RankEntry  `json:"leaderboard"`
	Trajectories       []RankSeries `json:"trajectories"`
}

type resolvedIdentity struct {
	IdentitySummary
	aliases map[string]struct{}
}

type edgeAccumulator struct {
	NetworkEdge
	days    map[string]struct{}
	periods map[string]struct{}
	repos   map[string]struct{}
}

type qualityAccumulator struct {
	commits         int
	reverts         int
	resolvedPulls   int
	mergedPulls     int
	mergeHours      []float64
	approvedMerged  int
	retainedLines   int
	addedLines      int
	retentionSample int
}

var coauthorPattern = regexp.MustCompile(`(?im)^co-authored-by:\s*(.+?)\s*<([^>]+)>\s*$`)

func (manager *Manager) InsightOverview(query InsightQuery) (OverviewResponse, error) {
	reports, overrides := manager.insightState()
	query, meta, err := prepareInsightQuery(reports, query)
	if err != nil {
		return OverviewResponse{}, err
	}
	return buildOverview(reports, overrides, query, meta), nil
}

func (manager *Manager) InsightNetwork(query InsightQuery) (NetworkResponse, error) {
	reports, overrides := manager.insightState()
	query, meta, err := prepareInsightQuery(reports, query)
	if err != nil {
		return NetworkResponse{}, err
	}
	return buildNetwork(reports, overrides, query, meta), nil
}

func (manager *Manager) InsightRamps(query InsightQuery) (RampResponse, error) {
	reports, overrides := manager.insightState()
	query, meta, err := prepareInsightQuery(reports, query)
	if err != nil {
		return RampResponse{}, err
	}
	return buildRamps(reports, overrides, query, meta), nil
}

func (manager *Manager) InsightRankings(query InsightQuery) (RankingResponse, error) {
	reports, overrides := manager.insightState()
	query, meta, err := prepareInsightQuery(reports, query)
	if err != nil {
		return RankingResponse{}, err
	}
	return buildRankings(reports, overrides, query, meta)
}

func (manager *Manager) Identities() IdentityResponse {
	reports, overrides := manager.insightState()
	return buildIdentityResponse(reports, overrides)
}

func (manager *Manager) ScopedIdentities(query InsightQuery) IdentityResponse {
	reports, overrides := manager.insightState()
	selected := make(map[int64]RepositoryReport)
	for id, report := range reports {
		if deliveryReportMatches(report, query) {
			if query.From != nil && query.To != nil {
				events := make([]CommitEvent, 0, len(report.Commits.Events))
				for _, event := range report.Commits.Events {
					if inInsightRange(event.CommittedAt, query) {
						events = append(events, event)
					}
				}
				report.Commits.Events = events
				if report.Pulls != nil {
					pulls := make([]PullRequest, 0, len(report.Pulls.PullRequests))
					for _, pull := range report.Pulls.PullRequests {
						if pull.MergedAt != nil && inInsightRange(*pull.MergedAt, query) {
							pulls = append(pulls, pull)
						}
					}
					stats := BuildPullStats(pulls)
					report.Pulls = &stats
				}
			}
			selected[id] = report
		}
	}
	return buildIdentityResponse(selected, overrides)
}

func buildIdentityResponse(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride) IdentityResponse {
	catalog := buildIdentityCatalog(reports, overrides)
	identities := make([]IdentitySummary, 0, len(catalog))
	for _, identity := range catalog {
		identity.Aliases = sortedSet(identity.aliases)
		identities = append(identities, identity.IdentitySummary)
	}
	sort.Slice(identities, func(i, j int) bool {
		if identities[i].Kind != identities[j].Kind {
			return identities[i].Kind < identities[j].Kind
		}
		return strings.ToLower(identities[i].Name) < strings.ToLower(identities[j].Name)
	})
	return IdentityResponse{Identities: identities}
}

func (manager *Manager) UpdateIdentity(key string, override IdentityOverride) (IdentityResponse, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return IdentityResponse{}, errors.New("identity key is required")
	}
	if override.Kind != "" && override.Kind != ActorHuman && override.Kind != ActorAgent && override.Kind != ActorUnknown {
		return IdentityResponse{}, errors.New("kind must be human, agent, or unknown")
	}
	override.DisplayName = strings.TrimSpace(override.DisplayName)
	override.CanonicalKey = strings.TrimSpace(override.CanonicalKey)
	if override.CanonicalKey == key {
		override.CanonicalKey = ""
		override.Unmerge = true
	}
	manager.mu.Lock()
	next := make(map[string]IdentityOverride, len(manager.identityOverrides)+1)
	for existingKey, existing := range manager.identityOverrides {
		next[existingKey] = existing
	}
	current := next[key]
	if override.Kind != "" {
		current.Kind = override.Kind
	}
	if override.DisplayName != "" {
		current.DisplayName = override.DisplayName
	}
	if override.Unmerge {
		current.CanonicalKey = ""
	} else if override.CanonicalKey != "" {
		current.CanonicalKey = override.CanonicalKey
	}
	current.Unmerge = false
	if current == (IdentityOverride{}) {
		delete(next, key)
	} else {
		next[key] = current
	}
	if identityAliasCycle(key, next) {
		manager.mu.Unlock()
		return IdentityResponse{}, errors.New("identity aliases cannot form a cycle")
	}
	if err := manager.store.SaveIdentityOverrides(next); err != nil {
		manager.mu.Unlock()
		return IdentityResponse{}, err
	}
	manager.identityOverrides = next
	manager.mu.Unlock()
	manager.notify("insights")
	return manager.Identities(), nil
}

func identityAliasCycle(start string, overrides map[string]IdentityOverride) bool {
	current := start
	seen := make(map[string]struct{})
	for range 32 {
		if _, exists := seen[current]; exists {
			return true
		}
		seen[current] = struct{}{}
		current = strings.TrimSpace(overrides[current].CanonicalKey)
		if current == "" {
			return false
		}
	}
	return true
}

func (manager *Manager) insightState() (map[int64]RepositoryReport, map[string]IdentityOverride) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	reports := make(map[int64]RepositoryReport, len(manager.reports))
	for id, report := range manager.reports {
		reports[id] = report
	}
	overrides := make(map[string]IdentityOverride, len(manager.identityOverrides))
	for key, override := range manager.identityOverrides {
		overrides[key] = override
	}
	return reports, overrides
}

func prepareInsightQuery(reports map[int64]RepositoryReport, query InsightQuery) (InsightQuery, InsightMeta, error) {
	if query.SessionHours == 0 {
		query.SessionHours = defaultSessionHours
	}
	if query.AdoptionDays == 0 {
		query.AdoptionDays = defaultWindowDays
	}
	if query.SurvivalDays == 0 {
		query.SurvivalDays = defaultWindowDays
	}
	if query.SessionHours < 1 || query.SessionHours > 168 {
		return query, InsightMeta{}, errors.New("session_hours must be between 1 and 168")
	}
	if query.AdoptionDays < 7 || query.AdoptionDays > 180 {
		return query, InsightMeta{}, errors.New("adoption_days must be between 7 and 180")
	}
	if query.SurvivalDays < 7 || query.SurvivalDays > 180 {
		return query, InsightMeta{}, errors.New("survival_days must be between 7 and 180")
	}
	if query.ActorKind != "" && query.ActorKind != ActorHuman && query.ActorKind != ActorAgent && query.ActorKind != ActorUnknown {
		return query, InsightMeta{}, errors.New("actor_kind must be human, agent, or unknown")
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return query, InsightMeta{}, errors.New("from must be on or before to")
	}
	availableFrom, availableTo := insightBounds(reports, query)
	meta := InsightMeta{Coverage: InsightCoverage{}, SessionHours: query.SessionHours, AdoptionDays: query.AdoptionDays, SurvivalDays: query.SurvivalDays}
	if availableFrom.IsZero() {
		meta.Unavailable = []string{"relationship_analysis_requires_commit_events"}
		return query, meta, nil
	}
	selectedFrom, selectedTo := availableFrom, availableTo
	if query.To != nil && query.To.Before(selectedTo) {
		selectedTo = *query.To
	}
	if query.From == nil {
		defaultFrom := addMonthsClamped(selectedTo, -defaultHistoryMonths)
		if defaultFrom.After(selectedFrom) {
			selectedFrom = defaultFrom
		}
	} else if query.From.After(selectedFrom) {
		selectedFrom = *query.From
	}
	if selectedFrom.After(selectedTo) {
		return query, InsightMeta{}, errors.New("selected date range does not overlap available data")
	}
	query.From, query.To = &selectedFrom, &selectedTo
	query.maturityCutoff = selectedTo
	if !selectedTo.Before(availableTo) {
		today := dayUTC(time.Now())
		if today.After(query.maturityCutoff) {
			query.maturityCutoff = today
		}
	}
	meta.AvailableFrom = availableFrom.Format(time.DateOnly)
	meta.AvailableTo = availableTo.Format(time.DateOnly)
	meta.From = selectedFrom.Format(time.DateOnly)
	meta.To = selectedTo.Format(time.DateOnly)
	meta.Granularity = activityGranularity(selectedFrom, selectedTo)
	meta.Unavailable = []string{"retained_line_rate_pending_enriched_git_analysis"}
	for _, report := range reports {
		if insightReportMatches(report, query) && report.Commits.Commits > 0 && len(report.Commits.Events) == 0 {
			meta.Unavailable = append(meta.Unavailable, "relationship_analysis_requires_commit_events")
			break
		}
	}
	return query, meta, nil
}

func addMonthsClamped(date time.Time, months int) time.Time {
	firstOfTarget := time.Date(date.Year(), date.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstOfTarget.AddDate(0, 1, -1).Day()
	day := date.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(firstOfTarget.Year(), firstOfTarget.Month(), day, 0, 0, 0, 0, time.UTC)
}

func insightBounds(reports map[int64]RepositoryReport, query InsightQuery) (time.Time, time.Time) {
	var first, last time.Time
	for _, report := range reports {
		if !insightReportMatches(report, query) {
			continue
		}
		for _, event := range report.Commits.Events {
			date := dayUTC(event.CommittedAt)
			if first.IsZero() || date.Before(first) {
				first = date
			}
			if last.IsZero() || date.After(last) {
				last = date
			}
		}
		if report.Pulls != nil {
			for _, pull := range report.Pulls.PullRequests {
				dates := []time.Time{pull.CreatedAt, pull.UpdatedAt}
				if pull.MergedAt != nil {
					dates = append(dates, *pull.MergedAt)
				} else if pull.ClosedAt != nil {
					dates = append(dates, *pull.ClosedAt)
				}
				for _, review := range pull.Reviews {
					if review.SubmittedAt != nil {
						dates = append(dates, *review.SubmittedAt)
					}
				}
				for _, value := range dates {
					if value.IsZero() {
						continue
					}
					date := dayUTC(value)
					if first.IsZero() || date.Before(first) {
						first = date
					}
					if last.IsZero() || date.After(last) {
						last = date
					}
				}
			}
		}
	}
	return first, last
}

func insightReportMatches(report RepositoryReport, query InsightQuery) bool {
	if (query.OwnerID != 0 && report.Repository.Owner.ID != query.OwnerID) ||
		(query.RepositoryID != 0 && report.Repository.ID != query.RepositoryID) {
		return false
	}
	return !query.ExcludeDead || !BuildRepositoryLiveness(report.Commits, report.Repository.CreatedAt, time.Now().UTC()).IsDead
}

func inInsightRange(value time.Time, query InsightQuery) bool {
	if value.IsZero() || query.From == nil || query.To == nil {
		return false
	}
	date := dayUTC(value)
	return !date.Before(*query.From) && !date.After(*query.To)
}

func insightMaturityCutoff(query InsightQuery) time.Time {
	if !query.maturityCutoff.IsZero() {
		return query.maturityCutoff
	}
	if query.To != nil {
		return *query.To
	}
	return time.Time{}
}

func buildIdentityCatalog(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride) map[string]*resolvedIdentity {
	catalog := make(map[string]*resolvedIdentity)
	usedOverrides := make(map[string]struct{})
	stableMigrations := stableIdentityMigrations(reports, overrides)
	add := func(person ContributorMetrics) *resolvedIdentity {
		if person.Key == "" {
			return nil
		}
		canonical := canonicalPersonIdentityKey(person, overrides, stableMigrations)
		identity := catalog[canonical]
		if identity == nil {
			summary := classifyIdentity(person)
			summary.Key, summary.CanonicalKey = canonical, canonical
			identity = &resolvedIdentity{IdentitySummary: summary, aliases: make(map[string]struct{})}
			catalog[canonical] = identity
		}
		identity.aliases[person.Key] = struct{}{}
		legacyKey := legacyPersonIdentityKey(person)
		if _, exists := overrides[legacyKey]; legacyKey != "" && legacyKey != person.Key && exists {
			identity.aliases[legacyKey] = struct{}{}
			usedOverrides[legacyKey] = struct{}{}
		}
		usedOverrides[person.Key] = struct{}{}
		usedOverrides[canonical] = struct{}{}
		mergeIdentityMetadata(&identity.IdentitySummary, person)
		applyIdentityOverride(&identity.IdentitySummary, overrides[legacyKey])
		applyIdentityOverride(&identity.IdentitySummary, overrides[person.Key])
		applyIdentityOverride(&identity.IdentitySummary, overrides[canonical])
		return identity
	}
	for _, report := range reports {
		for _, event := range report.Commits.Events {
			participants := event.Participants
			if len(participants) == 0 {
				participants = append([]ContributorMetrics{event.Author}, commitCoauthors(event.Message)...)
			}
			seen := make(map[string]struct{})
			for _, participant := range participants {
				canonical := canonicalPersonIdentityKey(participant, overrides, stableMigrations)
				if _, exists := seen[canonical]; exists {
					continue
				}
				seen[canonical] = struct{}{}
				identity := add(participant)
				if identity != nil {
					identity.Commits++
				}
			}
		}
		if report.Pulls != nil {
			for key, person := range report.Pulls.Contributors {
				person.Key = key
				identity := add(person)
				if identity != nil {
					identity.PullRequests += person.PullRequests
				}
			}
			for _, pull := range report.Pulls.PullRequests {
				for _, review := range pull.Reviews {
					identity := add(personMetrics(review.Author))
					if identity != nil {
						identity.Reviews++
					}
				}
			}
		}
	}
	for key, override := range overrides {
		if _, used := usedOverrides[key]; used {
			continue
		}
		if _, exists := catalog[canonicalIdentityKey(key, overrides)]; exists {
			continue
		}
		person := ContributorMetrics{Key: key, Name: override.DisplayName}
		add(person)
	}
	return catalog
}

func classifyIdentity(person ContributorMetrics) IdentitySummary {
	name := strings.TrimSpace(person.Name)
	if name == "" {
		name = strings.TrimSpace(person.Login)
	}
	if name == "" {
		name = person.Key
	}
	lowerLogin := strings.ToLower(strings.TrimSpace(person.Login))
	lowerKey := strings.ToLower(person.Key)
	if strings.EqualFold(person.Type, "Bot") || strings.EqualFold(person.Type, "App") || strings.HasSuffix(lowerLogin, "[bot]") || strings.Contains(lowerKey, "[bot]") {
		return IdentitySummary{Name: name, Login: person.Login, AvatarURL: person.AvatarURL, Kind: ActorAgent, Evidence: "github_bot", Confidence: "confirmed"}
	}
	if person.Type == "AgentSignature" {
		return IdentitySummary{Name: name, Login: person.Login, AvatarURL: person.AvatarURL, Kind: ActorAgent, Evidence: "known_agent_signature", Confidence: "confirmed"}
	}
	if strings.HasPrefix(lowerKey, "github:") {
		return IdentitySummary{Name: name, Login: person.Login, AvatarURL: person.AvatarURL, Kind: ActorHuman, Evidence: "github_user", Confidence: "suggested"}
	}
	return IdentitySummary{Name: name, Login: person.Login, AvatarURL: person.AvatarURL, Kind: ActorUnknown, Evidence: "unverified_git_identity", Confidence: "unknown"}
}

func knownAgentIdentity(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	known := map[string]struct{}{
		"claude": {}, "claude code": {}, "noreply@anthropic.com": {},
		"codex": {}, "openai codex": {}, "codex@openai.com": {},
		"github copilot": {}, "copilot": {}, "copilot@github.com": {},
		"cursor": {}, "cursor agent": {}, "devin": {}, "devin-ai-integration[bot]": {},
		"moltenbot000": {}, "molten bot 000": {}, "moltenhub-bot": {},
		"dependabot": {}, "dependabot[bot]": {}, "renovate[bot]": {}, "github-actions[bot]": {},
	}
	_, exists := known[value]
	return exists
}

func canonicalIdentityKey(key string, overrides map[string]IdentityOverride) string {
	current := key
	seen := map[string]struct{}{current: {}}
	for range 16 {
		next := strings.TrimSpace(overrides[current].CanonicalKey)
		if next == "" {
			return current
		}
		if _, exists := seen[next]; exists {
			return current
		}
		seen[next] = struct{}{}
		current = next
	}
	return current
}

func canonicalPersonIdentityKey(person ContributorMetrics, overrides map[string]IdentityOverride, stableMigrations map[string]string) string {
	canonical := canonicalIdentityKey(person.Key, overrides)
	if canonical != person.Key {
		return canonical
	}
	if migrated := stableMigrations[person.Key]; migrated != "" {
		return migrated
	}
	return canonical
}

func stableIdentityMigrations(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride) map[string]string {
	targets := make(map[string]map[string]struct{})
	for _, report := range reports {
		for _, event := range report.Commits.Events {
			participants := event.Participants
			if len(participants) == 0 {
				participants = append([]ContributorMetrics{event.Author}, commitCoauthors(event.Message)...)
			}
			for _, person := range participants {
				if !strings.HasPrefix(person.Key, "email:") || canonicalIdentityKey(person.Key, overrides) != person.Key {
					continue
				}
				legacyKey := legacyPersonIdentityKey(person)
				if legacyKey == "" {
					continue
				}
				target := canonicalIdentityKey(legacyKey, overrides)
				if target == legacyKey {
					continue
				}
				if targets[person.Key] == nil {
					targets[person.Key] = make(map[string]struct{})
				}
				targets[person.Key][target] = struct{}{}
			}
		}
	}
	migrations := make(map[string]string)
	for key, candidates := range targets {
		if len(candidates) != 1 {
			continue
		}
		for target := range candidates {
			migrations[key] = target
		}
	}
	return migrations
}

func legacyPersonIdentityKey(person ContributorMetrics) string {
	if strings.TrimSpace(person.Name) == "" {
		return ""
	}
	return gitIdentityKey(person.Name)
}

func applyIdentityOverride(identity *IdentitySummary, override IdentityOverride) {
	if override.Kind != "" {
		identity.Kind = override.Kind
		identity.Evidence = "manual_override"
		identity.Confidence = "confirmed"
	}
	if override.DisplayName != "" {
		identity.Name = override.DisplayName
	}
}

func mergeIdentityMetadata(identity *IdentitySummary, person ContributorMetrics) {
	if identity.Name == "" && person.Name != "" {
		identity.Name = person.Name
	}
	if identity.Login == "" && person.Login != "" {
		identity.Login = person.Login
	}
	if identity.AvatarURL == "" && person.AvatarURL != "" {
		identity.AvatarURL = person.AvatarURL
	}
}

func commitCoauthors(message string) []ContributorMetrics {
	lines := strings.Split(strings.TrimRight(message, " \t\r\n"), "\n")
	start := len(lines)
	for start > 0 && isCommitTrailerLine(lines[start-1]) {
		start--
	}
	if start == len(lines) || start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		return nil
	}

	result := make([]ContributorMetrics, 0, len(lines)-start)
	seen := make(map[string]struct{})
	for _, line := range lines[start:] {
		match := coauthorPattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		name, email := strings.TrimSpace(match[1]), strings.ToLower(strings.TrimSpace(match[2]))
		key, login := "email:"+email, ""
		if at := strings.Index(email, "@users.noreply.github.com"); at > 0 {
			prefix := strings.TrimPrefix(email[:at], "")
			if dash := strings.Index(prefix, "+"); dash >= 0 {
				prefix = prefix[dash+1:]
			}
			login = prefix
			key = "github:" + strings.ToLower(prefix)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		identityType := ""
		if knownAgentIdentity(login) || knownAgentIdentity(strings.ToLower(name)) || knownAgentIdentity(email) {
			identityType = "AgentSignature"
		}
		result = append(result, ContributorMetrics{Key: key, Login: login, Name: name, Type: identityType})
	}
	return result
}

func isCommitTrailerLine(line string) bool {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	key, _, found := strings.Cut(line, ":")
	key = strings.TrimSpace(key)
	if !found || key == "" {
		return false
	}
	for _, character := range key {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func personMetrics(person Person) ContributorMetrics {
	login := strings.TrimSpace(person.Login)
	key := "github:" + strings.ToLower(login)
	if login == "" {
		key = "git:unknown"
	}
	name := person.Name
	if name == "" {
		name = login
	}
	return ContributorMetrics{Key: key, Login: login, Name: name, AvatarURL: person.AvatarURL, Type: person.Type}
}

func identityAliasIndex(catalog map[string]*resolvedIdentity) map[string]*resolvedIdentity {
	index := make(map[string]*resolvedIdentity)
	for _, identity := range catalog {
		for alias := range identity.aliases {
			index[alias] = identity
		}
	}
	return index
}

func eventIdentities(event CommitEvent, catalog map[string]*resolvedIdentity, aliases map[string]*resolvedIdentity, overrides map[string]IdentityOverride) []*resolvedIdentity {
	people := event.Participants
	if len(people) == 0 {
		people = append([]ContributorMetrics{event.Author}, commitCoauthors(event.Message)...)
	}
	result := make([]*resolvedIdentity, 0, len(people))
	seen := make(map[string]struct{})
	for _, person := range people {
		key := canonicalIdentityKey(person.Key, overrides)
		identity := catalog[key]
		if identity == nil {
			identity = aliases[person.Key]
		}
		if identity == nil {
			legacyKey := legacyPersonIdentityKey(person)
			legacyCanonical := canonicalIdentityKey(legacyKey, overrides)
			if legacyKey != "" && legacyCanonical != legacyKey && catalog[legacyCanonical] != nil {
				key = legacyCanonical
				identity = catalog[key]
			}
		}
		if identity == nil {
			continue
		}
		key = identity.Key
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, identity)
	}
	return result
}

func knownIdentities(identities []*resolvedIdentity) []*resolvedIdentity {
	known := make([]*resolvedIdentity, 0, len(identities))
	for _, identity := range identities {
		if isKnownIdentity(identity) {
			known = append(known, identity)
		}
	}
	return known
}

func knownEventIdentities(event CommitEvent, catalog map[string]*resolvedIdentity, aliases map[string]*resolvedIdentity, overrides map[string]IdentityOverride) []*resolvedIdentity {
	return knownIdentities(eventIdentities(event, catalog, aliases, overrides))
}

func isKnownIdentity(identity *resolvedIdentity) bool {
	return identity != nil && (identity.Kind == ActorHuman || identity.Kind == ActorAgent)
}

func primaryKnownIdentity(identities []*resolvedIdentity) *resolvedIdentity {
	if len(identities) == 0 || !isKnownIdentity(identities[0]) {
		return nil
	}
	return identities[0]
}

func workBucket(identities []*resolvedIdentity) string {
	human, agent, unknown := false, false, false
	for _, identity := range identities {
		switch identity.Kind {
		case ActorHuman:
			human = true
		case ActorAgent:
			agent = true
		default:
			unknown = true
		}
	}
	if human && agent {
		return "mixed"
	}
	if unknown {
		return "unknown"
	}
	if agent {
		return "agent_only"
	}
	if human {
		return "human_only"
	}
	return "unknown"
}

func actorFilterMatchesBucket(filter ActorKind, bucket string) bool {
	if bucket == "unknown" || filter == ActorUnknown {
		return false
	}
	if filter == "" {
		return true
	}
	switch filter {
	case ActorHuman:
		return bucket == "human_only" || bucket == "mixed"
	case ActorAgent:
		return bucket == "agent_only" || bucket == "mixed"
	default:
		return false
	}
}

func actorFilterMatchesIdentity(filter ActorKind, identity *resolvedIdentity) bool {
	return isKnownIdentity(identity) && (filter == "" || identity.Kind == filter)
}

func insightCoverage(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery) InsightCoverage {
	coverage := InsightCoverage{}
	if query.From == nil || query.To == nil {
		return coverage
	}
	catalog := buildIdentityCatalog(reports, overrides)
	aliases := identityAliasIndex(catalog)
	maturityCutoff := insightMaturityCutoff(query)
	for _, report := range reports {
		if !insightReportMatches(report, query) {
			continue
		}
		for _, event := range report.Commits.Events {
			if !inInsightRange(event.CommittedAt, query) {
				continue
			}
			identities := knownEventIdentities(event, catalog, aliases, overrides)
			if len(identities) == 0 {
				if query.ActorKind == "" {
					coverage.TotalCommits++
					coverage.UnknownCommits++
				}
				continue
			}
			bucket := workBucket(identities)
			if !actorFilterMatchesBucket(query.ActorKind, bucket) {
				continue
			}
			coverage.TotalCommits++
			coverage.ClassifiedCommits++
			if !dayUTC(event.CommittedAt.AddDate(0, 0, query.SurvivalDays)).After(maturityCutoff) {
				coverage.MatureCommits++
			}
		}
		if report.Pulls == nil {
			continue
		}
		for _, pull := range report.Pulls.PullRequests {
			if !inInsightRange(pull.CreatedAt, query) || !actorFilterMatchesIdentity(query.ActorKind, catalog[canonicalIdentityKey(personMetrics(pull.Author).Key, overrides)]) {
				continue
			}
			resolvedAt := pullResolvedAt(pull)
			if resolvedAt == nil || dayUTC(*resolvedAt).After(maturityCutoff) {
				continue
			}
			coverage.EligiblePulls++
			if pull.MergedAt != nil && pullHasKnownApproval(pull, catalog, overrides) {
				coverage.ReviewedPulls++
			}
		}
	}
	if coverage.TotalCommits > 0 {
		coverage.ClassificationRate = float64(coverage.ClassifiedCommits) / float64(coverage.TotalCommits)
	}
	return coverage
}

func pullResolvedAt(pull PullRequest) *time.Time {
	if pull.MergedAt != nil {
		return pull.MergedAt
	}
	return pull.ClosedAt
}

func buildNetwork(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery, meta InsightMeta) NetworkResponse {
	return buildNetworkWithLimit(reports, overrides, query, meta, maximumNetworkNodes)
}

func buildNetworkWithLimit(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery, meta InsightMeta, maximumNodes int) NetworkResponse {
	meta.Coverage = insightCoverage(reports, overrides, query)
	catalog := buildIdentityCatalog(reports, overrides)
	aliases := identityAliasIndex(catalog)
	for _, identity := range catalog {
		identity.Commits, identity.PullRequests, identity.Reviews = 0, 0, 0
	}
	edges := make(map[string]*edgeAccumulator)
	ensureEdge := func(left, right *resolvedIdentity, repository string, at time.Time) *edgeAccumulator {
		if !isKnownIdentity(left) || !isKnownIdentity(right) || left.Key == right.Key {
			return nil
		}
		source, target := left, right
		if source.Key > target.Key {
			source, target = target, source
		}
		key := source.Key + "\x00" + target.Key
		edge := edges[key]
		if edge == nil {
			edge = &edgeAccumulator{NetworkEdge: NetworkEdge{Source: source.Key, Target: target.Key, PairType: identityPairType(source.Kind, target.Kind)}, days: make(map[string]struct{}), periods: make(map[string]struct{}), repos: make(map[string]struct{})}
			edges[key] = edge
		}
		edge.days[dayUTC(at).Format(time.DateOnly)] = struct{}{}
		edge.periods[activityBucketStart(dayUTC(at), meta.Granularity).Format(time.DateOnly)] = struct{}{}
		edge.repos[repository] = struct{}{}
		return edge
	}
	session := time.Duration(query.SessionHours) * time.Hour
	for _, report := range reports {
		if !insightReportMatches(report, query) {
			continue
		}
		events := append([]CommitEvent(nil), report.Commits.Events...)
		sort.Slice(events, func(i, j int) bool { return events[i].CommittedAt.Before(events[j].CommittedAt) })
		var previousPrimary *resolvedIdentity
		var previousAt time.Time
		for index := range events {
			event := &events[index]
			if !inInsightRange(event.CommittedAt, query) {
				continue
			}
			resolved := eventIdentities(*event, catalog, aliases, overrides)
			participants := knownIdentities(resolved)
			primary := primaryKnownIdentity(resolved)
			for _, identity := range participants {
				identity.Commits++
			}
			if primary != nil && len(participants) > 1 {
				for _, coauthor := range participants {
					if edge := ensureEdge(primary, coauthor, report.Repository.FullName, event.CommittedAt); edge != nil {
						edge.Coauthorships++
					}
				}
			}
			if previousPrimary != nil && primary != nil && event.CommittedAt.Sub(previousAt) <= session {
				if edge := ensureEdge(previousPrimary, primary, report.Repository.FullName, event.CommittedAt); edge != nil {
					edge.Handoffs++
					if previousPrimary.Kind == ActorHuman && primary.Kind == ActorAgent {
						edge.HumanToAgent++
					}
				}
			}
			if primary != nil {
				previousPrimary, previousAt = primary, event.CommittedAt
			}
		}
		if report.Pulls != nil {
			for _, pull := range report.Pulls.PullRequests {
				if !inInsightRange(pull.CreatedAt, query) {
					continue
				}
				author := catalog[canonicalIdentityKey(personMetrics(pull.Author).Key, overrides)]
				if isKnownIdentity(author) {
					author.PullRequests++
				} else {
					author = nil
				}
				for _, review := range pull.Reviews {
					at := pull.CreatedAt
					if review.SubmittedAt != nil {
						at = *review.SubmittedAt
					}
					if !inInsightRange(at, query) {
						continue
					}
					reviewer := catalog[canonicalIdentityKey(personMetrics(review.Author).Key, overrides)]
					if isKnownIdentity(reviewer) {
						reviewer.Reviews++
					} else {
						reviewer = nil
					}
					if edge := ensureEdge(author, reviewer, report.Repository.FullName, at); edge != nil {
						edge.ReviewInteractions++
					}
				}
			}
		}
	}
	relevant := make(map[string]struct{})
	if query.ActorKind != "" {
		for key, identity := range catalog {
			if identity.Kind == query.ActorKind {
				relevant[key] = struct{}{}
			}
		}
		for _, edge := range edges {
			_, sourceMatches := relevant[edge.Source]
			_, targetMatches := relevant[edge.Target]
			if sourceMatches || targetMatches {
				relevant[edge.Source] = struct{}{}
				relevant[edge.Target] = struct{}{}
			}
		}
	}
	nodes := make([]NetworkNode, 0, len(catalog))
	for key, identity := range catalog {
		if !isKnownIdentity(identity) {
			continue
		}
		if query.ActorKind != "" {
			if _, include := relevant[key]; !include {
				continue
			}
		}
		identity.Aliases = sortedSet(identity.aliases)
		nodes = append(nodes, NetworkNode{IdentitySummary: identity.IdentitySummary, Activity: identity.Commits + identity.PullRequests + identity.Reviews})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Activity == nodes[j].Activity {
			return nodes[i].Name < nodes[j].Name
		}
		return nodes[i].Activity > nodes[j].Activity
	})
	totalIdentities := len(nodes)
	meta.TotalResults = totalIdentities
	allowed := make(map[string]struct{})
	if maximumNodes > 0 && len(nodes) > maximumNodes {
		nodes = nodes[:maximumNodes]
		meta.Truncated = true
	}
	for _, node := range nodes {
		allowed[node.Key] = struct{}{}
	}
	resultEdges := make([]NetworkEdge, 0, len(edges))
	for _, edge := range edges {
		if _, ok := allowed[edge.Source]; !ok {
			continue
		}
		if _, ok := allowed[edge.Target]; !ok {
			continue
		}
		edge.InteractionDays = len(edge.days)
		edge.Periods = sortedSet(edge.periods)
		edge.Repositories = sortedSet(edge.repos)
		resultEdges = append(resultEdges, edge.NetworkEdge)
	}
	sort.Slice(resultEdges, func(i, j int) bool {
		if resultEdges[i].InteractionDays == resultEdges[j].InteractionDays {
			return resultEdges[i].Handoffs > resultEdges[j].Handoffs
		}
		return resultEdges[i].InteractionDays > resultEdges[j].InteractionDays
	})
	return NetworkResponse{Meta: meta, Nodes: nodes, Edges: resultEdges, TotalIdentities: totalIdentities}
}

func identityPairType(left, right ActorKind) string {
	if (left == ActorHuman && right == ActorAgent) || (left == ActorAgent && right == ActorHuman) {
		return "human_agent"
	}
	if left == ActorHuman && right == ActorHuman {
		return "human_human"
	}
	if left == ActorAgent && right == ActorAgent {
		return "agent_agent"
	}
	return "unknown"
}

func normalizedPairKey(left, right string) string {
	if left > right {
		left, right = right, left
	}
	return left + "\x00" + right
}

func buildOverview(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery, meta InsightMeta) OverviewResponse {
	meta.Coverage = insightCoverage(reports, overrides, query)
	result := OverviewResponse{Meta: meta, Timeline: []TimelinePoint{}, Quality: []QualityPoint{}, Repositories: []RepositoryPulse{}}
	if query.From == nil {
		return result
	}
	catalog := buildIdentityCatalog(reports, overrides)
	aliases := identityAliasIndex(catalog)
	maturityCutoff := insightMaturityCutoff(query)
	timeline := make(map[string]*TimelinePoint)
	quality := make(map[string]*qualityAccumulator)
	pulses := make(map[int64]*RepositoryPulse)
	pulseBuckets := make(map[int64]map[string]*PulsePoint)
	for bucket := activityBucketStart(*query.From, meta.Granularity); !bucket.After(*query.To); bucket = nextActivityBucket(bucket, meta.Granularity) {
		key := bucket.Format(time.DateOnly)
		timeline[key] = &TimelinePoint{Date: key}
		quality[key] = &qualityAccumulator{}
	}
	for _, report := range reports {
		if !insightReportMatches(report, query) {
			continue
		}
		pulse := &RepositoryPulse{RepositoryID: report.Repository.ID, Name: report.Repository.FullName}
		pulses[report.Repository.ID], pulseBuckets[report.Repository.ID] = pulse, make(map[string]*PulsePoint)
		for _, event := range report.Commits.Events {
			if !inInsightRange(event.CommittedAt, query) {
				continue
			}
			identities := knownEventIdentities(event, catalog, aliases, overrides)
			if len(identities) == 0 {
				continue
			}
			kind := workBucket(identities)
			if !actorFilterMatchesBucket(query.ActorKind, kind) {
				continue
			}
			bucket := activityBucketStart(dayUTC(event.CommittedAt), meta.Granularity).Format(time.DateOnly)
			point := timeline[bucket]
			pulsePoint := pulseBuckets[report.Repository.ID][bucket]
			if pulsePoint == nil {
				pulsePoint = &PulsePoint{Date: bucket}
				pulseBuckets[report.Repository.ID][bucket] = pulsePoint
			}
			switch kind {
			case "human_only":
				point.HumanOnly++
				pulsePoint.HumanOnly++
			case "agent_only":
				point.AgentOnly++
				pulsePoint.AgentOnly++
			case "mixed":
				point.Mixed++
				pulsePoint.Mixed++
			default:
				point.Unknown++
				pulsePoint.Unknown++
			}
			pulse.Total++
			qa := quality[bucket]
			mature := !dayUTC(event.CommittedAt.AddDate(0, 0, query.SurvivalDays)).After(maturityCutoff)
			if mature {
				qa.commits++
			}
			if mature && query.SurvivalDays == 30 && event.RetentionMeasured && event.LinesAdded > 0 {
				qa.retainedLines += event.RetainedLines
				qa.addedLines += event.LinesAdded
				qa.retentionSample++
			}
			if mature && eventIsExplicitRevert(event) {
				qa.reverts++
			}
		}
		if report.Pulls != nil {
			for _, pull := range report.Pulls.PullRequests {
				if !inInsightRange(pull.CreatedAt, query) {
					continue
				}
				author := catalog[canonicalIdentityKey(personMetrics(pull.Author).Key, overrides)]
				if !actorFilterMatchesIdentity(query.ActorKind, author) {
					continue
				}
				bucket := activityBucketStart(dayUTC(pull.CreatedAt), meta.Granularity).Format(time.DateOnly)
				point := timeline[bucket]
				if point == nil {
					continue
				}
				point.Pulls++
				qa := quality[bucket]
				resolvedAt := pullResolvedAt(pull)
				if resolvedAt != nil && !dayUTC(*resolvedAt).After(maturityCutoff) {
					qa.resolvedPulls++
					if pull.MergedAt != nil {
						qa.mergedPulls++
						qa.mergeHours = append(qa.mergeHours, pull.MergedAt.Sub(pull.CreatedAt).Hours())
						if pullHasKnownApproval(pull, catalog, overrides) {
							qa.approvedMerged++
						}
					}
				}
			}
		}
	}
	keys := make([]string, 0, len(timeline))
	for key := range timeline {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	agentEvents := 0
	for _, key := range keys {
		point := *timeline[key]
		agentEvents += point.AgentOnly + point.Mixed
		result.Timeline = append(result.Timeline, point)
		qa := quality[key]
		qualityPoint := QualityPoint{Date: key, CommitSample: qa.commits, PullRequestSample: qa.resolvedPulls, RetentionSample: qa.retentionSample}
		if qa.commits > 0 {
			value := float64(qa.reverts) / float64(qa.commits)
			qualityPoint.RevertRate = &value
		}
		if qa.resolvedPulls > 0 {
			value := float64(qa.mergedPulls) / float64(qa.resolvedPulls)
			qualityPoint.MergeRate = &value
		}
		if len(qa.mergeHours) > 0 {
			value := median(qa.mergeHours)
			qualityPoint.MedianMergeHours = &value
		}
		if qa.mergedPulls > 0 {
			value := float64(qa.approvedMerged) / float64(qa.mergedPulls)
			qualityPoint.ReviewCoverage = &value
		}
		if qa.addedLines > 0 {
			value := float64(qa.retainedLines) / float64(qa.addedLines)
			qualityPoint.RetainedLineRate = &value
		}
		result.Quality = append(result.Quality, qualityPoint)
	}
	for _, point := range result.Quality {
		if point.RetentionSample > 0 {
			result.Meta.Unavailable = removeString(result.Meta.Unavailable, "retained_line_rate_pending_enriched_git_analysis")
			break
		}
	}
	if result.Meta.Coverage.ClassifiedCommits > 0 {
		result.Summary.AgentParticipation = float64(agentEvents) / float64(result.Meta.Coverage.ClassifiedCommits)
	}
	for id, pulse := range pulses {
		for _, key := range keys {
			point := pulseBuckets[id][key]
			if point == nil {
				point = &PulsePoint{Date: key}
			}
			pulse.Points = append(pulse.Points, *point)
		}
		if pulse.Total > 0 {
			result.Repositories = append(result.Repositories, *pulse)
		}
	}
	sort.Slice(result.Repositories, func(i, j int) bool { return result.Repositories[i].Total > result.Repositories[j].Total })
	result.Meta.TotalResults = len(result.Repositories)
	if len(result.Repositories) > 100 {
		result.Repositories = result.Repositories[:100]
		result.Meta.Truncated = true
	}
	network := buildNetworkWithLimit(reports, overrides, query, meta, 0)
	if len(network.Edges) > 0 {
		top := network.Edges[0]
		names := map[string]string{}
		for _, node := range network.Nodes {
			names[node.Key] = node.Name
		}
		result.Summary.StrongestPair = names[top.Source] + " × " + names[top.Target]
		result.Summary.StrongestPairDays = top.InteractionDays
	}
	ramps := buildRamps(reports, overrides, query, meta)
	var pre, after float64
	for _, point := range ramps.Handoffs {
		pre += point.Baseline
		after += point.After
		result.Summary.HandoffEpisodes += point.Completed
	}
	if pre > 0 {
		lift := (after - pre) / pre
		result.Summary.HandoffLift = &lift
	}
	if len(result.Quality) >= 2 && result.Quality[len(result.Quality)-1].RevertRate != nil && result.Quality[len(result.Quality)-2].RevertRate != nil {
		direction := *result.Quality[len(result.Quality)-2].RevertRate - *result.Quality[len(result.Quality)-1].RevertRate
		result.Summary.QualityDirection = &direction
	}
	return result
}

func buildRamps(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery, meta InsightMeta) RampResponse {
	meta.Coverage = insightCoverage(reports, overrides, query)
	result := RampResponse{Meta: meta, Handoffs: []RampPoint{}, Adoptions: []AdoptionPoint{}}
	if query.From == nil || query.ActorKind == ActorUnknown {
		return result
	}
	catalog := buildIdentityCatalog(reports, overrides)
	aliases := identityAliasIndex(catalog)
	maturityCutoff := insightMaturityCutoff(query)
	type rampAccumulator struct {
		human, agent             *resolvedIdentity
		episodes, completed      int
		pre, after               float64
		preReverts, afterReverts int
	}
	pairs := make(map[string]*rampAccumulator)
	session := time.Duration(query.SessionHours) * time.Hour
	for _, report := range reports {
		if !insightReportMatches(report, query) {
			continue
		}
		allEvents := append([]CommitEvent(nil), report.Commits.Events...)
		sort.Slice(allEvents, func(i, j int) bool { return allEvents[i].CommittedAt.Before(allEvents[j].CommittedAt) })
		for i := 1; i < len(allEvents); i++ {
			if !inInsightRange(allEvents[i].CommittedAt, query) {
				continue
			}
			previousIdentity := primaryKnownIdentity(eventIdentities(allEvents[i-1], catalog, aliases, overrides))
			currentIdentity := primaryKnownIdentity(eventIdentities(allEvents[i], catalog, aliases, overrides))
			if previousIdentity == nil || currentIdentity == nil || previousIdentity.Kind != ActorHuman || currentIdentity.Kind != ActorAgent || allEvents[i].CommittedAt.Sub(allEvents[i-1].CommittedAt) > session {
				continue
			}
			key := previousIdentity.Key + "\x00" + currentIdentity.Key
			acc := pairs[key]
			if acc == nil {
				acc = &rampAccumulator{human: previousIdentity, agent: currentIdentity}
				pairs[key] = acc
			}
			acc.episodes++
			preStart, postEnd := allEvents[i].CommittedAt.Add(-session), allEvents[i].CommittedAt.Add(session)
			if dayUTC(postEnd).After(maturityCutoff) {
				continue
			}
			acc.completed++
			for _, candidate := range allEvents {
				if len(knownEventIdentities(candidate, catalog, aliases, overrides)) == 0 {
					continue
				}
				if !candidate.CommittedAt.Before(allEvents[i].CommittedAt) && !candidate.CommittedAt.After(postEnd) {
					acc.after++
					if eventIsExplicitRevert(candidate) {
						acc.afterReverts++
					}
				}
				if !candidate.CommittedAt.Before(preStart) && candidate.CommittedAt.Before(allEvents[i].CommittedAt) {
					acc.pre++
					if eventIsExplicitRevert(candidate) {
						acc.preReverts++
					}
				}
			}
		}
		var adoptionIndex = -1
		for i, event := range allEvents {
			for _, identity := range eventIdentities(event, catalog, aliases, overrides) {
				if identity.Kind == ActorAgent && identity.Confidence == "confirmed" {
					adoptionIndex = i
					break
				}
			}
			if adoptionIndex >= 0 {
				break
			}
		}
		if adoptionIndex >= 0 && inInsightRange(allEvents[adoptionIndex].CommittedAt, query) {
			adopted := allEvents[adoptionIndex].CommittedAt
			window := time.Duration(query.AdoptionDays) * 24 * time.Hour
			var pre, after float64
			var preReverts, afterReverts int
			for _, event := range allEvents {
				if len(knownEventIdentities(event, catalog, aliases, overrides)) == 0 {
					continue
				}
				if !event.CommittedAt.Before(adopted) && !event.CommittedAt.After(adopted.Add(window)) {
					after++
					if eventIsExplicitRevert(event) {
						afterReverts++
					}
				}
				if !event.CommittedAt.Before(adopted.Add(-window)) && event.CommittedAt.Before(adopted) {
					pre++
					if eventIsExplicitRevert(event) {
						preReverts++
					}
				}
			}
			point := AdoptionPoint{RepositoryID: report.Repository.ID, Repository: report.Repository.FullName, AdoptedAt: adopted.UTC().Format(time.DateOnly), Baseline: pre, After: after, AbsoluteChange: after - pre, Mature: !dayUTC(adopted.Add(window)).After(maturityCutoff)}
			if point.Mature && pre > 0 {
				value := (after - pre) / pre
				point.ObservedLift = &value
			}
			if point.Mature && pre > 0 && after > 0 {
				value := float64(preReverts)/pre - float64(afterReverts)/after
				point.QualityDelta = &value
			}
			result.Adoptions = append(result.Adoptions, point)
		}
	}
	interactionDays := make(map[string]int)
	for _, edge := range buildNetworkWithLimit(reports, overrides, query, meta, 0).Edges {
		if edge.PairType == "human_agent" {
			interactionDays[normalizedPairKey(edge.Source, edge.Target)] = edge.InteractionDays
		}
	}
	for key, acc := range pairs {
		point := RampPoint{Key: key, Human: acc.human.IdentitySummary, Agent: acc.agent.IdentitySummary, Episodes: acc.episodes, Completed: acc.completed, InteractionDays: interactionDays[normalizedPairKey(acc.human.Key, acc.agent.Key)], Baseline: acc.pre, After: acc.after, AbsoluteChange: acc.after - acc.pre, Mature: acc.completed == acc.episodes, RankEligible: acc.completed >= 3}
		if acc.pre > 0 {
			value := (acc.after - acc.pre) / acc.pre
			point.ObservedLift = &value
		}
		if acc.pre > 0 && acc.after > 0 {
			value := float64(acc.preReverts)/acc.pre - float64(acc.afterReverts)/acc.after
			point.QualityDelta = &value
		}
		result.Handoffs = append(result.Handoffs, point)
	}
	sort.Slice(result.Handoffs, func(i, j int) bool { return result.Handoffs[i].Episodes > result.Handoffs[j].Episodes })
	sort.Slice(result.Adoptions, func(i, j int) bool { return result.Adoptions[i].AdoptedAt < result.Adoptions[j].AdoptedAt })
	return result
}

func buildRankings(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery, meta InsightMeta) (RankingResponse, error) {
	meta.Coverage = insightCoverage(reports, overrides, query)
	if query.Cohort == "" {
		query.Cohort = "humans"
	}
	validCohort := query.Cohort == "humans" || query.Cohort == "agents" || query.Cohort == "human_agent" || query.Cohort == "human_human"
	if !validCohort {
		return RankingResponse{}, errors.New("cohort must be humans, agents, human_agent, or human_human")
	}
	if query.Metric == "" {
		if query.Cohort == "humans" || query.Cohort == "agents" {
			query.Metric = "commits"
		} else {
			query.Metric = "interaction_days"
		}
	}
	individual := query.Cohort == "humans" || query.Cohort == "agents"
	individualMetric := query.Metric == "commits" || query.Metric == "pull_requests" || query.Metric == "revert_rate" || query.Metric == "retained_line_rate"
	pairMetric := query.Metric == "interaction_days" || query.Metric == "handoffs" || query.Metric == "review_interactions"
	if (individual && !individualMetric) || (!individual && !pairMetric) {
		return RankingResponse{}, errors.New("ranking metric is not available for this cohort")
	}
	direction := "higher"
	if query.Metric == "revert_rate" {
		direction = "lower"
	}
	response := RankingResponse{Meta: meta, Cohort: query.Cohort, Metric: query.Metric, FavorableDirection: direction, Leaderboard: []RankEntry{}, Trajectories: []RankSeries{}}
	values, labels, kinds, metrics := rankValues(reports, overrides, query)
	response.Leaderboard = orderedRanks(values, labels, kinds, metrics, direction)
	response.Meta.TotalResults = len(response.Leaderboard)
	if len(response.Leaderboard) > 100 {
		response.Leaderboard = response.Leaderboard[:100]
		response.Meta.Truncated = true
	}
	if query.From == nil {
		return response, nil
	}
	top := response.Leaderboard
	if len(top) > 10 {
		top = top[:10]
	}
	series := make(map[string]*RankSeries)
	for _, entry := range top {
		series[entry.Key] = &RankSeries{Key: entry.Key, Label: entry.Label}
	}
	for bucket := activityBucketStart(*query.From, meta.Granularity); !bucket.After(*query.To); bucket = nextActivityBucket(bucket, meta.Granularity) {
		end := nextActivityBucket(bucket, meta.Granularity).Add(-time.Nanosecond)
		if end.After(*query.To) {
			end = *query.To
		}
		bucketQuery := query
		start := bucket
		bucketQuery.From = &start
		bucketQuery.To = &end
		bucketValues, bucketLabels, bucketKinds, bucketMetrics := rankValues(reports, overrides, bucketQuery)
		ranks := orderedRanks(bucketValues, bucketLabels, bucketKinds, bucketMetrics, direction)
		for _, rank := range ranks {
			if target := series[rank.Key]; target != nil {
				target.Points = append(target.Points, RankPoint{Date: bucket.Format(time.DateOnly), Rank: rank.Rank, Value: rank.Value})
			}
		}
	}
	for _, entry := range top {
		response.Trajectories = append(response.Trajectories, *series[entry.Key])
	}
	return response, nil
}

func rankValues(reports map[int64]RepositoryReport, overrides map[string]IdentityOverride, query InsightQuery) (map[string]float64, map[string]string, map[string]ActorKind, map[string]map[string]float64) {
	values := make(map[string]float64)
	labels := make(map[string]string)
	kinds := make(map[string]ActorKind)
	metrics := make(map[string]map[string]float64)
	if query.ActorKind == ActorUnknown || query.ActorKind == ActorHuman && query.Cohort == "agents" || query.ActorKind == ActorAgent && (query.Cohort == "humans" || query.Cohort == "human_human") {
		return values, labels, kinds, metrics
	}
	if query.Cohort == "human_agent" || query.Cohort == "human_human" {
		_, meta, _ := prepareInsightQuery(reports, query)
		network := buildNetworkWithLimit(reports, overrides, query, meta, 0)
		names := map[string]string{}
		for _, node := range network.Nodes {
			names[node.Key] = node.Name
		}
		for _, edge := range network.Edges {
			if edge.PairType != query.Cohort {
				continue
			}
			key := edge.Source + "\x00" + edge.Target
			labels[key] = names[edge.Source] + " × " + names[edge.Target]
			kinds[key] = ActorUnknown
			metrics[key] = map[string]float64{"interaction_days": float64(edge.InteractionDays), "handoffs": float64(edge.Handoffs), "review_interactions": float64(edge.ReviewInteractions)}
			values[key] = metrics[key][query.Metric]
		}
		return values, labels, kinds, metrics
	}
	catalog := buildIdentityCatalog(reports, overrides)
	aliases := identityAliasIndex(catalog)
	totals := make(map[string]float64)
	matureTotals := make(map[string]float64)
	reverts := make(map[string]float64)
	pulls := make(map[string]float64)
	retained := make(map[string]float64)
	added := make(map[string]float64)
	retentionSamples := make(map[string]int)
	maturityCutoff := insightMaturityCutoff(query)
	for _, report := range reports {
		if !insightReportMatches(report, query) {
			continue
		}
		for _, event := range report.Commits.Events {
			if !inInsightRange(event.CommittedAt, query) {
				continue
			}
			participants := knownEventIdentities(event, catalog, aliases, overrides)
			mature := !maturityCutoff.IsZero() && !dayUTC(event.CommittedAt.AddDate(0, 0, query.SurvivalDays)).After(maturityCutoff)
			for _, identity := range participants {
				if (query.Cohort == "humans" && identity.Kind != ActorHuman) || (query.Cohort == "agents" && identity.Kind != ActorAgent) {
					continue
				}
				totals[identity.Key]++
				if mature {
					matureTotals[identity.Key]++
				}
				if mature && eventIsExplicitRevert(event) {
					reverts[identity.Key]++
				}
				if query.SurvivalDays == 30 && event.RetentionMeasured && event.LinesAdded > 0 && !maturityCutoff.IsZero() && !dayUTC(event.CommittedAt.AddDate(0, 0, query.SurvivalDays)).After(maturityCutoff) {
					retained[identity.Key] += float64(event.RetainedLines)
					added[identity.Key] += float64(event.LinesAdded)
					retentionSamples[identity.Key]++
				}
			}
		}
	}
	for _, report := range reports {
		if !insightReportMatches(report, query) || report.Pulls == nil {
			continue
		}
		for _, pull := range report.Pulls.PullRequests {
			if !inInsightRange(pull.CreatedAt, query) {
				continue
			}
			identity := catalog[canonicalIdentityKey(personMetrics(pull.Author).Key, overrides)]
			if identity == nil || (query.Cohort == "humans" && identity.Kind != ActorHuman) || (query.Cohort == "agents" && identity.Kind != ActorAgent) {
				continue
			}
			pulls[identity.Key]++
		}
	}
	for key, total := range totals {
		identity := catalog[key]
		labels[key] = identity.Name
		kinds[key] = identity.Kind
		metrics[key] = map[string]float64{"commits": total, "pull_requests": pulls[key], "revert_rate": 0, "retained_line_rate": 0}
		if matureTotals[key] > 0 {
			metrics[key]["revert_rate"] = reverts[key] / matureTotals[key]
		}
		if added[key] > 0 {
			metrics[key]["retained_line_rate"] = retained[key] / added[key]
		}
		if query.Metric == "revert_rate" && matureTotals[key] < 5 {
			continue
		}
		if query.Metric == "retained_line_rate" && retentionSamples[key] < 5 {
			continue
		}
		values[key] = metrics[key][query.Metric]
	}
	if query.Metric == "pull_requests" {
		for key, total := range pulls {
			if _, exists := values[key]; exists {
				continue
			}
			identity := catalog[key]
			if identity == nil {
				continue
			}
			labels[key], kinds[key] = identity.Name, identity.Kind
			metrics[key] = map[string]float64{"commits": totals[key], "pull_requests": total, "revert_rate": 0, "retained_line_rate": 0}
			values[key] = total
		}
	}
	return values, labels, kinds, metrics
}

func orderedRanks(values map[string]float64, labels map[string]string, kinds map[string]ActorKind, metrics map[string]map[string]float64, direction string) []RankEntry {
	entries := make([]RankEntry, 0, len(values))
	for key, value := range values {
		entries = append(entries, RankEntry{Key: key, Label: labels[key], Kind: kinds[key], Value: value, Eligible: true, Metrics: metrics[key]})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Value == entries[j].Value {
			return entries[i].Label < entries[j].Label
		}
		if direction == "lower" {
			return entries[i].Value < entries[j].Value
		}
		return entries[i].Value > entries[j].Value
	})
	for i := range entries {
		entries[i].Rank = i + 1
	}
	return entries
}

func isExplicitRevert(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.HasPrefix(lower, "revert ") || strings.Contains(lower, "this reverts commit ")
}

func eventIsExplicitRevert(event CommitEvent) bool {
	return event.ExplicitRevert || isExplicitRevert(event.Message)
}

func pullHasApproval(pull PullRequest) bool {
	return pullHasApprovalFrom(pull, nil)
}

func pullHasKnownApproval(pull PullRequest, catalog map[string]*resolvedIdentity, overrides map[string]IdentityOverride) bool {
	return pullHasApprovalFrom(pull, func(person Person) bool {
		return isKnownIdentity(catalog[canonicalIdentityKey(personMetrics(person).Key, overrides)])
	})
}

func pullHasApprovalFrom(pull PullRequest, eligible func(Person) bool) bool {
	latest := make(map[string]string)
	authorKey := strings.ToLower(strings.TrimSpace(pull.Author.Login))
	for _, review := range pull.Reviews {
		if pull.MergedAt != nil && review.SubmittedAt != nil && review.SubmittedAt.After(*pull.MergedAt) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(review.Author.Login))
		if key == "" || key == authorKey || eligible != nil && !eligible(review.Author) {
			continue
		}
		latest[key] = strings.ToUpper(strings.TrimSpace(review.State))
	}
	for _, state := range latest {
		if state == "APPROVED" {
			return true
		}
	}
	return false
}

func dayUTC(value time.Time) time.Time {
	value = value.UTC()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	middle := len(copyValues) / 2
	if len(copyValues)%2 == 0 {
		return (copyValues[middle-1] + copyValues[middle]) / 2
	}
	return copyValues[middle]
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func removeString(values []string, target string) []string {
	result := values[:0]
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}
