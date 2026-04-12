package format

import (
	"testing"
)

func TestFormatSQL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "simple select",
			input: "select 1",
			want:  "SELECT 1",
		},
		{
			name:  "with from",
			input: "select * from users",
			want:  "SELECT *\n    FROM users",
		},
		{
			name:  "with where",
			input: "select id from users where id = 1",
			want:  "SELECT id\n    FROM users\n    WHERE id = 1",
		},
		{
			name:  "multi-clause",
			input: "select id, name from users where active = 1 order by name limit 10",
			want:  "SELECT id, name\n    FROM users\n    WHERE active = 1\n    ORDER BY name\n    LIMIT 10",
		},
		{
			name:  "preserves string literal",
			input: "select * from t where name = 'hello world'",
			want:  "SELECT *\n    FROM t\n    WHERE name = 'hello world'",
		},
		{
			name:  "already uppercase",
			input: "SELECT id FROM users",
			want:  "SELECT id\n    FROM users",
		},
		{
			name:  "group by having",
			input: "select count(*) from events group by type having count(*) > 5",
			want:  "SELECT count(*)\n    FROM events\n    GROUP BY type\n    HAVING count(*) > 5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatSQL(tc.input)
			if got != tc.want {
				t.Errorf("FormatSQL(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}
