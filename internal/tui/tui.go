package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
)

type Options struct{ DataDir string }

func Run(opts Options) error {
	dir, err := store.ResolveDataDir(opts.DataDir)
	if err != nil {
		return err
	}
	s, err := store.NewFS(dir)
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	p := tea.NewProgram(NewRoot(s), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}
