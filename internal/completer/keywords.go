package completer

// sqlKeywords contains SQL and ClickHouse-specific keywords for autocompletion.
var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "BETWEEN", "LIKE", "ILIKE",
	"IS", "NULL", "AS", "ON", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "CROSS",
	"FULL", "SEMI", "ANTI", "ANY", "ALL", "UNION", "EXCEPT", "INTERSECT", "INSERT",
	"INTO", "VALUES", "UPDATE", "DELETE", "ALTER", "DROP", "CREATE", "TABLE",
	"DATABASE", "VIEW", "INDEX", "IF", "EXISTS", "TEMPORARY", "ORDER", "BY", "GROUP",
	"HAVING", "LIMIT", "OFFSET", "WITH", "DISTINCT", "ASC", "DESC", "CASE", "WHEN",
	"THEN", "ELSE", "END", "MATERIALIZED", "POPULATE", "ENGINE", "PARTITION",
	"PRIMARY", "KEY", "SAMPLE", "TTL", "SETTINGS", "FORMAT", "PREWHERE", "FINAL",
	"GLOBAL", "LOCAL", "ATTACH", "DETACH", "OPTIMIZE", "RENAME", "SHOW", "DESCRIBE",
	"USE", "SET", "GRANT", "REVOKE", "KILL", "QUERY", "SYSTEM", "RELOAD", "FLUSH",
	"TRUNCATE", "EXPLAIN",
}

// engineNames contains ClickHouse table engine names for autocompletion.
var engineNames = []string{
	"MergeTree", "ReplacingMergeTree", "SummingMergeTree", "AggregatingMergeTree",
	"CollapsingMergeTree", "VersionedCollapsingMergeTree", "GraphiteMergeTree",
	"ReplicatedMergeTree", "ReplicatedReplacingMergeTree", "ReplicatedSummingMergeTree",
	"ReplicatedAggregatingMergeTree", "ReplicatedCollapsingMergeTree",
	"ReplicatedVersionedCollapsingMergeTree", "Log", "TinyLog", "StripeLog", "Memory",
	"Buffer", "Distributed", "Dictionary", "Merge", "File", "Null", "Set", "URL",
	"View", "LiveView", "Kafka", "MySQL", "PostgreSQL", "S3", "EmbeddedRocksDB",
}
