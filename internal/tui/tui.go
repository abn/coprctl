// Package tui implements the interactive project dashboard using Bubble Tea.
// It is a view over the same data the CLI renders, and degrades to plain
// output when stdout is not a TTY. Everything the dashboard shows has a
// printable command behind it.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/abn/coprctl/internal/copr"
)

// PollMsg triggers a monitor refresh.
type PollMsg struct{}

// model is the dashboard state.
type model struct {
	client   *copr.Client
	ctx      context.Context
	owner    string
	project  string
	dir      string
	rows     []copr.MonitorRow
	viewport viewport.Model
	width    int
	height   int
	err      error
}

// Run starts the dashboard, blocking until it exits. It is only called when
// stdout is a TTY. The context cancels in-flight monitor polls and the
// program's refresh loop. dir selects a side repo; an empty dir monitors the
// project's main directory.
func Run(ctx context.Context, client *copr.Client, owner, project, dir string) error {
	m := &model{
		client:  client,
		ctx:     ctx,
		owner:   owner,
		project: project,
		dir:     dir,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init kicks off the first poll.
func (m *model) Init() tea.Cmd {
	return m.poll()
}

// Update handles messages and key presses.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport = viewport.New(msg.Width, msg.Height-3)
		m.viewport.SetContent(m.render())
		return m, nil
	case PollMsg:
		if err := m.refresh(); err != nil {
			m.err = err
		}
		m.viewport.SetContent(m.render())
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return PollMsg{} })
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			m.viewport.LineDown(1)
		case "k", "up":
			m.viewport.LineUp(1)
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the dashboard.
func (m *model) View() string {
	return m.viewport.View()
}

func (m *model) poll() tea.Cmd {
	return func() tea.Msg { return PollMsg{} }
}

func (m *model) refresh() error {
	rows, err := m.client.Monitor(m.ctx, m.owner, m.project, m.dir)
	if err != nil {
		return err
	}
	m.rows = rows
	return nil
}

func (m *model) render() string {
	var b strings.Builder
	ref := m.owner + "/" + m.project
	if m.dir != "" {
		ref += ":" + m.dir
	}
	title := lipgloss.NewStyle().Bold(true).Padding(0, 1).Render("coprctl: " + ref)
	b.WriteString(title + "\n\n")
	header := lipgloss.NewStyle().Bold(true).
		Padding(0, 1).Render("PACKAGE\tCHROOT\tSTATE\tBUILD\tVERSION")
	b.WriteString(header + "\n")
	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(
			"error: "+m.err.Error()) + "\n")
	}
	for _, row := range m.rows {
		for ch, info := range row.Chroots {
			state := info.State
			color := "8"
			switch state {
			case "succeeded":
				color = "2"
			case "failed":
				color = "1"
			case "running", "starting":
				color = "3"
			}
			stateSty := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(state)
			build := "-"
			if info.BuildID != 0 {
				build = fmt.Sprintf("%d", info.BuildID)
			}
			b.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\t%s\n",
				row.Name, ch, stateSty, build, info.PkgVersion))
		}
	}
	b.WriteString("\n  j/k scroll   q quit\n")
	return b.String()
}
