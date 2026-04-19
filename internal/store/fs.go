// internal/store/fs.go
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/oklog/ulid/v2"
	"github.com/torosent/crankfire/internal/config"
	"gopkg.in/yaml.v3"
)

// tagRe is the validation pattern for session tag tokens.
// Mirrored in internal/tagfilter — keep in sync.
var tagRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

type fsStore struct{ dir string }

func NewFS(dataDir string) (Store, error) {
	for _, d := range []string{
		dataDir,
		sessionsDir(dataDir),
		runsDir(dataDir),
		setsDir(dataDir),
		setRunsDir(dataDir),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return &fsStore{dir: dataDir}, nil
}

func newULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulid.DefaultEntropy()).String()
}

// validateID rejects IDs that could escape the data directory or otherwise
// produce unintended file paths. Session IDs are normally ULIDs but may also
// flow in from user-edited YAML, so all callers that derive a path from an
// ID must validate first.
func validateID(id string) error {
	if id == "" {
		return fmt.Errorf("%w: empty id", ErrInvalidSession)
	}
	if id == "." || id == ".." {
		return fmt.Errorf("%w: id %q is not allowed", ErrInvalidSession, id)
	}
	if strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return fmt.Errorf("%w: id %q contains path separators or traversal", ErrInvalidSession, id)
	}
	if strings.ContainsAny(id, "\x00\n\r") {
		return fmt.Errorf("%w: id %q contains control characters", ErrInvalidSession, id)
	}
	if id != filepath.Base(id) {
		return fmt.Errorf("%w: id %q must be a single path segment", ErrInvalidSession, id)
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func (s *fsStore) ListSessions(ctx context.Context) ([]Session, error) {
	entries, err := os.ReadDir(sessionsDir(s.dir))
	if err != nil { return nil, fmt.Errorf("read sessions dir: %w", err) }
	var out []Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") { continue }
		id := strings.TrimSuffix(e.Name(), ".yaml")
		sess, err := s.GetSession(ctx, id)
		if err != nil { continue }
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *fsStore) GetSession(ctx context.Context, id string) (Session, error) {
	if err := validateID(id); err != nil {
		return Session{}, err
	}
	data, err := os.ReadFile(sessionPath(s.dir, id))
	if errors.Is(err, os.ErrNotExist) { return Session{}, ErrNotFound }
	if err != nil { return Session{}, fmt.Errorf("read session %s: %w", id, err) }
	var sess Session
	if err := yaml.Unmarshal(data, &sess); err != nil {
		return Session{}, fmt.Errorf("unmarshal session %s: %w: %w", id, ErrInvalidSession, err)
	}
	return sess, nil
}

func (s *fsStore) SaveSession(ctx context.Context, sess Session) error {
	if sess.ID == "" {
		sess.ID = newULID()
	} else if err := validateID(sess.ID); err != nil {
		return err
	}
	if sess.SchemaVersion == 0 { sess.SchemaVersion = SchemaVersion }
	if sess.CreatedAt.IsZero() { sess.CreatedAt = time.Now().UTC() }
	sess.UpdatedAt = time.Now().UTC()

	// validate, dedupe, sort tags
	if len(sess.Tags) > 0 {
		seen := make(map[string]struct{}, len(sess.Tags))
		cleaned := make([]string, 0, len(sess.Tags))
		for _, tag := range sess.Tags {
			if !tagRe.MatchString(tag) {
				return fmt.Errorf("%w: %q", ErrInvalidTag, tag)
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			cleaned = append(cleaned, tag)
		}
		sort.Strings(cleaned)
		sess.Tags = cleaned
	} else {
		sess.Tags = nil // normalize empty slice to nil for clean YAML
	}

	data, err := yaml.Marshal(&sess)
	if err != nil { return fmt.Errorf("marshal session: %w", err) }

	path := sessionPath(s.dir, sess.ID)
	lk := flock.New(path + ".lock")
	if err := lk.Lock(); err != nil { return fmt.Errorf("lock %s: %w", path, err) }
	defer lk.Unlock()
	return writeAtomic(path, data)
}

func (s *fsStore) DeleteSession(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	err := os.Remove(sessionPath(s.dir, id))
	if errors.Is(err, os.ErrNotExist) { return ErrNotFound }
	if err != nil { return fmt.Errorf("delete session %s: %w", id, err) }
	_ = os.Remove(sessionPath(s.dir, id) + ".lock")
	return nil
}

func (s *fsStore) ImportSessionFromConfigFile(ctx context.Context, path, name string) (Session, error) {
	cfg, err := config.LoadFromFile(path)
	if err != nil {
		return Session{}, fmt.Errorf("load %s: %w: %w", path, ErrInvalidConfig, err)
	}
	sess := Session{Name: name, Config: *cfg}
	if err := s.SaveSession(ctx, sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}
func (s *fsStore) CreateRun(ctx context.Context, sessionID string) (Run, error) {
	if err := validateID(sessionID); err != nil {
		return Run{}, err
	}
	if _, err := s.GetSession(ctx, sessionID); err != nil {
		return Run{}, err
	}
	started := time.Now().UTC()
	dir := runDir(s.dir, sessionID, started.Format(time.RFC3339Nano))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Run{}, fmt.Errorf("mkdir run dir: %w", err)
	}
	run := Run{SessionID: sessionID, StartedAt: started, Status: RunStatusRunning, Dir: dir}
	if err := s.writeRunMeta(run); err != nil {
		return Run{}, err
	}
	return run, nil
}

func (s *fsStore) FinalizeRun(ctx context.Context, run Run, summary RunSummary) error {
	run.EndedAt = time.Now().UTC()
	if run.Status == "" || run.Status == RunStatusRunning {
		run.Status = RunStatusCompleted
	}
	if summary.ErrorMessage != "" && run.Status == RunStatusCompleted {
		run.Status = RunStatusFailed
	}
	run.Summary = summary
	return s.writeRunMeta(run)
}

func (s *fsStore) writeRunMeta(run Run) error {
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}
	return writeAtomic(filepath.Join(run.Dir, "run.json"), data)
}

func (s *fsStore) ListRuns(ctx context.Context, sessionID string) ([]Run, error) {
	if err := validateID(sessionID); err != nil {
		return nil, err
	}
	base := filepath.Join(runsDir(s.dir), sessionID)
	entries, err := os.ReadDir(base)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read runs dir: %w", err)
	}
	var out []Run
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		meta, err := os.ReadFile(filepath.Join(base, e.Name(), "run.json"))
		if err != nil {
			continue
		}
		var r Run
		if err := json.Unmarshal(meta, &r); err != nil {
			continue
		}
		r.Dir = filepath.Join(base, e.Name())
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out, nil
}

