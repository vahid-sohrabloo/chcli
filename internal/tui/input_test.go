package tui

import (
	"strings"
	"testing"
)

func TestInputModel_InsertText(t *testing.T) {
	m := NewInputModel("chcli> ", nil)
	m.InsertText("SELECT 1;")
	if got := m.Value(); got != "SELECT 1;" {
		t.Errorf("Value() = %q, want %q", got, "SELECT 1;")
	}
}

func TestInputModel_Clear(t *testing.T) {
	m := NewInputModel("chcli> ", nil)
	m.InsertText("SELECT 1;")
	m.Clear()
	if got := m.Value(); got != "" {
		t.Errorf("after Clear(), Value() = %q, want empty string", got)
	}
}

func TestInputModel_ViewContainsPrompt(t *testing.T) {
	m := NewInputModel("chcli> ", nil)
	m.InsertText("SELECT 1;")
	view := m.View()
	if !strings.Contains(view, "chcli>") {
		t.Errorf("View() does not contain prompt %q; got:\n%s", "chcli>", view)
	}
}
