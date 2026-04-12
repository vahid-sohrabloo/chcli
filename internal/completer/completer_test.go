package completer

import (
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/schema"
)

func mockCompleter() *Completer {
	return &Completer{
		databases: []string{"default", "system", "analytics"},
		tables: map[string][]string{
			"default":   {"users", "orders", "products"},
			"system":    {"databases", "tables", "columns"},
			"analytics": {"events", "metrics"},
		},
		columns: map[string][]schema.ColumnInfo{
			"default.users": {
				{Name: "id", Type: "UInt64"},
				{Name: "name", Type: "String"},
				{Name: "email", Type: "String"},
				{Name: "created_at", Type: "DateTime"},
			},
			"default.orders": {
				{Name: "id", Type: "UInt64"},
				{Name: "user_id", Type: "UInt64"},
				{Name: "total", Type: "Float64"},
				{Name: "created_at", Type: "DateTime"},
			},
			"default.products": {
				{Name: "id", Type: "UInt64"},
				{Name: "name", Type: "String"},
				{Name: "price", Type: "Float64"},
			},
		},
		functions: []schema.FunctionInfo{
			{Name: "count", IsAggregate: true},
			{Name: "sum", IsAggregate: true},
			{Name: "avg", IsAggregate: true},
			{Name: "min", IsAggregate: true},
			{Name: "max", IsAggregate: true},
			{Name: "toDate"},
			{Name: "now"},
		},
		settings: []string{"max_threads", "max_memory_usage", "max_execution_time"},
		snippets: map[string]string{"active_users": "SELECT count() FROM users"},
	}
}

func hasCompletion(completions []Completion, text string, kind CompletionKind) bool {
	for _, c := range completions {
		if c.Text == text && c.Kind == kind {
			return true
		}
	}
	return false
}

func hasCompletionText(completions []Completion, text string) bool {
	for _, c := range completions {
		if c.Text == text {
			return true
		}
	}
	return false
}

func TestCompleteSelectColumns(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("SELECT ", "default")
	if !hasCompletion(completions, "count", KindAggFunction) {
		t.Errorf("expected 'count' function in SELECT completions, got %v", completions)
	}
}

func TestCompleteFromTables(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("SELECT id FROM ", "default")
	if !hasCompletion(completions, "users", KindTable) {
		t.Errorf("expected 'users' table in FROM completions, got %v", completions)
	}
}

func TestCompleteFromTablesWithPrefix(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("SELECT id FROM us", "default")
	if len(completions) != 1 {
		t.Fatalf("expected exactly 1 completion for prefix 'us', got %d: %v", len(completions), completions)
	}
	if completions[0].Text != "users" {
		t.Errorf("expected 'users', got '%s'", completions[0].Text)
	}
}

func TestCompleteWhereColumns(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("SELECT id FROM users WHERE ", "default")
	if !hasCompletion(completions, "name", KindColumn) {
		t.Errorf("expected 'name' column in WHERE completions, got %v", completions)
	}
}

func TestCompleteUseDatabase(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("USE ", "default")
	if !hasCompletion(completions, "analytics", KindDatabase) {
		t.Errorf("expected 'analytics' database in USE completions, got %v", completions)
	}
}

func TestCompleteEngines(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("CREATE TABLE t (id UInt64) ENGINE = ", "default")
	if !hasCompletion(completions, "MergeTree", KindEngine) {
		t.Errorf("expected 'MergeTree' engine in ENGINE completions, got %v", completions)
	}
}

func TestCompleteSettings(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("SET ", "default")
	if !hasCompletion(completions, "max_threads", KindSetting) {
		t.Errorf("expected 'max_threads' in SET completions, got %v", completions)
	}
}

func TestCompletePrefixFilter(t *testing.T) {
	c := mockCompleter()
	completions := c.Complete("SELECT id FROM users WHERE na", "default")
	if hasCompletionText(completions, "id") {
		t.Errorf("'id' should not appear when prefix is 'na', got %v", completions)
	}
}
