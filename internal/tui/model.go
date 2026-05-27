// Package tui implements the Bubble Tea terminal user interface.
package tui

import (
	"context"
	"fmt"
	"sort"
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
	dataEditor       service.DataEditService
	execOpts         service.ExecOptions
	width            int
	height           int
	focus            int
	input            textinput.Model
	filterInput      textinput.Model
	editInput        textinput.Model
	status           string
	result           string
	rows             [][]interface{}
	columns          []string
	rowStart         int
	rowCursor        int
	cellCursor       int
	resultFilter     string
	filteringResult  bool
	editingRow       bool
	editingCell      bool
	editColumn       string
	resultSort       bool
	resultSortDesc   bool
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

var sqlCompletionKeywords = []string{
	"SELECT", "FROM", "WHERE", "JOIN", "LEFT JOIN", "INNER JOIN", "GROUP BY",
	"ORDER BY", "LIMIT", "INSERT", "UPDATE", "DELETE", "CREATE TABLE",
	"ALTER TABLE", "DROP TABLE", "EXPLAIN",
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

type editResultMsg struct {
	action   string
	affected int
	values   map[string]string
	err      error
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
	filterInput := textinput.New()
	filterInput.Placeholder = "filter"
	filterInput.Prompt = "filter> "
	editInput := textinput.New()
	editInput.Placeholder = "column=value"
	editInput.Prompt = "edit> "
	connections := make([]ConnectionEntry, 0, len(cfg.Connections))
	for _, conn := range cfg.Connections {
		connections = append(connections, ConnectionEntry{Name: conn.Name, Driver: conn.Driver, DSN: conn.DSN})
	}
	return Model{cfg: cfg, metadata: metadata, executor: executor, dataEditor: service.DataEditService{Driver: execOpts.Driver, DSN: execOpts.DSN}, execOpts: execOpts, input: input, filterInput: filterInput, editInput: editInput, status: "loading metadata", noColor: noColor, objects: []string{"loading"}, connections: connections}
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
		if m.filteringResult {
			return m.updateResultFilter(msg)
		}
		if m.editingRow {
			return m.updateRowEdit(msg)
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
		case "/":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" {
				m.startResultFilter()
			}
		case "s":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" && len(m.rows) > 0 {
				if m.resultSort {
					m.resultSortDesc = !m.resultSortDesc
				} else {
					m.resultSort = true
					m.resultSortDesc = false
				}
				m.rowStart = 0
				m.result = m.renderResultWindow()
			}
		case "e":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" && len(m.selectedResultRow()) > 0 {
				m.startRowEdit()
			}
		case "c":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" && len(m.selectedResultRow()) > 0 {
				m.startCellEdit()
			}
		case "x":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" && len(m.selectedResultRow()) > 0 {
				where, ok := m.selectedRowWhere()
				if !ok {
					m.status = "row delete requires a key column"
					return m, nil
				}
				table := m.selectedTableName()
				if table == "" {
					m.status = "row delete requires selected table"
					return m, nil
				}
				m.status = "deleting selected row"
				return m, deleteRow(m.dataEditor, table, where)
			}
		case "j", "down":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" {
				m.moveResultCursor(1)
			} else if m.focus == 0 && m.selected < len(m.objects)-1 {
				m.selected++
			}
		case "k", "up":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" {
				m.moveResultCursor(-1)
			} else if m.focus == 0 && m.selected > 0 {
				m.selected--
			}
		case "h", "left":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" {
				m.moveCellCursor(-1)
			}
		case "l", "right":
			if m.focus == 1 && detailTabs[m.detailTab] == "result" {
				m.moveCellCursor(1)
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
		case "ctrl+n":
			if m.focus == 2 {
				m.applySQLCompletion()
			}
		case "pgdown":
			if len(m.filteredRows()) > 0 {
				m.rowStart += resultWindowSize
				rows := m.filteredRows()
				if m.rowStart >= len(rows) {
					m.rowStart = max(0, len(rows)-resultWindowSize)
				}
				m.result = m.renderResultWindow()
			}
		case "pgup":
			if len(m.filteredRows()) > 0 {
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
		m.rowCursor = 0
		m.resultFilter = ""
		m.resultSort = false
		m.resultSortDesc = false
		m.filterInput.SetValue("")
		m.result = m.renderResultWindow()
	case editResultMsg:
		if msg.err != nil {
			m.status = "edit error: " + msg.err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("%s: %d rows", msg.action, msg.affected)
		if msg.action == "update" {
			m.updateSelectedResultRow(msg.values)
			m.result = m.renderResultWindow()
		}
		if msg.action == "delete" {
			m.removeSelectedResultRow()
			m.result = m.renderResultWindow()
		}
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

func (m *Model) applySQLCompletion() {
	current := m.input.Value()
	token := completionToken(current)
	candidates := m.sqlCompletionCandidates(token)
	if len(candidates) == 0 {
		m.status = "completion: no candidates"
		return
	}
	completed := replaceCompletionToken(current, candidates[0])
	m.input.SetValue(completed)
	m.input.CursorEnd()
	m.status = "completion: " + candidates[0]
}

func (m Model) sqlCompletionCandidates(token string) []string {
	tokenLower := strings.ToLower(token)
	seen := map[string]bool{}
	candidates := []string{}
	add := func(value string) {
		if value == "" || seen[value] {
			return
		}
		if tokenLower != "" && !strings.HasPrefix(strings.ToLower(value), tokenLower) {
			return
		}
		seen[value] = true
		candidates = append(candidates, value)
	}
	for _, keyword := range sqlCompletionKeywords {
		add(keyword)
	}
	for _, table := range m.tables {
		add(table.Name)
		for _, column := range table.Columns {
			add(column.Name)
		}
	}
	sort.Strings(candidates)
	return candidates
}

func completionToken(input string) string {
	fields := strings.FieldsFunc(input, completionDelimiter)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func replaceCompletionToken(input, completion string) string {
	idx := strings.LastIndexFunc(input, completionDelimiter)
	if idx < 0 {
		return completion
	}
	return input[:idx+1] + completion
}

func completionDelimiter(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ',' || r == '(' || r == ')'
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
	consoleBody := m.input.View() + "\n" + m.status
	if m.filteringResult {
		consoleBody = m.filterInput.View() + "\n" + m.status
	} else if m.editingRow {
		consoleBody = m.editInput.View() + "\n" + m.status
	}
	console := m.panel("sql console", consoleBody, m.width, 4, m.focus == 2 || m.filteringResult)
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
		case "indexes":
			lines := []string{"", "indexes"}
			for _, idx := range t.Indexes {
				flags := "nonunique"
				if idx.Unique {
					flags = "unique"
				}
				if idx.Primary {
					flags += " primary"
				}
				lines = append(lines, fmt.Sprintf("%s\t%s\t%s", idx.Name, strings.Join(idx.Columns, ","), flags))
			}
			body += "\n" + strings.Join(lines, "\n")
		case "result":
			body += "\n\nresult\n" + m.result
		}
	}
	return body
}

// detailTabs defines the tab order in the detail panel.
var detailTabs = []string{"columns", "indexes", "ddl", "result"}

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

func updateRow(editor service.DataEditService, table string, values map[string]string, where string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		affected, err := editor.Update(ctx, table, values, where)
		return editResultMsg{action: "update", affected: affected, values: values, err: err}
	}
}

func deleteRow(editor service.DataEditService, table, where string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		affected, err := editor.Delete(ctx, table, where)
		return editResultMsg{action: "delete", affected: affected, err: err}
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
	rows := m.filteredRows()
	start := m.rowStart
	if start > len(rows) {
		start = len(rows)
	}
	end := m.rowStart + resultWindowSize
	if end > len(rows) {
		end = len(rows)
	}
	lines := []string{strings.Join(m.columns, "\t")}
	for rowOffset, r := range rows[start:end] {
		vals := make([]string, len(r))
		for i, v := range r {
			vals[i] = fmt.Sprint(v)
			if start+rowOffset == m.rowStart+m.rowCursor && i == m.cellCursor {
				vals[i] = "[" + vals[i] + "]"
			}
		}
		prefix := "  "
		if start+rowOffset == m.rowStart+m.rowCursor {
			prefix = "> "
		}
		lines = append(lines, prefix+strings.Join(vals, "\t"))
	}
	if len(rows) == 0 && m.resultFilter != "" {
		lines = append(lines, "no matching rows")
	}
	if len(rows) > resultWindowSize {
		lines = append(lines, fmt.Sprintf("[%d-%d/%d]", start+1, end, len(rows)))
	}
	if m.resultFilter != "" {
		lines = append(lines, fmt.Sprintf("filter: %s (%d/%d rows)", m.resultFilter, len(rows), len(m.rows)))
	}
	if m.resultSort && len(m.columns) > 0 {
		direction := "asc"
		if m.resultSortDesc {
			direction = "desc"
		}
		lines = append(lines, fmt.Sprintf("sort: %s %s", m.columns[0], direction))
	}
	return strings.Join(lines, "\n")
}

func (m Model) filteredRows() [][]interface{} {
	rows := m.rows
	if m.resultFilter != "" {
		filter := strings.ToLower(m.resultFilter)
		rows = make([][]interface{}, 0, len(m.rows))
		for _, row := range m.rows {
			for _, value := range row {
				if strings.Contains(strings.ToLower(fmt.Sprint(value)), filter) {
					rows = append(rows, row)
					break
				}
			}
		}
	}
	if m.resultSort {
		rows = append([][]interface{}(nil), rows...)
		sort.SliceStable(rows, func(i, j int) bool {
			left := ""
			right := ""
			if len(rows[i]) > 0 {
				left = fmt.Sprint(rows[i][0])
			}
			if len(rows[j]) > 0 {
				right = fmt.Sprint(rows[j][0])
			}
			if m.resultSortDesc {
				return left > right
			}
			return left < right
		})
	}
	return rows
}

func (m *Model) moveResultCursor(delta int) {
	rows := m.filteredRows()
	if len(rows) == 0 {
		m.rowCursor = 0
		return
	}
	absolute := m.rowStart + m.rowCursor + delta
	if absolute < 0 {
		absolute = 0
	}
	if absolute >= len(rows) {
		absolute = len(rows) - 1
	}
	if absolute < m.rowStart {
		m.rowStart = absolute
	}
	if absolute >= m.rowStart+resultWindowSize {
		m.rowStart = absolute - resultWindowSize + 1
	}
	m.rowCursor = absolute - m.rowStart
	m.result = m.renderResultWindow()
}

func (m *Model) moveCellCursor(delta int) {
	if len(m.columns) == 0 {
		m.cellCursor = 0
		return
	}
	m.cellCursor += delta
	if m.cellCursor < 0 {
		m.cellCursor = 0
	}
	if m.cellCursor >= len(m.columns) {
		m.cellCursor = len(m.columns) - 1
	}
	m.result = m.renderResultWindow()
}

func (m Model) selectedResultRow() []interface{} {
	rows := m.filteredRows()
	idx := m.rowStart + m.rowCursor
	if idx < 0 || idx >= len(rows) {
		return nil
	}
	return rows[idx]
}

func (m Model) selectedTableName() string {
	if m.selected >= 0 && m.selected < len(m.tables) {
		return m.tables[m.selected].Name
	}
	return ""
}

func (m Model) selectedRowWhere() (string, bool) {
	row := m.selectedResultRow()
	if len(m.columns) == 0 || len(row) == 0 {
		return "", false
	}
	return quoteIdentForSQL(m.columns[0]) + " = " + sqlLiteral(row[0]), true
}

func (m *Model) startRowEdit() {
	row := m.selectedResultRow()
	assignments := make([]string, 0, len(m.columns))
	for i, col := range m.columns {
		if i < len(row) {
			assignments = append(assignments, col+"="+fmt.Sprint(row[i]))
		}
	}
	m.editingRow = true
	m.editInput.Focus()
	m.editInput.SetValue(strings.Join(assignments, ","))
	m.editInput.CursorEnd()
	m.status = "editing row: column=value,column=value Enterで保存 Escでキャンセル"
}

func (m *Model) startCellEdit() {
	row := m.selectedResultRow()
	if len(row) == 0 || len(m.columns) == 0 {
		m.status = "cell edit requires result rows"
		return
	}
	if m.cellCursor >= len(m.columns) {
		m.cellCursor = len(m.columns) - 1
	}
	value := ""
	if m.cellCursor < len(row) {
		value = fmt.Sprint(row[m.cellCursor])
	}
	m.editingRow = true
	m.editingCell = true
	m.editColumn = m.columns[m.cellCursor]
	m.editInput.Focus()
	m.editInput.SetValue(value)
	m.editInput.CursorEnd()
	m.status = "editing cell: Enterで保存 Escでキャンセル"
}

func (m Model) updateRowEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editingRow = false
		m.editingCell = false
		m.editColumn = ""
		m.editInput.SetValue("")
		m.status = "row edit canceled"
		return m, nil
	case "enter":
		values := map[string]string{}
		if m.editingCell {
			if m.editColumn == "" {
				m.status = "cell edit requires selected column"
				return m, nil
			}
			values[m.editColumn] = m.editInput.Value()
		} else {
			var err error
			values, err = parseAssignments(m.editInput.Value())
			if err != nil {
				m.status = "row edit error: " + err.Error()
				return m, nil
			}
		}
		where, ok := m.selectedRowWhere()
		if !ok {
			m.status = "edit requires a key column"
			return m, nil
		}
		table := m.selectedTableName()
		if table == "" {
			m.status = "edit requires selected table"
			return m, nil
		}
		m.editingRow = false
		m.editingCell = false
		m.editColumn = ""
		m.status = "updating selected row"
		return m, updateRow(m.dataEditor, table, values, where)
	}
	var cmd tea.Cmd
	m.editInput, cmd = m.editInput.Update(msg)
	return m, cmd
}

