package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/isksss/sqio/internal/config"
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
		DDL: "CREATE TABLE users (id integer primary key);",
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
		DDL: "CREATE TABLE users (id integer primary key);",
	}}})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	if model.focus != 1 {
		t.Fatalf("expected detail focus, got %d", model.focus)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	model = updated.(Model)
	if !strings.Contains(model.renderDetail(), "ddl") || !strings.Contains(model.renderDetail(), "CREATE TABLE users") {
		t.Fatalf("unexpected ddl detail: %s", model.renderDetail())
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
