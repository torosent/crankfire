// internal/store/fs.go
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/oklog/ulid/v2"
	"gopkg.in/yaml.v3"
)

type fsStore struct{ dir string }

func NewFS(dataDir string) (Store, error) {
	for _, d := range []string{dataDir, sessionsDir(dataDir), runsDir(dataDir)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return &fsStore{dir: dataDir}, nil
}

func newULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulid.DefaultEntropy()).String()
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
	if sess.ID == "" { sess.ID = newULID() }
	if sess.SchemaVersion == 0 { sess.SchemaVersion = SchemaVersion }
	if sess.CreatedAt.IsZero() { sess.CreatedAt = time.Now().UTC() }
	sess.UpdatedAt = time.Now().UTC()

	data, err := yaml.Marshal(&sess)
	if err != nil { return fmt.Errorf("marshal session: %w", err) }

	path := sessionPath(s.dir, sess.ID)
	lk := flock.New(path + ".lock")
	if err := lk.Lock(); err != nil { return fmt.Errorf("lock %s: %w", path, err) }
	defer lk.Unlock()
	return writeAtomic(path, data)
}

func (s *fsStore) DeleteSession(ctx context.Context, id string) error {
	err := os.Remove(sessionPath(s.dir, id))
	if errors.Is(err, os.ErrNotExist) { return ErrNotFound }
	if err != nil { return fmt.Errorf("delete session %s: %w", id, err) }
	_ = os.Remove(sessionPath(s.dir, id) + ".lock")
	return nil
}

func (s *fsStore) ImportSessionFromConfigFile(ctx context.Context, path, name string) (Session, error) {
	return Session{}, fmt.Errorf("not implemented") // Task 5
}
func (s *fsStore) ListRuns(ctx context.Context, sessionID string) ([]Run, error) {
	return nil, fmt.Errorf("not implemented") // Task 6
}
func (s *fsStore) CreateRun(ctx context.Context, sessionID string) (Run, error) {
	return Run{}, fmt.Errorf("not implemented") // Task 6
}
func (s *fsStore) FinalizeRun(ctx context.Context, run Run, summary RunSummary) error {
	return fmt.Errorf("not implemented") // Task 6
}

var _ = filepath.Join // keep imports tidy across tasks
