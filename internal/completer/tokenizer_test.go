package completer

import (
	"testing"
)

func TestDetectClauseSelect(t *testing.T) {
	clause := DetectClause("SELECT ")
	if clause != ClauseSelect {
		t.Errorf("expected ClauseSelect, got %v", clause)
	}
}

func TestDetectClauseFrom(t *testing.T) {
	clause := DetectClause("SELECT id FROM ")
	if clause != ClauseFrom {
		t.Errorf("expected ClauseFrom, got %v", clause)
	}
}

func TestDetectClauseWhere(t *testing.T) {
	clause := DetectClause("SELECT id FROM users WHERE ")
	if clause != ClauseWhere {
		t.Errorf("expected ClauseWhere, got %v", clause)
	}
}

func TestDetectClauseJoin(t *testing.T) {
	clause := DetectClause("SELECT id FROM users JOIN ")
	if clause != ClauseJoin {
		t.Errorf("expected ClauseJoin, got %v", clause)
	}
}

func TestDetectClauseOrderBy(t *testing.T) {
	clause := DetectClause("SELECT id FROM users ORDER BY ")
	if clause != ClauseOrderBy {
		t.Errorf("expected ClauseOrderBy, got %v", clause)
	}
}

func TestDetectClauseGroupBy(t *testing.T) {
	clause := DetectClause("SELECT id FROM users GROUP BY ")
	if clause != ClauseGroupBy {
		t.Errorf("expected ClauseGroupBy, got %v", clause)
	}
}

func TestDetectClauseEngine(t *testing.T) {
	clause := DetectClause("CREATE TABLE t (id UInt64) ENGINE = ")
	if clause != ClauseEngine {
		t.Errorf("expected ClauseEngine, got %v", clause)
	}
}

func TestDetectClauseUse(t *testing.T) {
	clause := DetectClause("USE ")
	if clause != ClauseUse {
		t.Errorf("expected ClauseUse, got %v", clause)
	}
}

func TestDetectClauseSet(t *testing.T) {
	clause := DetectClause("SET ")
	if clause != ClauseSet {
		t.Errorf("expected ClauseSet, got %v", clause)
	}
}

func TestExtractTablesFromQuery(t *testing.T) {
	tables := ExtractTables("SELECT id FROM users JOIN orders ON users.id = orders.user_id WHERE ")
	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %v", len(tables), tables)
	}
	found := make(map[string]bool)
	for _, tbl := range tables {
		found[tbl] = true
	}
	if !found["users"] {
		t.Errorf("expected 'users' in tables, got %v", tables)
	}
	if !found["orders"] {
		t.Errorf("expected 'orders' in tables, got %v", tables)
	}
}
