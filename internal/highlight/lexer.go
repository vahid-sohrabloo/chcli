package highlight

import (
	"github.com/alecthomas/chroma/v2"
)

// ClickHouseLexer is a custom chroma lexer for ClickHouse SQL.
var ClickHouseLexer = chroma.MustNewLexer(
	&chroma.Config{
		Name:            "ClickHouse SQL",
		Aliases:         []string{"clickhouse", "chsql"},
		CaseInsensitive: true,
	},
	func() chroma.Rules {
		return chroma.Rules{
			"root": {
				// Whitespace
				{Pattern: `\s+`, Type: chroma.TextWhitespace, Mutator: nil},

				// Single-line comment
				{Pattern: `--.*$`, Type: chroma.CommentSingle, Mutator: nil},

				// Multi-line comment
				{Pattern: `/\*`, Type: chroma.CommentMultiline, Mutator: chroma.Push("comment")},

				// String literal
				{Pattern: `'`, Type: chroma.LiteralStringSingle, Mutator: chroma.Push("string")},

				// Numbers: float before integer so the float rule wins
				{Pattern: `\b\d+\.\d+\b`, Type: chroma.LiteralNumberFloat, Mutator: nil},
				{Pattern: `\b\d+\b`, Type: chroma.LiteralNumberInteger, Mutator: nil},

				// ClickHouse storage engines
				{
					Pattern: chroma.Words(`(?i)\b`, `\b`,
						"MergeTree", "ReplacingMergeTree", "SummingMergeTree",
						"AggregatingMergeTree", "CollapsingMergeTree",
						"VersionedCollapsingMergeTree", "GraphiteMergeTree",
						"ReplicatedMergeTree", "ReplicatedReplacingMergeTree",
						"ReplicatedSummingMergeTree", "ReplicatedAggregatingMergeTree",
						"ReplicatedCollapsingMergeTree", "ReplicatedVersionedCollapsingMergeTree",
						"Log", "TinyLog", "StripeLog", "Memory", "Buffer", "Distributed",
						"MaterializedView", "Dictionary", "Merge", "File", "Null", "Set",
						"Join", "URL", "View", "LiveView", "Kafka", "MySQL", "PostgreSQL",
						"S3", "EmbeddedRocksDB",
					),
					Type:    chroma.NameBuiltin,
					Mutator: nil,
				},

				// ClickHouse types
				{
					Pattern: chroma.Words(`(?i)\b`, `\b`,
						"UInt8", "UInt16", "UInt32", "UInt64", "UInt128", "UInt256",
						"Int8", "Int16", "Int32", "Int64", "Int128", "Int256",
						"Float32", "Float64",
						"Decimal", "Decimal32", "Decimal64", "Decimal128", "Decimal256",
						"String", "FixedString",
						"UUID",
						"Date", "Date32", "DateTime", "DateTime64",
						"Enum8", "Enum16",
						"Array", "Tuple", "Map", "Nested", "Nullable", "Nothing",
						"IPv4", "IPv6",
						"LowCardinality",
						"SimpleAggregateFunction", "AggregateFunction",
						"Bool",
					),
					Type:    chroma.KeywordType,
					Mutator: nil,
				},

				// SQL keywords
				{
					Pattern: chroma.Words(`(?i)\b`, `\b`,
						"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "BETWEEN",
						"LIKE", "ILIKE", "IS", "NULL", "AS", "ON", "JOIN", "LEFT",
						"RIGHT", "INNER", "OUTER", "CROSS", "FULL", "SEMI", "ANTI",
						"ANY", "ALL", "UNION", "EXCEPT", "INTERSECT",
						"INSERT", "INTO", "VALUES", "UPDATE", "DELETE",
						"ALTER", "DROP", "CREATE", "TABLE", "DATABASE", "VIEW",
						"INDEX", "IF", "EXISTS", "TEMPORARY",
						"ORDER", "BY", "GROUP", "HAVING", "LIMIT", "OFFSET",
						"WITH", "DISTINCT", "ASC", "DESC",
						"CASE", "WHEN", "THEN", "ELSE", "END",
						"MATERIALIZED", "POPULATE", "ENGINE", "PARTITION", "PRIMARY",
						"KEY", "SAMPLE", "TTL", "SETTINGS", "FORMAT",
						"PREWHERE", "FINAL", "GLOBAL", "LOCAL",
						"ATTACH", "DETACH", "OPTIMIZE", "RENAME",
						"SHOW", "DESCRIBE", "USE", "SET",
						"GRANT", "REVOKE", "KILL", "QUERY",
						"SYSTEM", "RELOAD", "FLUSH", "TRUNCATE", "EXPLAIN",
					),
					Type:    chroma.Keyword,
					Mutator: nil,
				},

				// Common ClickHouse / SQL functions (followed by opening paren)
				{
					Pattern: chroma.Words(`(?i)\b`, `(?=\()`,
						"count", "sum", "avg", "min", "max",
						"any", "anyLast",
						"groupArray", "groupUniqArray",
						"argMin", "argMax",
						"uniq", "uniqExact", "uniqHLL12", "uniqCombined",
						"quantile", "quantiles", "median",
						"toUInt8", "toUInt16", "toUInt32", "toUInt64",
						"toInt8", "toInt16", "toInt32", "toInt64",
						"toFloat32", "toFloat64",
						"toString", "toDate", "toDateTime", "toDateTime64",
						"toTypeName",
						"toStartOfDay", "toStartOfHour", "toStartOfMinute",
						"toStartOfMonth", "toStartOfYear", "toStartOfWeek",
						"now", "today", "yesterday",
						"length", "empty", "notEmpty", "reverse",
						"lower", "upper", "trim", "trimLeft", "trimRight",
						"substring", "concat", "replaceOne", "replaceAll",
						"splitByChar", "splitByString",
						"arrayJoin", "arrayMap", "arrayFilter",
						"has", "hasAll", "hasAny", "indexOf",
						"if", "multiIf", "coalesce",
						"formatReadableSize", "formatReadableQuantity",
						"dictGet", "dictGetOrDefault",
						"sipHash64", "cityHash64", "murmurHash3_64",
					),
					Type:    chroma.NameFunction,
					Mutator: nil,
				},

				// Operators
				{Pattern: `[+\-*/<>=!~&|^]`, Type: chroma.Operator, Mutator: nil},

				// Punctuation
				{Pattern: `[;:()\[\],.]`, Type: chroma.Punctuation, Mutator: nil},

				// Backtick-quoted identifier
				{Pattern: "`[^`]*`", Type: chroma.NameOther, Mutator: nil},

				// Double-quoted identifier
				{Pattern: `"[^"]*"`, Type: chroma.NameOther, Mutator: nil},

				// Plain identifier
				{Pattern: `[a-zA-Z_]\w*`, Type: chroma.Name, Mutator: nil},
			},

			"comment": {
				// Nested /* ... */
				{Pattern: `/\*`, Type: chroma.CommentMultiline, Mutator: chroma.Push("comment")},
				{Pattern: `\*/`, Type: chroma.CommentMultiline, Mutator: chroma.Pop(1)},
				{Pattern: `[^/*]+`, Type: chroma.CommentMultiline, Mutator: nil},
				{Pattern: `[/*]`, Type: chroma.CommentMultiline, Mutator: nil},
			},

			"string": {
				// Escaped single quote (two single quotes in a row)
				{Pattern: `''`, Type: chroma.LiteralStringEscape, Mutator: nil},
				// Backslash escape sequences
				{Pattern: `\\.`, Type: chroma.LiteralStringEscape, Mutator: nil},
				// Normal string content
				{Pattern: `[^'\\]+`, Type: chroma.LiteralStringSingle, Mutator: nil},
				// Closing quote
				{Pattern: `'`, Type: chroma.LiteralStringSingle, Mutator: chroma.Pop(1)},
			},
		}
	},
)
