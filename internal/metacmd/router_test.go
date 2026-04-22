package metacmd

import (
	"context"
	"testing"
)

func TestRouterDispatchesTop(t *testing.T) {
	r := &Router{handlers: map[string]Handler{}, hctx: &HandlerContext{}}
	r.handlers["top"] = handleTop

	res, err := r.Execute(context.Background(), `\top`)
	if err != nil {
		t.Fatalf("\\top returned error: %v", err)
	}
	if res == nil || !res.OpenMonitor {
		t.Errorf("\\top should set OpenMonitor; got %+v", res)
	}
	if res.ActiveTab != "processes" {
		t.Errorf("\\top ActiveTab = %q, want processes", res.ActiveTab)
	}
	if res.TopInterval != "" {
		t.Errorf("\\top without arg should leave TopInterval empty, got %q", res.TopInterval)
	}
}

func TestRouterDispatchesMonitor(t *testing.T) {
	r := &Router{handlers: map[string]Handler{}, hctx: &HandlerContext{}}
	r.handlers["monitor"] = handleMonitor

	res, err := r.Execute(context.Background(), `\monitor`)
	if err != nil {
		t.Fatalf("\\monitor returned error: %v", err)
	}
	if res == nil || !res.OpenMonitor {
		t.Errorf("\\monitor should set OpenMonitor; got %+v", res)
	}
	if res.ActiveTab != "" {
		t.Errorf("ActiveTab = %q, want empty for \\monitor", res.ActiveTab)
	}
}

func TestRouterDispatchesTopWithInterval(t *testing.T) {
	r := &Router{handlers: map[string]Handler{}, hctx: &HandlerContext{}}
	r.handlers["top"] = handleTop

	res, err := r.Execute(context.Background(), `\top 5s`)
	if err != nil {
		t.Fatalf("\\top 5s returned error: %v", err)
	}
	if res.TopInterval != "5s" {
		t.Errorf("TopInterval = %q, want 5s", res.TopInterval)
	}
}

func TestRouterDispatchesTopInvalidInterval(t *testing.T) {
	r := &Router{handlers: map[string]Handler{}, hctx: &HandlerContext{}}
	r.handlers["top"] = handleTop

	_, err := r.Execute(context.Background(), `\top bogus`)
	if err == nil {
		t.Fatal("\\top bogus should return an error")
	}
}

func TestIsMetaCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`\dt`, true},
		{`\l`, true},
		{"SELECT 1", false},
		{"  SELECT 1  ", false},
		{`  \dt  `, true},
	}
	for _, tc := range tests {
		got := IsMetaCommand(tc.input)
		if got != tc.want {
			t.Errorf("IsMetaCommand(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseMetaCommand(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{`\dt`, "dt", nil},
		{`\dt+`, "dt+", nil},
		{`\d users`, "d", []string{"users"}},
		{`\d+ users`, "d+", []string{"users"}},
		{`\f active_users`, "f", []string{"active_users"}},
		{`\fs mysnippet SELECT 1`, "fs", []string{"mysnippet", "SELECT 1"}},
		{`\h SELECT`, "h", []string{"SELECT"}},
		{`\hb daily_report`, "hb", []string{"daily_report"}},
	}
	for _, tc := range tests {
		gotCmd, gotArgs := parseMetaCommand(tc.input)
		if gotCmd != tc.wantCmd {
			t.Errorf("parseMetaCommand(%q) cmd = %q, want %q", tc.input, gotCmd, tc.wantCmd)
		}
		if len(gotArgs) != len(tc.wantArgs) {
			t.Errorf("parseMetaCommand(%q) args = %v, want %v", tc.input, gotArgs, tc.wantArgs)
			continue
		}
		for i := range gotArgs {
			if gotArgs[i] != tc.wantArgs[i] {
				t.Errorf("parseMetaCommand(%q) args[%d] = %q, want %q", tc.input, i, gotArgs[i], tc.wantArgs[i])
			}
		}
	}
}
