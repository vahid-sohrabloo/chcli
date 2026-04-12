package metacmd

import (
	"testing"
)

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
