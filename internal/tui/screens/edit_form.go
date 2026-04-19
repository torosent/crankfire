package screens

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
	"gopkg.in/yaml.v3"
)

type Edit struct {
	store        store.Store
	sess         store.Session
	fields       []textinput.Model
	labels       []string
	focus        int
	err          error
	mode         editMode
	yaml         textarea.Model
	yamlDisabled bool
	yamlReason   string
}

type editMode int

const (
	modeForm editMode = iota
	modeYAML
)

func NewEdit(s store.Store, sess store.Session) Edit {
	labels := []string{"Name", "Target", "Protocol", "Total", "Rate", "Duration", "Concurrency", "Timeout"}
	fields := make([]textinput.Model, len(labels))
	for i := range fields {
		fields[i] = textinput.New()
		fields[i].CharLimit = 256
	}
	fields[0].SetValue(sess.Name)
	fields[1].SetValue(sess.Config.TargetURL)
	fields[2].SetValue(string(sess.Config.Protocol))
	if sess.Config.Total != 0 {
		fields[3].SetValue(strconv.Itoa(sess.Config.Total))
	}
	if sess.Config.Rate != 0 {
		fields[4].SetValue(strconv.Itoa(sess.Config.Rate))
	}
	if sess.Config.Duration != 0 {
		fields[5].SetValue(sess.Config.Duration.String())
	}
	if sess.Config.Concurrency != 0 {
		fields[6].SetValue(strconv.Itoa(sess.Config.Concurrency))
	}
	if sess.Config.Timeout != 0 {
		fields[7].SetValue(sess.Config.Timeout.String())
	}
	fields[0].Focus()

	ta := textarea.New()
	ta.CharLimit = 0 // no limit for YAML
	ta.SetHeight(10)

	return Edit{
		store: s, sess: sess, fields: fields, labels: labels,
		yaml: ta,
	}
}

func (e Edit) Init() tea.Cmd { return textinput.Blink }

// FocusIndex returns the index of the currently focused field (for tests).
func (e Edit) FocusIndex() int { return e.focus }

// Err returns the most recent validation/save error (for tests).
func (e Edit) Err() error { return e.err }

// InYAMLMode returns true if currently in YAML edit mode.
func (e Edit) InYAMLMode() bool { return e.mode == modeYAML }

func (e Edit) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		// Handle YAML-mode-specific keys
		if e.mode == modeYAML {
			switch k.Type {
			case tea.KeyF2:
				// Toggle back to form (with validation)
				return e.toggleToForm()
			case tea.KeyCtrlS:
				return e.saveFromYAML()
			case tea.KeyEsc:
				return e, popCmd
			}
			// Pass other keys to textarea
			var cmd tea.Cmd
			e.yaml, cmd = e.yaml.Update(msg)
			return e, cmd
		}

		// Form-mode-specific keys
		switch k.Type {
		case tea.KeyF2:
			// Toggle to YAML
			return e.toggleToYAML()
		case tea.KeyTab, tea.KeyDown:
			// Tab on last field: enter YAML mode
			if e.focus == len(e.fields)-1 {
				return e.toggleToYAML()
			}
			e.fields[e.focus].Blur()
			e.focus = (e.focus + 1) % len(e.fields)
			e.fields[e.focus].Focus()
			return e, nil
		case tea.KeyShiftTab, tea.KeyUp:
			e.fields[e.focus].Blur()
			e.focus = (e.focus - 1 + len(e.fields)) % len(e.fields)
			e.fields[e.focus].Focus()
			return e, nil
		case tea.KeyCtrlS:
			return e.save()
		case tea.KeyEsc:
			return e, popCmd
		}
	}
	var cmd tea.Cmd
	e.fields[e.focus], cmd = e.fields[e.focus].Update(msg)
	return e, cmd
}

func (e Edit) save() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(e.fields[0].Value())
	if name == "" {
		e.err = errors.New("name is required")
		return e, nil
	}
	cfg := e.sess.Config
	cfg.TargetURL = strings.TrimSpace(e.fields[1].Value())
	if cfg.TargetURL != "" {
		u, err := url.Parse(cfg.TargetURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			e.err = fmt.Errorf("target: invalid URL")
			return e, nil
		}
	}
	if p := strings.TrimSpace(e.fields[2].Value()); p != "" {
		switch config.Protocol(p) {
		case config.ProtocolHTTP, config.ProtocolWebSocket, config.ProtocolSSE, config.ProtocolGRPC:
			cfg.Protocol = config.Protocol(p)
		default:
			e.err = fmt.Errorf("protocol: must be one of http, websocket, sse, grpc")
			return e, nil
		}
	}
	if cfg.Protocol == "" {
		cfg.Protocol = config.ProtocolHTTP
	}
	if v := strings.TrimSpace(e.fields[3].Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			e.err = fmt.Errorf("total: %w", err)
			return e, nil
		}
		cfg.Total = n
	}
	if v := strings.TrimSpace(e.fields[4].Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			e.err = fmt.Errorf("rate: %w", err)
			return e, nil
		}
		cfg.Rate = n
	}
	if v := strings.TrimSpace(e.fields[5].Value()); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			e.err = fmt.Errorf("duration: %w", err)
			return e, nil
		}
		cfg.Duration = d
	}
	if v := strings.TrimSpace(e.fields[6].Value()); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			e.err = fmt.Errorf("concurrency: %w", err)
			return e, nil
		}
		cfg.Concurrency = n
	}
	if v := strings.TrimSpace(e.fields[7].Value()); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			e.err = fmt.Errorf("timeout: %w", err)
			return e, nil
		}
		cfg.Timeout = d
	}
	e.sess.Name = name
	e.sess.Config = cfg
	if err := e.store.SaveSession(context.Background(), e.sess); err != nil {
		e.err = err
		return e, nil
	}
	e.err = nil
	return e, popCmd
}

