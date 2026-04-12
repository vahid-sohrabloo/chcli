package tui

import (
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/completer"
)

func TestCompletionModel_SetItems(t *testing.T) {
	m := NewCompletionModel()
	items := []completer.Completion{
		{Text: "foo", Kind: completer.KindTable},
		{Text: "bar", Kind: completer.KindColumn},
	}
	m.SetItems(items)
	if m.Len() != 2 {
		t.Fatalf("expected Len() == 2, got %d", m.Len())
	}
}

func TestCompletionModel_Navigation(t *testing.T) {
	m := NewCompletionModel()
	items := []completer.Completion{
		{Text: "first", Kind: completer.KindKeyword},
		{Text: "second", Kind: completer.KindKeyword},
		{Text: "third", Kind: completer.KindKeyword},
	}
	m.SetItems(items)
	m.Show()

	if got := m.Selected(); got != "first" {
		t.Fatalf("expected Selected() == %q, got %q", "first", got)
	}

	m.Next()
	if got := m.Selected(); got != "second" {
		t.Fatalf("after Next(): expected Selected() == %q, got %q", "second", got)
	}

	m.Prev()
	if got := m.Selected(); got != "first" {
		t.Fatalf("after Prev(): expected Selected() == %q, got %q", "first", got)
	}
}

func TestCompletionModel_Visibility(t *testing.T) {
	m := NewCompletionModel()

	if m.Visible() {
		t.Fatal("expected not visible by default")
	}

	items := []completer.Completion{
		{Text: "alpha", Kind: completer.KindTable},
	}
	m.SetItems(items)
	m.Show()

	if !m.Visible() {
		t.Fatal("expected visible after Show()")
	}

	m.Hide()
	if m.Visible() {
		t.Fatal("expected not visible after Hide()")
	}
}
