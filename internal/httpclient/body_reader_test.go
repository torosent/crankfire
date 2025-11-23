package httpclient

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/torosent/crankfire/internal/config"
)

func TestNewBodySource(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		_, err := NewBodySource(nil)
		if err == nil {
			t.Error("NewBodySource(nil) error = nil, want error")
		}
	})

	t.Run("both body and body file", func(t *testing.T) {
		cfg := &config.Config{
			Body:     "inline",
			BodyFile: "file.txt",
		}
		_, err := NewBodySource(cfg)
		if err == nil {
			t.Error("NewBodySource(both) error = nil, want error")
		}
	})

	t.Run("inline body", func(t *testing.T) {
		content := "hello world"
		cfg := &config.Config{Body: content}
		source, err := NewBodySource(cfg)
		if err != nil {
			t.Fatalf("NewBodySource(inline) error = %v", err)
		}

		// Check ContentLength
		if length, ok := source.ContentLength(); !ok || length != int64(len(content)) {
			t.Errorf("ContentLength() = %d, %v; want %d, true", length, ok, len(content))
		}

		// Check Reader
		rc, err := source.NewReader()
		if err != nil {
			t.Fatalf("NewReader() error = %v", err)
		}
		defer rc.Close()

		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if string(got) != content {
			t.Errorf("ReadAll() = %q, want %q", string(got), content)
		}
	})

	t.Run("file body", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "body.txt")
		content := "file content"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		cfg := &config.Config{BodyFile: path}
		source, err := NewBodySource(cfg)
		if err != nil {
			t.Fatalf("NewBodySource(file) error = %v", err)
		}

		// Check ContentLength
		if length, ok := source.ContentLength(); !ok || length != int64(len(content)) {
			t.Errorf("ContentLength() = %d, %v; want %d, true", length, ok, len(content))
		}

		// Check Reader
		rc, err := source.NewReader()
		if err != nil {
			t.Fatalf("NewReader() error = %v", err)
		}
		defer rc.Close()

		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if string(got) != content {
			t.Errorf("ReadAll() = %q, want %q", string(got), content)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		cfg := &config.Config{BodyFile: "/nonexistent/file"}
		_, err := NewBodySource(cfg)
		if err == nil {
			t.Error("NewBodySource(missing file) error = nil, want error")
		}
	})

	t.Run("directory as file", func(t *testing.T) {
		dir := t.TempDir()
		cfg := &config.Config{BodyFile: dir}
		_, err := NewBodySource(cfg)
		if err == nil {
			t.Error("NewBodySource(directory) error = nil, want error")
		}
	})

	t.Run("empty source", func(t *testing.T) {
		cfg := &config.Config{}
		source, err := NewBodySource(cfg)
		if err != nil {
			t.Fatalf("NewBodySource(empty) error = %v", err)
		}

		if length, ok := source.ContentLength(); !ok || length != 0 {
			t.Errorf("ContentLength() = %d, %v; want 0, true", length, ok)
		}

		rc, err := source.NewReader()
		if err != nil {
			t.Fatalf("NewReader() error = %v", err)
		}
		defer rc.Close()

		got, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ReadAll() = %q, want empty", string(got))
		}
	})
}

func TestFileBodySource_OpenError(t *testing.T) {
	// Create a file with no permissions
	dir := t.TempDir()
	path := filepath.Join(dir, "noperms.txt")
	if err := os.WriteFile(path, []byte("content"), 0000); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// On Windows, 0000 might not prevent reading, so we skip if we can read it.
	// But for Linux/macOS it should fail.
	f, err := os.Open(path)
	if err == nil {
		f.Close()
		t.Skip("Skipping permission test as file is readable")
	}

	cfg := &config.Config{BodyFile: path}
	// NewBodySource checks Stat, which might succeed even if Open fails later.
	// But Stat on 0000 file usually works.
	source, err := NewBodySource(cfg)
	if err != nil {
		// If NewBodySource fails (e.g. Stat fails), that's also fine for this test intent,
		// but we specifically want to test NewReader failure.
		// If Stat fails, we can't test NewReader.
		t.Logf("NewBodySource failed: %v", err)
		return
	}

	_, err = source.NewReader()
	if err == nil {
		t.Error("NewReader(noperms) error = nil, want error")
	}
}