// toggleToYAML switches from form mode to YAML mode by marshaling the session.
func (e Edit) toggleToYAML() (tea.Model, tea.Cmd) {
	// First, validate and update session from current form fields
	name := strings.TrimSpace(e.fields[0].Value())
	if name == "" {
		e.err = errors.New("name is required to enter YAML mode")
		return e, nil
	}
	e.sess.Name = name

	// Marshal session to YAML
	data, err := yaml.Marshal(e.sess)
	if err != nil {
		e.err = fmt.Errorf("failed to marshal YAML: %w", err)
		return e, nil
	}

	e.yaml.SetValue(string(data))
	e.mode = modeYAML
	e.err = nil
	e.yamlDisabled = false
	e.yamlReason = ""
	return e, nil
}

// toggleToForm switches from YAML mode back to form mode by unmarshaling the YAML.
// If YAML is invalid, it keeps YAML mode active with a disabled toggle and error message.
func (e Edit) toggleToForm() (tea.Model, tea.Cmd) {
	yamlText := e.yaml.Value()
	var sess store.Session
	if err := yaml.Unmarshal([]byte(yamlText), &sess); err != nil {
		e.yamlDisabled = true
		e.yamlReason = fmt.Sprintf("Invalid YAML: %v", err)
		return e, nil
	}

	// Successfully parsed; update form fields from the session
	e.sess = sess
	e.fields[0].SetValue(sess.Name)
	e.fields[1].SetValue(sess.Config.TargetURL)
	e.fields[2].SetValue(string(sess.Config.Protocol))
	e.fields[3].SetValue(strconv.Itoa(sess.Config.Total))
	e.fields[4].SetValue(strconv.Itoa(sess.Config.Rate))
	e.fields[5].SetValue(sess.Config.Duration.String())
	e.fields[6].SetValue(strconv.Itoa(sess.Config.Concurrency))
	e.fields[7].SetValue(sess.Config.Timeout.String())

	e.mode = modeForm
	e.focus = 0
	e.fields[0].Focus()
	e.err = nil
	e.yamlDisabled = false
	e.yamlReason = ""
	return e, nil
}

// saveFromYAML parses the YAML, validates it, and saves to the store.
func (e Edit) saveFromYAML() (tea.Model, tea.Cmd) {
	yamlText := e.yaml.Value()
	var sess store.Session
	if err := yaml.Unmarshal([]byte(yamlText), &sess); err != nil {
		e.err = fmt.Errorf("invalid YAML: %w", err)
		e.yamlDisabled = true
		e.yamlReason = "Cannot save with invalid YAML"
		return e, nil
	}

	if sess.Name == "" {
		e.err = errors.New("name is required")
		return e, nil
	}

	if err := e.store.SaveSession(context.Background(), sess); err != nil {
		e.err = err
		return e, nil
	}

	e.sess = sess
	e.err = nil
	return e, popCmd
}

func (e Edit) View() string {
	var b strings.Builder

	if e.mode == modeYAML {
		b.WriteString("Edit session (YAML mode)\n\n")
		b.WriteString(e.yaml.View())
		if e.yamlDisabled {
			fmt.Fprintf(&b, "\n⚠ %s\n", e.yamlReason)
		}
		if e.err != nil {
			fmt.Fprintf(&b, "\nerror: %v\n", e.err)
		}
		b.WriteString("\n[Ctrl+S] save  [F2] back to form  [Esc] cancel\n")
		return b.String()
	}

	b.WriteString("Edit session\n\n")
	for i, f := range e.fields {
		fmt.Fprintf(&b, "%-13s %s\n", e.labels[i]+":", f.View())
	}
	if e.err != nil {
		fmt.Fprintf(&b, "\nerror: %v\n", e.err)
	}
	b.WriteString("\n[Tab/Shift+Tab] navigate  [F2] YAML mode  [Ctrl+S] save  [Esc] cancel\n")
	return b.String()
}
