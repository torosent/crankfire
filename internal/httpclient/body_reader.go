package httpclient

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/torosent/crankfire/internal/config"
)

type BodySource interface {
	NewReader() (io.ReadCloser, error)
	ContentLength() (int64, bool)
}

func NewBodySource(cfg *config.Config) (BodySource, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	if cfg.Body != "" && strings.TrimSpace(cfg.BodyFile) != "" {
		return nil, errors.New("body and body file cannot both be provided")
	}

	if cfg.Body != "" {
		return &inlineBodySource{data: []byte(cfg.Body)}, nil
	}

	bodyFile := strings.TrimSpace(cfg.BodyFile)
	if bodyFile != "" {
		info, err := os.Stat(bodyFile)
		if err != nil {
			return nil, fmt.Errorf("body file: %w", err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("body file %q is a directory", bodyFile)
		}
		return &fileBodySource{path: bodyFile, size: info.Size()}, nil
	}

	return emptyBodySource{}, nil
}

type inlineBodySource struct {
	data []byte
}

func (s *inlineBodySource) NewReader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

func (s *inlineBodySource) ContentLength() (int64, bool) {
	return int64(len(s.data)), true
}

type fileBodySource struct {
	path string
	size int64
}

func (s *fileBodySource) NewReader() (io.ReadCloser, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *fileBodySource) ContentLength() (int64, bool) {
	return s.size, true
}

type emptyBodySource struct{}

func (emptyBodySource) NewReader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (emptyBodySource) ContentLength() (int64, bool) {
	return 0, true
}
