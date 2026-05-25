package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/isksss/sqio/internal/config"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/service"
)

type Model struct {
	cfg       config.Config
	metadata  service.MetadataService
	executor  service.Executor
	execOpts  service.ExecOptions
	width     int
	height    int
	focus     int
	input     textinput.Model
	status    string
	result    string
	rows      [][]interface{}
	columns   []string
	rowStart  int
	detailTab int
	noColor   bool
	objects   []string
	tables    []service.Table
	selected  int
}

type metadataMsg struct {
	tables []service.Table
	err    error
}

type execResultMsg struct {
	result output.Result
	err    error
}

func New(cfg config.Config, noColor bool) Model {
	return NewWithMetadata(cfg, service.NewMetadataService(), noColor)
}

func NewWithMetadata(cfg config.Config, metadata service.MetadataService, noColor bool) Model {
	return NewWithServices(cfg, metadata, service.Executor{}, service.ExecOptions{Format: cfg.Query.Format, MaxRows: cfg.Query.MaxRows}, noColor)
}

func NewWithServices(cfg config.Config, metadata service.MetadataService, executor service.Executor, execOpts service.ExecOptions, noColor bool) Model {
	input := textinput.New()
	input.Placeholder = "select 1"
	input.Prompt = "sql> "
	input.Focus()
	return Model{
		cfg:      cfg,
		metadata: metadata,
		executor: executor,
		execOpts: execOpts,
		input:    input,
		status:   "loading metadata",
		noColor:  noColor,
		objects:  []string{"loading"},
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, loadMetadata(m.metadata))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
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
		for _, table := range msg.tables {
			m.objects = append(m.objects, table.Name)
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
	return lipgloss.JoinHorizontal(lipgloss.Top, tree, detail) + "\n" + console
}

func (m Model) renderTree() string {
	lines := make([]string, len(m.objects))
	for i, object := range m.objects {
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}
		lines[i] = prefix + object
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderDetail() string {
	if len(m.objects) == 0 {
		return "no object"
	}
	body := fmt.Sprintf("selected: %s\n%s", m.objects[m.selected], m.renderDetailTabs())
	if m.selected < len(m.tables) {
		table := m.tables[m.selected]
		switch detailTabs[m.detailTab] {
		case "columns":
			lines := []string{"", "columns"}
			for _, column := range table.Columns {
				flags := ""
				if column.Primary {
					flags += " pk"
				}
				if !column.Nullable {
					flags += " not-null"
				}
				lines = append(lines, fmt.Sprintf("%s\t%s%s", column.Name, column.Type, flags))
			}
			body += "\n" + strings.Join(lines, "\n")
		case "ddl":
			if table.DDL != "" {
				body += "\n\nddl\n" + table.DDL
			} else {
				body += "\n\nddl\n"
			}
		case "result":
			if strings.TrimSpace(m.result) != "" {
				body += "\n\nresult\n" + m.result
			} else {
				body += "\n\nresult\n"
			}
		}
	}
	return body
}

var detailTabs = []string{"columns", "ddl", "result"}

func (m Model) renderDetailTabs() string {
	parts := make([]string, len(detailTabs))
	for i, tab := range detailTabs {
		if i == m.detailTab {
			parts[i] = "[" + tab + "]"
			continue
		}
		parts[i] = " " + tab + " "
	}
	return strings.Join(parts, " ")
}

func (m Model) panel(title, body string, width, height int, focused bool) string {
	border := lipgloss.NormalBorder()
	if focused {
		border = lipgloss.ThickBorder()
	}
	style := lipgloss.NewStyle().Width(width-2).Height(height-2).Border(border).Padding(0, 1)
	if !m.noColor && focused {
		style = style.BorderForeground(lipgloss.Color("39"))
	}
	content := title + "\n" + body
	return style.Render(content)
}

func runSQL(executor service.Executor, opts service.ExecOptions, sql string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := executor.Exec(ctx, sql, opts)
		return execResultMsg{result: result, err: err}
	}
}

func loadMetadata(metadata service.MetadataService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tables, err := metadata.Tables(ctx)
		return metadataMsg{tables: tables, err: err}
	}
}

const resultWindowSize = 20

func (m Model) renderResultWindow() string {
	if len(m.columns) == 0 {
		return fmt.Sprintf("OK (%d rows)", len(m.rows))
	}
	end := m.rowStart + resultWindowSize
	if end > len(m.rows) {
		end = len(m.rows)
	}
	lines := []string{strings.Join(m.columns, "\t")}
	for _, row := range m.rows[m.rowStart:end] {
		values := make([]string, len(row))
		for i, value := range row {
			values[i] = fmt.Sprint(value)
		}
		lines = append(lines, strings.Join(values, "\t"))
	}
	if len(m.rows) > resultWindowSize {
		lines = append(lines, fmt.Sprintf("[%d-%d/%d]", m.rowStart+1, end, len(m.rows)))
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