func (s *fsStore) ListTemplates(ctx context.Context) ([]string, error) {
	dir := templatesDir(s.dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read templates dir: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".yaml"))
	}
	sort.Strings(out)
	return out, nil
}

func (s *fsStore) GetTemplate(ctx context.Context, id string) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(templatePath(s.dir, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", id, err)
	}
	return data, nil
}

func (s *fsStore) SaveTemplate(ctx context.Context, id string, body []byte) error {
	if err := validateID(id); err != nil {
		return err
	}
	var probe struct {
		Template bool `yaml:"template"`
	}
	if err := yaml.Unmarshal(body, &probe); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTemplate, err)
	}
	if !probe.Template {
		return fmt.Errorf("%w: missing top-level `template: true` marker", ErrInvalidTemplate)
	}
	dir := templatesDir(s.dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := templatePath(s.dir, id)
	lk := flock.New(path + ".lock")
	if err := lk.Lock(); err != nil {
		return fmt.Errorf("lock %s: %w", path, err)
	}
	defer lk.Unlock()
	return writeAtomic(path, body)
}

func (s *fsStore) DeleteTemplate(ctx context.Context, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	err := os.Remove(templatePath(s.dir, id))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete template %s: %w", id, err)
	}
	_ = os.Remove(templatePath(s.dir, id) + ".lock")
	return nil
}

var _ = filepath.Join // keep imports tidy across tasks
