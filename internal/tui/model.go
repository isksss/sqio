// Package tui implements the Bubble Tea terminal user interface.
package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isksss/sqio/internal/config"
	"github.com/isksss/sqio/internal/db"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/service"
)

// ConnectionEntry is a compact connection label shown by the TUI.
type ConnectionEntry struct {
	Name   string
	Driver string
	DSN    string
}

// Model contains all TUI state needed by Bubble Tea update and render cycles.
type Model struct {
	cfg              config.Config
	metadata         service.MetadataService
	executor         service.Executor
	execOpts         service.ExecOptions
	width            int
	height           int
	focus            int
	input            textinput.Model
	status           string
	result           string
	rows             [][]interface{}
	columns          []string
	rowStart         int
	detailTab        int
	noColor          bool
	objects          []string
	tables           []service.Table
	selected         int
	showHelp         bool
	addingConnection bool
	connections      []ConnectionEntry
	activeConnection string
	connInputs       []textinput.Model
	connFocus        int
}

// metadataMsg delivers asynchronous table metadata loading results.
type metadataMsg struct {
	tables []service.Table
	err    error
}

// execResultMsg delivers asynchronous SQL execution results.
type execResultMsg struct {
	result output.Result
	err    error
}

// New returns a TUI model using the default in-memory metadata service.
func New(cfg config.Config, noColor bool) Model {
	return NewWithMetadata(cfg, service.NewMetadataService(), noColor)
}

// NewWithMetadata returns a TUI model with an injected metadata service.
func NewWithMetadata(cfg config.Config, metadata service.MetadataService, noColor bool) Model {
	return NewWithServices(cfg, metadata, service.Executor{}, service.ExecOptions{Format: cfg.Query.Format, MaxRows: cfg.Query.MaxRows}, noColor)
}

// NewWithServices returns a TUI model with injectable services for production
// and tests.
func NewWithServices(cfg config.Config, metadata service.MetadataService, executor service.Executor, execOpts service.ExecOptions, noColor bool) Model {
	input := textinput.New()
	input.Placeholder = "select 1"
	input.Prompt = "sql> "
	input.Focus()
	connections := make([]ConnectionEntry, 0, len(cfg.Connections))
	for _, conn := range cfg.Connections {
		connections = append(connections, ConnectionEntry{Name: conn.Name, Driver: conn.Driver, DSN: conn.DSN})
	}
	return Model{cfg: cfg, metadata: metadata, executor: executor, execOpts: execOpts, input: input, status: "loading metadata", noColor: noColor, objects: []string{"loading"}, connections: connections}
}

// Init starts cursor blinking and initial metadata loading.
func (m Model) Init() tea.Cmd { return tea.Batch(textinput.Blink, loadMetadata(m.metadata)) }

// Update applies Bubble Tea messages to Model and returns any follow-up command.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if m.addingConnection {
			return m.updateAddConnection(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		case "a":
			m.startAddConnection()
		case "tab":
			m.focus = (m.focus + 1) % 3
		case "[", "shift+tab":
			if m.focus == 1 {
				m.detailTab--
				if m.detailTab < 0 {
					m.detailTab = len(detailTabs) - 1
				}
			}
		case "]":
			if m.focus == 1 {
				m.detailTab = (m.detailTab + 1) % len(detailTabs)
			}
		case "j", "down":
			if m.focus == 0 && m.selected < len(m.objects)-1 {
				m.selected++
			}
		case "k", "up":
			if m.focus == 0 && m.selected > 0 {
				m.selected--
			}
		case "enter":
			if m.focus == 2 {
				sql := strings.TrimSpace(m.input.Value())
				if sql == "" {
					sql = "select 1"
				}
				m.status = fmt.Sprintf("running: %s", sql)
				return m, runSQL(m.executor, m.execOpts, sql)
			}
		case "pgdown":
			if len(m.rows) > 0 {
				m.rowStart += resultWindowSize
				if m.rowStart >= len(m.rows) {
					m.rowStart = max(0, len(m.rows)-resultWindowSize)
				}
				m.result = m.renderResultWindow()
			}
		case "pgup":
			if len(m.rows) > 0 {
				m.rowStart -= resultWindowSize
				if m.rowStart < 0 {
					m.rowStart = 0
				}
				m.result = m.renderResultWindow()
			}
		}
	case execResultMsg:
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
			m.result = ""
			return m, nil
		}
		m.status = fmt.Sprintf("done: %d rows", msg.result.RowCount)
		m.columns = msg.result.Columns
		m.rows = msg.result.Rows
		m.rowStart = 0
		m.result = m.renderResultWindow()
	case metadataMsg:
		if msg.err != nil {
			m.status = "metadata error: " + msg.err.Error()
			m.objects = []string{"metadata error"}
			m.selected = 0
			return m, nil
		}
		m.objects = make([]string, 0, len(msg.tables))
		m.tables = msg.tables
		for _, t := range msg.tables {
			m.objects = append(m.objects, t.Name)
		}
		if len(m.objects) == 0 {
			m.objects = []string{"no tables"}
			m.tables = nil
		}
		m.selected = 0
		m.status = "ready"
	}
	var cmd tea.Cmd
	if m.focus == 2 {
		m.input, cmd = m.input.Update(msg)
	}
	return m, cmd
}

