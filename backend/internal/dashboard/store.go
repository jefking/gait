package dashboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Store struct {
	root string
}

func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("data directory is required")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	for _, directory := range []string{absolute, filepath.Join(absolute, "repos"), filepath.Join(absolute, "reports")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, fmt.Errorf("create data directory %q: %w", directory, err)
		}
	}
	return &Store{root: absolute}, nil
}

func (store *Store) RepoPath(repositoryID int64) string {
	return filepath.Join(store.root, "repos", strconv.FormatInt(repositoryID, 10))
}

func (store *Store) reportDir(repositoryID int64) string {
	return filepath.Join(store.root, "reports", strconv.FormatInt(repositoryID, 10))
}

func (store *Store) commitPath(repositoryID int64) string {
	return filepath.Join(store.reportDir(repositoryID), "commits.csv")
}

func (store *Store) pullPath(repositoryID int64) string {
	return filepath.Join(store.reportDir(repositoryID), "pulls.json")
}

func (store *Store) NewCommitTemp(repositoryID int64) (*os.File, error) {
	directory := store.reportDir(repositoryID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create repository report directory: %w", err)
	}
	file, err := os.CreateTemp(directory, "commits-*.csv")
	if err != nil {
		return nil, fmt.Errorf("create temporary commit report: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, fmt.Errorf("protect temporary commit report: %w", err)
	}
	return file, nil
}

func (store *Store) PromoteCommitTemp(repositoryID int64, temporaryPath string) error {
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return fmt.Errorf("protect commit report: %w", err)
	}
	if err := os.Rename(temporaryPath, store.commitPath(repositoryID)); err != nil {
		return fmt.Errorf("publish commit report: %w", err)
	}
	return nil
}

func (store *Store) LoadCommitStats(repositoryID int64) (CommitStats, error) {
	file, err := os.Open(store.commitPath(repositoryID))
	if err != nil {
		return CommitStats{}, err
	}
	defer file.Close()
	return ParseCommitCSV(file)
}

func (store *Store) LoadPullCache(repositoryID int64) (PullCache, error) {
	var cache PullCache
	file, err := os.Open(store.pullPath(repositoryID))
	if err != nil {
		return cache, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&cache); err != nil {
		return PullCache{}, fmt.Errorf("decode pull request cache: %w", err)
	}
	return cache, nil
}

func (store *Store) SavePullCache(repositoryID int64, cache PullCache) error {
	return store.writeJSON(store.pullPath(repositoryID), cache)
}

func (store *Store) LoadSnapshot() (*Snapshot, error) {
	file, err := os.Open(filepath.Join(store.root, "snapshot.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open dashboard snapshot: %w", err)
	}
	defer file.Close()
	var snapshot Snapshot
	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("decode dashboard snapshot: %w", err)
	}
	return &snapshot, nil
}

func (store *Store) SaveSnapshot(snapshot *Snapshot) error {
	return store.writeJSON(filepath.Join(store.root, "snapshot.json"), snapshot)
}

func (store *Store) LoadReports(snapshot *Snapshot) (map[int64]RepositoryReport, []string) {
	reports := make(map[int64]RepositoryReport)
	if snapshot == nil {
		return reports, nil
	}
	warnings := make([]string, 0)
	for _, summary := range snapshot.Repositories {
		repository := repositoryFromSummary(summary)
		report := RepositoryReport{
			Repository:  repository,
			SyncStatus:  summary.SyncStatus,
			SyncMessage: summary.SyncMessage,
		}
		commits, err := store.LoadCommitStats(repository.ID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			warnings = append(warnings, fmt.Sprintf("%s: cached commit report could not be loaded", repository.FullName))
		} else if err == nil {
			report.Commits = commits
		}
		if summary.PullRequests != nil {
			cache, loadErr := store.LoadPullCache(repository.ID)
			if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
				warnings = append(warnings, fmt.Sprintf("%s: cached pull request report could not be loaded", repository.FullName))
			} else if loadErr == nil {
				pullStats := BuildPullStats(cache.PullRequests)
				report.Pulls = &pullStats
			}
		}
		reports[repository.ID] = report
	}
	return reports, warnings
}

func repositoryFromSummary(summary RepositorySummary) Repository {
	return Repository{
		ID:            summary.ID,
		Name:          summary.Name,
		FullName:      summary.FullName,
		HTMLURL:       summary.HTMLURL,
		Description:   summary.Description,
		DefaultBranch: summary.DefaultBranch,
		Private:       summary.Private,
		Archived:      summary.Archived,
		Fork:          summary.Fork,
		Owner:         summary.Owner,
	}
}

func (store *Store) writeJSON(path string, value any) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create JSON data directory: %w", err)
	}
	file, err := os.CreateTemp(directory, ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temporary JSON file: %w", err)
	}
	temporaryPath := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(temporaryPath)
	}
	if err := file.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("protect temporary JSON file: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		cleanup()
		return fmt.Errorf("encode JSON file: %w", err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync JSON file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("close JSON file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return fmt.Errorf("publish JSON file: %w", err)
	}
	return nil
}