func parseAssignments(input string) (map[string]string, error) {
	values := map[string]string{}
	for _, assignment := range strings.Split(input, ",") {
		assignment = strings.TrimSpace(assignment)
		if assignment == "" {
			continue
		}
		column, value, ok := strings.Cut(assignment, "=")
		column = strings.TrimSpace(column)
		if !ok || column == "" {
			return nil, fmt.Errorf("assignments must use column=value")
		}
		values[column] = value
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one assignment is required")
	}
	return values, nil
}

func quoteIdentForSQL(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func sqlLiteral(value interface{}) string {
	if value == nil {
		return "NULL"
	}
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(value)
	default:
		return "'" + strings.ReplaceAll(fmt.Sprint(value), "'", "''") + "'"
	}
}

func (m *Model) removeSelectedResultRow() {
	selected := m.selectedResultRow()
	if selected == nil {
		return
	}
	for i, row := range m.rows {
		if sameRow(row, selected) {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			break
		}
	}
	if m.rowStart+m.rowCursor >= len(m.filteredRows()) && m.rowCursor > 0 {
		m.rowCursor--
	}
}

func (m *Model) updateSelectedResultRow(values map[string]string) {
	if len(values) == 0 {
		return
	}
	selected := m.selectedResultRow()
	if selected == nil {
		return
	}
	for rowIndex, row := range m.rows {
		if !sameRow(row, selected) {
			continue
		}
		for column, value := range values {
			for columnIndex, name := range m.columns {
				if name == column && columnIndex < len(m.rows[rowIndex]) {
					m.rows[rowIndex][columnIndex] = value
				}
			}
		}
		return
	}
}