// View renders the current TUI frame.
func (m Model) View() string {
	if m.width == 0 {
		return "sqio\n"
	}
	treeWidth := max(20, m.width/3)
	detailWidth := max(20, m.width-treeWidth-1)
	topHeight := max(8, m.height-5)
	tree := m.panel("object tree", m.renderTree(), treeWidth, topHeight, m.focus == 0)
	detail := m.panel("detail", m.renderDetail(), detailWidth, topHeight, m.focus == 1)
	console := m.panel("sql console", m.input.View()+"\n"+m.status, m.width, 4, m.focus == 2)
	v := lipgloss.JoinHorizontal(lipgloss.Top, tree, detail) + "\n" + console
	if m.showHelp {
		v += "\n" + m.renderHelp()
	}
	if m.addingConnection {
		v += "\n" + m.renderAddConnection()
	}
	return v
}

// renderTree renders the selectable database object list.
func (m Model) renderTree() string {
	lines := make([]string, len(m.objects))
	for i, o := range m.objects {
		p := "  "
		if i == m.selected {
			p = "> "
		}
		lines[i] = p + o
	}
	return strings.Join(lines, "\n")
}

// renderDetail renders metadata and query result details for the selected
// object.
func (m Model) renderDetail() string {
	if len(m.objects) == 0 {
		return "no object"
	}
	body := fmt.Sprintf("selected: %s\nconnection: %s\n%s", m.objects[m.selected], m.currentConnectionLabel(), m.renderDetailTabs())
	if m.selected < len(m.tables) {
		t := m.tables[m.selected]
		switch detailTabs[m.detailTab] {
		case "columns":
			lines := []string{"", "columns"}
			for _, c := range t.Columns {
				f := ""
				if c.Primary {
					f += " pk"
				}
				if !c.Nullable {
					f += " not-null"
				}
				lines = append(lines, fmt.Sprintf("%s\t%s%s", c.Name, c.Type, f))
			}
			body += "\n" + strings.Join(lines, "\n")
		case "ddl":
			body += "\n\nddl\n" + t.DDL
		case "result":
			body += "\n\nresult\n" + m.result
		}
	}
	return body
}

// detailTabs defines the tab order in the detail panel.
var detailTabs = []string{"columns", "ddl", "result"}

// renderDetailTabs renders the compact detail tab selector.
func (m Model) renderDetailTabs() string {
	parts := make([]string, len(detailTabs))
	for i, t := range detailTabs {
		if i == m.detailTab {
			parts[i] = "[" + t + "]"
		} else {
			parts[i] = " " + t + " "
		}
	}
	return strings.Join(parts, " ")
}

// panel wraps content in a lipgloss border and highlights focused panels.
func (m Model) panel(title, body string, width, height int, focused bool) string {
	border := lipgloss.NormalBorder()
	if focused {
		border = lipgloss.ThickBorder()
	}
	style := lipgloss.NewStyle().Width(width-2).Height(height-2).Border(border).Padding(0, 1)
	if !m.noColor && focused {
		style = style.BorderForeground(lipgloss.Color("39"))
	}
	return style.Render(title + "\n" + body)
}

// runSQL returns a command that executes SQL with a fixed TUI timeout.
func runSQL(executor service.Executor, opts service.ExecOptions, sql string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		r, e := executor.Exec(ctx, sql, opts)
		return execResultMsg{result: r, err: e}
	}
}

// loadMetadata returns a command that loads table metadata asynchronously.
func loadMetadata(metadata service.MetadataService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		t, e := metadata.Tables(ctx)
		return metadataMsg{tables: t, err: e}
	}
}

// resultWindowSize limits how many result rows are shown in one TUI page.
const resultWindowSize = 20

