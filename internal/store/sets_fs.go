package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	cron "github.com/robfig/cron/v3"
	"gopkg.in/yaml.v3"
)

func (s *fsStore) ListSets(ctx context.Context) ([]Set, error) {
	entries, err := os.ReadDir(setsDir(s.dir))
	if err != nil {
		return nil, fmt.Errorf("read sets dir: %w", err)
	}
	var out []Set
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yaml")
		set, err := s.GetSet(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, set)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *fsStore) GetSet(_ context.Context, id string) (Set, error) {
	if err := validateID(id); err != nil {
		return Set{}, fmt.Errorf("get set: %w", err)
	}
	data, err := os.ReadFile(setPath(s.dir, id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Set{}, fmt.Errorf("%w: %s", ErrInvalidSet, id)
		}
		return Set{}, fmt.Errorf("read set: %w", err)
	}
	var set Set
	if err := yaml.Unmarshal(data, &set); err != nil {
		return Set{}, fmt.Errorf("unmarshal set: %w", err)
	}
	return set, nil
}

func (s *fsStore) SaveSet(ctx context.Context, set Set) error {
	if set.ID == "" {
		set.ID = newULID()
	}
	if err := validateID(set.ID); err != nil {
		return fmt.Errorf("save set: %w", err)
	}
	if err := s.validateSetContents(ctx, set); err != nil {
		return err
	}
	now := time.Now().UTC()
	if set.CreatedAt.IsZero() {
		set.CreatedAt = now
	}
	set.UpdatedAt = now
	set.SchemaVersion = SchemaVersion

	if set.Schedule != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		if _, err := parser.Parse(set.Schedule); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidSchedule, err)
		}
	}

	lock := flock.New(setPath(s.dir, set.ID) + ".lock")
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("lock set: %w", err)
	}
	defer func() { _ = lock.Unlock() }()

	data, err := yaml.Marshal(&set)
	if err != nil {
		return fmt.Errorf("marshal set: %w", err)
	}
	return writeAtomic(setPath(s.dir, set.ID), data)
}

func (s *fsStore) validateSetContents(ctx context.Context, set Set) error {
	if strings.TrimSpace(set.Name) == "" {
		return fmt.Errorf("%w: name required", ErrInvalidSet)
	}
	if len(set.Name) > 200 {
		return fmt.Errorf("%w: name exceeds 200 chars", ErrInvalidSet)
	}
	if len(set.Stages) == 0 {
		return fmt.Errorf("%w: at least one stage required", ErrInvalidSet)
	}
	for si, stage := range set.Stages {
		if len(stage.Items) == 0 {
			return fmt.Errorf("%w: stage %d (%q) has no items", ErrInvalidSet, si, stage.Name)
		}
		seen := map[string]bool{}
		for _, item := range stage.Items {
			name := item.Name
			if name == "" {
				name = item.SessionID
			}
			if seen[name] {
				return fmt.Errorf("%w: duplicate item name %q in stage %q", ErrInvalidSet, name, stage.Name)
			}
			seen[name] = true
			if _, err := s.GetSession(ctx, item.SessionID); err != nil {
				return fmt.Errorf("%w: stage %q item %q references unknown session %q", ErrInvalidSet, stage.Name, name, item.SessionID)
			}
		}
		if stage.OnFailure != "" && stage.OnFailure != OnFailureAbort && stage.OnFailure != OnFailureContinue {
			return fmt.Errorf("%w: stage %q on_failure must be abort|continue, got %q", ErrInvalidSet, stage.Name, stage.OnFailure)
		}
	}
	for ti, th := range set.Thresholds {
		if th.Metric == "" || th.Op == "" {
			return fmt.Errorf("%w: threshold %d missing metric or op", ErrInvalidSet, ti)
		}
	}
	return nil
}

func (s *fsStore) DeleteSet(_ context.Context, id string) error {
	if err := validateID(id); err != nil {
		return fmt.Errorf("delete set: %w", err)
	}
	if err := os.Remove(setPath(s.dir, id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove set: %w", err)
	}
	return nil
}

func (s *fsStore) CreateSetRun(_ context.Context, setID string) (SetRun, error) {
	if err := validateID(setID); err != nil {
		return SetRun{}, fmt.Errorf("create set run: %w", err)
	}
	ts := time.Now().UTC().Format("2006-01-02T15-04-05.000000000Z")
	dir := setRunDir(s.dir, setID, ts)
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		return SetRun{}, fmt.Errorf("mkdir set run: %w", err)
	}
	run := SetRun{
		SchemaVersion: SchemaVersion,
		SetID:         setID,
		StartedAt:     time.Now().UTC(),
		Status:        SetRunRunning,
		Dir:           dir,
	}
	if err := s.writeSetRunJSON(run); err != nil {
		return SetRun{}, err
	}
	return run, nil
}

func (s *fsStore) FinalizeSetRun(_ context.Context, run SetRun) error {
	if err := validateID(run.SetID); err != nil {
		return fmt.Errorf("finalize set run: %w", err)
	}
	if run.Dir == "" {
		return fmt.Errorf("%w: missing run dir", ErrInvalidSet)
	}
	if run.EndedAt.IsZero() {
		run.EndedAt = time.Now().UTC()
	}
	return s.writeSetRunJSON(run)
}

func (s *fsStore) writeSetRunJSON(run SetRun) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal set run: %w", err)
	}
	return writeAtomic(filepath.Join(run.Dir, "set-run.json"), data)
}

func (s *fsStore) ListSetRuns(_ context.Context, setID string) ([]SetRun, error) {
	if err := validateID(setID); err != nil {
		return nil, fmt.Errorf("list set runs: %w", err)
	}
	root := filepath.Join(setRunsDir(s.dir), setID)
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read set runs dir: %w", err)
	}
	var out []SetRun
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "set-run.json"))
		if err != nil {
			continue
		}
		var run SetRun
		if err := json.Unmarshal(data, &run); err != nil {
			continue
		}
		run.Dir = dir
		out = append(out, run)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}
