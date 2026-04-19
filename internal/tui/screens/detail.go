package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/store"
)

type Detail struct {
	store store.Store
	sess  store.Session
}

func NewDetail(s store.Store, sess store.Session) Detail { return Detail{store: s, sess: sess} }
func (d Detail) Init() tea.Cmd { return nil }
func (d Detail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			return d, popCmd
		case "r":
			return d, func() tea.Msg {
				return PushMsg{NewRun(d.store, d.sess)}
			}
		}
	}
	return d, nil
}
func (d Detail) View() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Session: %s\nID: %s\n%s\n\nTarget: %s\nProtocol: %s\nTotal: %d\n",
		d.sess.Name, d.sess.ID, d.sess.Description, d.sess.Config.TargetURL, d.sess.Config.Protocol, d.sess.Config.Total)
	b.WriteString("\n[r] run  [e] edit  [h] history  [Esc] back\n")
	return b.String()
}

func popCmd() tea.Msg { return PopMsg{} }
