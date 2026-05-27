package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/isksss/sqio/internal/config"
	"github.com/isksss/sqio/internal/db"
	"github.com/isksss/sqio/internal/output"
	"github.com/isksss/sqio/internal/service"
)

// TestNavigation verifies the behavior covered by this test helper or case.
func TestNavigation(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(metadataMsg{tables: []service.Table{{Name: "users"}, {Name: "posts"}}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selected != 1 {
		t.Fatalf("expected selected 1, got %d", model.selected)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.selected != 0 {
		t.Fatalf("expected selected 0, got %d", model.selected)
	}
}

// TestFocusCycle verifies the behavior covered by this test helper or case.
func TestFocusCycle(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.focus != 1 {
		t.Fatalf("expected focus 1, got %d", model.focus)
	}
}

// TestExecResultMessage verifies the behavior covered by this test helper or case.
func TestExecResultMessage(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(execResultMsg{
		result: output.Result{
			Columns:  []string{"?column?"},
			Rows:     [][]interface{}{{1}},
			RowCount: 1,
		},
	})
	model = updated.(Model)
	if !strings.Contains(model.status, "done: 1 rows") {
		t.Fatalf("unexpected status: %s", model.status)
	}
	if !strings.Contains(model.result, "1") {
		t.Fatalf("unexpected result: %s", model.result)
	}
}

// TestResultPagination verifies the behavior covered by this test helper or case.
func TestResultPagination(t *testing.T) {
	model := New(config.Default(), true)
	rows := make([][]interface{}, 25)
	for i := range rows {
		rows[i] = []interface{}{i + 1}
	}
	updated, _ := model.Update(execResultMsg{
		result: output.Result{
			Columns:  []string{"id"},
			Rows:     rows,
			RowCount: len(rows),
		},
	})
	model = updated.(Model)
	if !strings.Contains(model.result, "[1-20/25]") {
		t.Fatalf("expected first page marker, got %s", model.result)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	model = updated.(Model)
	if !strings.Contains(model.result, "[21-25/25]") {
		t.Fatalf("expected second page marker, got %s", model.result)
	}
}

// TestMetadataMessage verifies the behavior covered by this test helper or case.
func TestMetadataMessage(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(metadataMsg{tables: []service.Table{{
		Name: "users",
		Columns: []service.Column{
			{Name: "id", Type: "integer", Primary: true},
		},
		Indexes: []service.Index{{Name: "users_pkey", Columns: []string{"id"}, Unique: true, Primary: true}},
		DDL:     "CREATE TABLE users (id integer primary key);",
	}}})
	model = updated.(Model)
	if len(model.objects) != 1 || model.objects[0] != "users" {
		t.Fatalf("unexpected objects: %#v", model.objects)
	}
	if model.status != "ready" {
		t.Fatalf("unexpected status: %s", model.status)
	}
	detail := model.renderDetail()
	if !strings.Contains(detail, "columns") || !strings.Contains(detail, "id\tinteger") {
		t.Fatalf("unexpected detail: %s", detail)
	}
}

// TestDetailTabs verifies the behavior covered by this test helper or case.
func TestDetailTabs(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(metadataMsg{tables: []service.Table{{
		Name: "users",
		Columns: []service.Column{
			{Name: "id", Type: "integer", Primary: true},
		},
		Indexes: []service.Index{{Name: "users_pkey", Columns: []string{"id"}, Unique: true, Primary: true}},
		DDL:     "CREATE TABLE users (id integer primary key);",
	}}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.focus != 1 {
		t.Fatalf("expected detail focus, got %d", model.focus)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	model = updated.(Model)
	if !strings.Contains(model.renderDetail(), "indexes") || !strings.Contains(model.renderDetail(), "users_pkey") {
		t.Fatalf("unexpected indexes detail: %s", model.renderDetail())
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	model = updated.(Model)
	if !strings.Contains(model.renderDetail(), "ddl") || !strings.Contains(model.renderDetail(), "CREATE TABLE users") {
		t.Fatalf("unexpected ddl detail: %s", model.renderDetail())
	}
}

func TestResultFilter(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(execResultMsg{
		result: output.Result{
			Columns: []string{"id", "name"},
			Rows: [][]interface{}{
				{1, "alice"},
				{2, "bob"},
				{3, "carol"},
			},
			RowCount: 3,
		},
	})
	model = updated.(Model)
	model.focus = 1
	model.detailTab = 3
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = updated.(Model)
	if !model.filteringResult {
		t.Fatal("expected result filter mode")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("bo")})
	model = updated.(Model)
	if !strings.Contains(model.result, "bob") || strings.Contains(model.result, "alice") {
		t.Fatalf("unexpected filtered result: %s", model.result)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.filteringResult || model.resultFilter != "bo" {
		t.Fatalf("expected applied filter, filtering=%v filter=%q", model.filteringResult, model.resultFilter)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.resultFilter != "" || !strings.Contains(model.result, "alice") {
		t.Fatalf("expected cleared filter, filter=%q result=%s", model.resultFilter, model.result)
	}
}

func TestResultSort(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(execResultMsg{
		result: output.Result{
			Columns: []string{"name"},
			Rows: [][]interface{}{
				{"carol"},
				{"alice"},
				{"bob"},
			},
			RowCount: 3,
		},
	})
	model = updated.(Model)
	model.focus = 1
	model.detailTab = 3
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = updated.(Model)
	if !strings.Contains(model.result, "sort: name asc") || strings.Index(model.result, "alice") > strings.Index(model.result, "bob") {
		t.Fatalf("unexpected ascending sort: %s", model.result)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = updated.(Model)
	if !strings.Contains(model.result, "sort: name desc") || strings.Index(model.result, "carol") > strings.Index(model.result, "bob") {
		t.Fatalf("unexpected descending sort: %s", model.result)
	}
}

func TestResultCursorAndRowHelpers(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(metadataMsg{tables: []service.Table{{Name: "users"}}})
	model = updated.(Model)
	updated, _ = model.Update(execResultMsg{
		result: output.Result{
			Columns: []string{"id", "name"},
			Rows: [][]interface{}{
				{1, "alice"},
				{2, "bob"},
			},
			RowCount: 2,
		},
	})
	model = updated.(Model)
	model.focus = 1
	model.detailTab = 3
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if row := model.selectedResultRow(); len(row) != 2 || row[1] != "bob" {
		t.Fatalf("unexpected selected row: %+v", row)
	}
	where, ok := model.selectedRowWhere()
	if !ok || where != `"id" = 2` {
		t.Fatalf("unexpected selected where: %s ok=%v", where, ok)
	}
}

func TestRowEditAndDeleteCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := db.Execute(context.Background(), db.Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key, name text); insert into users (id, name) values (1, 'alice'), (2, 'bob');`, db.ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	model := NewWithServices(config.Default(), service.NewMetadataService(), service.Executor{}, service.ExecOptions{Driver: "sqlite", DSN: path}, true)
	updated, _ := model.Update(metadataMsg{tables: []service.Table{{Name: "users"}}})
	model = updated.(Model)
	updated, _ = model.Update(execResultMsg{
		result: output.Result{
			Columns:  []string{"id", "name"},
			Rows:     [][]interface{}{{1, "alice"}, {2, "bob"}},
			RowCount: 2,
		},
	})
	model = updated.(Model)
	model.focus = 1
	model.detailTab = 3

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	model = updated.(Model)
	if !model.editingRow {
		t.Fatal("expected row edit mode")
	}
	model.editInput.SetValue("name=ann")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected row update command")
	}
	msg := cmd()
	updated, _ = model.Update(msg)
	model = updated.(Model)
	if !strings.Contains(model.status, "update: 1 rows") {
		t.Fatalf("unexpected update status: %s", model.status)
	}
	result, err := db.Execute(context.Background(), db.Config{Driver: "sqlite", DSN: path}, `select name from users where id = 1`, db.ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Rows[0][0] != "ann" {
		t.Fatalf("expected updated name, got %+v", result)
	}

	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected row delete command")
	}
	msg = cmd()
	updated, _ = model.Update(msg)
	model = updated.(Model)
	if !strings.Contains(model.status, "delete: 1 rows") || len(model.rows) != 1 {
		t.Fatalf("unexpected delete status=%s rows=%+v", model.status, model.rows)
	}
}

func TestCellEditCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	if _, err := db.Execute(context.Background(), db.Config{Driver: "sqlite", DSN: path}, `create table users (id integer primary key, name text); insert into users (id, name) values (1, 'alice');`, db.ExecuteOptions{}); err != nil {
		t.Fatal(err)
	}
	model := NewWithServices(config.Default(), service.NewMetadataService(), service.Executor{}, service.ExecOptions{Driver: "sqlite", DSN: path}, true)
	updated, _ := model.Update(metadataMsg{tables: []service.Table{{Name: "users"}}})
	model = updated.(Model)
	updated, _ = model.Update(execResultMsg{
		result: output.Result{
			Columns:  []string{"id", "name"},
			Rows:     [][]interface{}{{1, "alice"}},
			RowCount: 1,
		},
	})
	model = updated.(Model)
	model.focus = 1
	model.detailTab = 3
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model = updated.(Model)
	if model.cellCursor != 1 || !strings.Contains(model.result, "[alice]") {
		t.Fatalf("expected selected name cell, cursor=%d result=%s", model.cellCursor, model.result)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model = updated.(Model)
	if !model.editingCell || model.editColumn != "name" || model.editInput.Value() != "alice" {
		t.Fatalf("unexpected cell edit state: editing=%v column=%s value=%s", model.editingCell, model.editColumn, model.editInput.Value())
	}
	model.editInput.SetValue("ann")
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected cell update command")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if !strings.Contains(model.status, "update: 1 rows") || model.rows[0][1] != "ann" {
		t.Fatalf("unexpected cell update status=%s rows=%+v", model.status, model.rows)
	}
}

func TestCellCursorAndEditValidationBranches(t *testing.T) {
	model := New(config.Default(), true)
	model.focus = 1
	model.detailTab = 3
	model.columns = []string{"id", "name"}
	model.rows = [][]interface{}{{1, "alice"}}
	model.result = model.renderResultWindow()
	model.moveCellCursor(-10)
	if model.cellCursor != 0 {
		t.Fatalf("expected lower-bound cell cursor, got %d", model.cellCursor)
	}
	model.moveCellCursor(10)
	if model.cellCursor != 1 {
		t.Fatalf("expected upper-bound cell cursor, got %d", model.cellCursor)
	}
	model.startCellEdit()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.editingCell || !strings.Contains(model.status, "canceled") {
		t.Fatalf("expected canceled cell edit, status=%s editing=%v", model.status, model.editingCell)
	}
	model.tables = nil
	model.startCellEdit()
	model.editInput.SetValue("bob")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !strings.Contains(model.status, "selected table") {
		t.Fatalf("expected selected table error, got %s", model.status)
	}
	model.tables = []service.Table{{Name: "users"}}
	model.columns = nil
	model.startCellEdit()
	model.editInput.SetValue("bob")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !strings.Contains(model.status, "key column") {
		t.Fatalf("expected key column error, got %s", model.status)
	}
	updated, _ = model.Update(editResultMsg{action: "update", err: assertErr("boom")})
	model = updated.(Model)
	if !strings.Contains(model.status, "edit error: boom") {
		t.Fatalf("unexpected edit error status: %s", model.status)
	}
}

func TestRowEditValidationHelpers(t *testing.T) {
	values, err := parseAssignments("name=alice,status=active")
	if err != nil {
		t.Fatal(err)
	}
	if values["name"] != "alice" || values["status"] != "active" {
		t.Fatalf("unexpected assignments: %+v", values)
	}
	if _, err := parseAssignments("bad"); err == nil {
		t.Fatal("expected bad assignment error")
	}
	if got := sqlLiteral("a'b"); got != "'a''b'" {
		t.Fatalf("unexpected string literal: %s", got)
	}
	if got := quoteIdentForSQL(`weird"name`); got != `"weird""name"` {
		t.Fatalf("unexpected quoted identifier: %s", got)
	}
	if got := sqlLiteral(nil); got != "NULL" {
		t.Fatalf("unexpected nil literal: %s", got)
	}
	if sameRow([]interface{}{1}, []interface{}{1, 2}) {
		t.Fatal("expected different length rows not to match")
	}
	if sameRow([]interface{}{1}, []interface{}{2}) {
		t.Fatal("expected different rows not to match")
	}
}

func TestRowEditValidationBranches(t *testing.T) {
	model := New(config.Default(), true)
	model.focus = 1
	model.detailTab = 3
	model.columns = []string{"id", "name"}
	model.rows = [][]interface{}{{1, "alice"}}
	model.result = model.renderResultWindow()
	model.startRowEdit()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.editingRow || !strings.Contains(model.status, "canceled") {
		t.Fatalf("expected canceled edit, status=%s editing=%v", model.status, model.editingRow)
	}

	model.startRowEdit()
	model.editInput.SetValue("bad")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !strings.Contains(model.status, "assignments must use") {
		t.Fatalf("expected assignment error, got %s", model.status)
	}

	model.tables = nil
	model.editInput.SetValue("name=bob")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !strings.Contains(model.status, "selected table") {
		t.Fatalf("expected selected table error, got %s", model.status)
	}

	model.columns = nil
	model.tables = []service.Table{{Name: "users"}}
	model.editInput.SetValue("name=bob")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if !strings.Contains(model.status, "key column") {
		t.Fatalf("expected key column error, got %s", model.status)
	}
}

func TestResultCursorBoundsAndDeleteError(t *testing.T) {
	model := New(config.Default(), true)
	model.focus = 1
	model.detailTab = 3
	model.tables = []service.Table{{Name: "users"}}
	model.columns = []string{"id", "name"}
	model.rows = [][]interface{}{{1, "alice"}}
	model.moveResultCursor(-10)
	if model.rowCursor != 0 {
		t.Fatalf("expected lower-bound cursor, got %d", model.rowCursor)
	}
	model.moveResultCursor(10)
	if model.rowCursor != 0 {
		t.Fatalf("expected upper-bound cursor, got %d", model.rowCursor)
	}
	updated, _ := model.Update(editResultMsg{action: "delete", err: assertErr("boom")})
	model = updated.(Model)
	if !strings.Contains(model.status, "edit error: boom") {
		t.Fatalf("unexpected edit error status: %s", model.status)
	}
}

func TestSQLCompletion(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(metadataMsg{tables: []service.Table{{
		Name: "users",
		Columns: []service.Column{
			{Name: "name", Type: "text"},
		},
	}}})
	model = updated.(Model)
	model.focus = 2
	model.input.SetValue("sel")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	model = updated.(Model)
	if model.input.Value() != "SELECT" || !strings.Contains(model.status, "completion") {
		t.Fatalf("unexpected keyword completion value=%q status=%s", model.input.Value(), model.status)
	}
	model.input.SetValue("select na")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	model = updated.(Model)
	if model.input.Value() != "select name" {
		t.Fatalf("unexpected column completion: %q", model.input.Value())
	}
	model.input.SetValue("zz")
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	model = updated.(Model)
	if !strings.Contains(model.status, "no candidates") {
		t.Fatalf("expected no candidates status, got %s", model.status)
	}
}

// TestToggleHelp verifies the behavior covered by this test helper or case.
func TestToggleHelp(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model = updated.(Model)
	if !model.showHelp {
		t.Fatal("expected help to be visible")
	}
}

// TestAddConnectionFlow verifies the behavior covered by this test helper or case.
func TestAddConnectionFlow(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model = updated.(Model)
	if !model.addingConnection {
		t.Fatal("expected add-connection mode")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("local")})
	model = updated.(Model)
	for i := 0; i < 6; i++ {
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
		model = updated.(Model)
	}
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected metadata reload command")
	}
	if model.execOpts.Driver != "postgres" || !strings.Contains(model.execOpts.DSN, "postgres") {
		t.Fatalf("unexpected exec opts: %#v", model.execOpts)
	}
	if model.activeConnection != "local" {
		t.Fatalf("unexpected active connection: %s", model.activeConnection)
	}
}

// TestAddConnectionPasswordInputIsMasked verifies the inline connection form
// does not render database passwords as plain text.
func TestAddConnectionPasswordInputIsMasked(t *testing.T) {
	model := New(config.Default(), true)
	model.startAddConnection()
	passwordInput := model.connInputs[6]
	if passwordInput.EchoMode != textinput.EchoPassword || passwordInput.EchoCharacter != '*' {
		t.Fatalf("expected masked password input, got mode=%v char=%q", passwordInput.EchoMode, passwordInput.EchoCharacter)
	}
}

// TestViewIncludesPanels verifies the behavior covered by this test helper or case.
func TestViewIncludesPanels(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model = updated.(Model)
	view := model.View()
	for _, want := range []string{"object tree", "detail", "sql console"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in view", want)
		}
	}
}

func TestInitRunSQLAndLoadMetadata(t *testing.T) {
	model := New(config.Default(), true)
	if cmd := model.Init(); cmd == nil {
		t.Fatal("expected init command")
	}
	msg := runSQL(service.Executor{}, service.ExecOptions{}, "select 1")()
	result, ok := msg.(execResultMsg)
	if !ok || result.err != nil || result.result.RowCount != 1 {
		t.Fatalf("unexpected run sql msg: %#v", msg)
	}
	meta := loadMetadata(service.NewMetadataService())()
	metadata, ok := meta.(metadataMsg)
	if !ok || metadata.err != nil || len(metadata.tables) == 0 {
		t.Fatalf("unexpected metadata msg: %#v", meta)
	}
}

func TestViewAndConnectionRenderingBranches(t *testing.T) {
	model := New(config.Default(), true)
	if got := model.View(); got != "sqio\n" {
		t.Fatalf("unexpected zero-size view: %q", got)
	}
	model.width = 80
	model.height = 20
	model.showHelp = true
	model.startAddConnection()
	view := model.View()
	for _, want := range []string{"help:", "[DB追加]", "password"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in view: %s", want, view)
		}
	}
	if got := model.renderAddConnection(); !strings.Contains(got, "[DB追加]") {
		t.Fatalf("unexpected add connection view: %s", got)
	}
	model.activeConnection = "local"
	if got := model.currentConnectionLabel(); got != "local" {
		t.Fatalf("unexpected active label: %s", got)
	}
	model.activeConnection = ""
	model.execOpts.Driver = "sqlite"
	if got := model.currentConnectionLabel(); got != "sqlite (direct)" {
		t.Fatalf("unexpected direct label: %s", got)
	}
}

func TestUpdateAdditionalKeysAndErrors(t *testing.T) {
	model := New(config.Default(), true)
	updated, _ := model.Update(metadataMsg{err: assertErr("boom")})
	model = updated.(Model)
	if !strings.Contains(model.status, "metadata error") {
		t.Fatalf("unexpected metadata error status: %s", model.status)
	}
	updated, _ = model.Update(execResultMsg{err: assertErr("boom")})
	model = updated.(Model)
	if !strings.Contains(model.status, "error: boom") {
		t.Fatalf("unexpected exec error status: %s", model.status)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.addingConnection {
		t.Fatal("expected add connection mode to close")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.focus != 1 {
		t.Fatalf("unexpected focus: %d", model.focus)
	}
}

func TestUpdateKeyBranches(t *testing.T) {
	model := New(config.Default(), true)
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	model.focus = 1
	model.detailTab = 0
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	model = updated.(Model)
	if model.detailTab != len(detailTabs)-1 {
		t.Fatalf("expected wrapped detail tab, got %d", model.detailTab)
	}
	model.detailTab = 3
	model.columns = nil
	model.rows = [][]interface{}{{"alice"}}
	model.result = model.renderResultWindow()
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = updated.(Model)
	if cmd != nil || !strings.Contains(model.status, "key column") {
		t.Fatalf("expected key column delete error, status=%s cmd=%v", model.status, cmd)
	}
	model.columns = []string{"id", "name"}
	model.rows = [][]interface{}{{1, "alice"}}
	model.tables = nil
	model.result = model.renderResultWindow()
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = updated.(Model)
	if cmd != nil || !strings.Contains(model.status, "selected table") {
		t.Fatalf("expected selected table delete error, status=%s cmd=%v", model.status, cmd)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	model = updated.(Model)
	if model.rowStart != 0 {
		t.Fatalf("expected pgup lower bound, got %d", model.rowStart)
	}
	model.focus = 2
	model.input.SetValue("")
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if cmd == nil || !strings.Contains(model.status, "running: select 1") {
		t.Fatalf("expected default SQL run, status=%s cmd=%v", model.status, cmd)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