func sameRow(left, right []interface{}) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if fmt.Sprint(left[i]) != fmt.Sprint(right[i]) {
			return false
		}
	}
	return true
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Model) startResultFilter() {
	m.filteringResult = true
	m.filterInput.Focus()
	m.filterInput.SetValue(m.resultFilter)
	m.status = "filtering result: Enterで確定、Escで解除"
}

func (m Model) updateResultFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filteringResult = false
		m.resultFilter = ""
		m.filterInput.SetValue("")
		m.rowStart = 0
		m.result = m.renderResultWindow()
		m.status = "result filter cleared"
		return m, nil
	case "enter":
		m.filteringResult = false
		m.resultFilter = strings.TrimSpace(m.filterInput.Value())
		m.rowStart = 0
		m.result = m.renderResultWindow()
		m.status = "result filter applied"
		return m, nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.resultFilter = strings.TrimSpace(m.filterInput.Value())
	m.rowStart = 0
	m.result = m.renderResultWindow()
	return m, cmd
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
		m.dataEditor = service.DataEditService{Driver: driver, DSN: dsn}
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
	return "help: q/ctrl+c=quit, tab=focus, j/k=move row, h/l=move cell, [/] = detail tab, /=filter result, s=sort result, e=edit row, c=edit cell, x=delete row, ctrl+n=complete, enter=run sql, PgUp/PgDn=result scroll, a=新規DB追加, ?=help"
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