// renderResultWindow renders the current page of query results.
func (m Model) renderResultWindow() string {
	if len(m.columns) == 0 {
		return fmt.Sprintf("OK (%d rows)", len(m.rows))
	}
	end := m.rowStart + resultWindowSize
	if end > len(m.rows) {
		end = len(m.rows)
	}
	lines := []string{strings.Join(m.columns, "\t")}
	for _, r := range m.rows[m.rowStart:end] {
		vals := make([]string, len(r))
		for i, v := range r {
			vals[i] = fmt.Sprint(v)
		}
		lines = append(lines, strings.Join(vals, "\t"))
	}
	if len(m.rows) > resultWindowSize {
		lines = append(lines, fmt.Sprintf("[%d-%d/%d]", m.rowStart+1, end, len(m.rows)))
	}
	return strings.Join(lines, "\n")
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// startAddConnection switches the TUI into the inline connection form state.
func (m *Model) startAddConnection() {
	m.addingConnection = true
	m.connFocus = 0
	m.connInputs = make([]textinput.Model, 7)
	labels := []string{"name", "driver", "host", "port", "database", "user", "password"}
	defaults := []string{"", "postgres", "localhost", "5432", "", "", ""}
	for i := range m.connInputs {
		ti := textinput.New()
		ti.Prompt = labels[i] + ": "
		ti.Placeholder = defaults[i]
		ti.SetValue(defaults[i])
		if labels[i] == "password" {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '*'
		}
		if i == 0 {
			ti.Focus()
		} else {
			ti.Blur()
		}
		m.connInputs[i] = ti
	}
	m.status = "新規DB接続を追加中: Tabで項目移動、Enterで確定"
}

// updateAddConnection handles keyboard input while the inline connection form
// is active.
func (m Model) updateAddConnection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.addingConnection = false
		m.status = "DB追加をキャンセルしました"
		return m, nil
	case "tab", "shift+tab":
		m.connInputs[m.connFocus].Blur()
		if msg.String() == "tab" {
			m.connFocus = (m.connFocus + 1) % len(m.connInputs)
		} else {
			m.connFocus = (m.connFocus - 1 + len(m.connInputs)) % len(m.connInputs)
		}
		m.connInputs[m.connFocus].Focus()
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.connInputs[0].Value())
		driver := strings.TrimSpace(m.connInputs[1].Value())
		host := strings.TrimSpace(m.connInputs[2].Value())
		portText := strings.TrimSpace(m.connInputs[3].Value())
		dbName := strings.TrimSpace(m.connInputs[4].Value())
		user := strings.TrimSpace(m.connInputs[5].Value())
		password := m.connInputs[6].Value()
		if name == "" || driver == "" {
			m.status = "name/driver は必須です"
			return m, nil
		}
		port, _ := strconv.Atoi(portText)
		dsn, err := db.DSN(db.Connection{Driver: driver, Host: host, Port: port, Database: dbName, User: user, Password: password})
		if err != nil {
			m.status = "接続情報エラー: " + err.Error()
			return m, nil
		}
		entry := ConnectionEntry{Name: name, Driver: driver, DSN: dsn}
		m.connections = append(m.connections, entry)
		m.execOpts.Driver = driver
		m.execOpts.DSN = dsn
		m.metadata = service.NewConnectedMetadataService(driver, dsn)
		m.activeConnection = name
		m.addingConnection = false
		m.status = fmt.Sprintf("接続 '%s' を追加して接続しました", name)
		return m, loadMetadata(m.metadata)
	}
	var cmd tea.Cmd
	m.connInputs[m.connFocus], cmd = m.connInputs[m.connFocus].Update(msg)
	return m, cmd
}

// renderHelp returns the compact keybinding help line.
func (m Model) renderHelp() string {
	return "help: q/ctrl+c=quit, tab=focus, j/k=move, [/] = detail tab, enter=run sql, PgUp/PgDn=result scroll, a=新規DB追加, ?=help"
}

// renderAddConnection renders the inline connection form.
func (m Model) renderAddConnection() string {
	lines := []string{"[DB追加] Tab/Shift+Tab:項目移動 Enter:確定 Esc:キャンセル"}
	for i, in := range m.connInputs {
		p := "  "
		if i == m.connFocus {
			p = "> "
		}
		lines = append(lines, p+in.View())
	}
	return strings.Join(lines, "\n")
}

// currentConnectionLabel returns the label shown in the detail panel for the
// active connection source.
func (m Model) currentConnectionLabel() string {
	if m.activeConnection != "" {
		return m.activeConnection
	}
	if m.execOpts.Driver != "" || m.execOpts.DSN != "" {
		return fmt.Sprintf("%s (direct)", m.execOpts.Driver)
	}
	return "default(mock)"
}
