// internal/chtop/queries.go
package chtop

// suppressLogging is appended to every polling SQL. log_queries = 0 and
// log_query_threads = 0 keep the top view out of system.query_log and
// system.query_thread_log. log_comment tags the row for filtering in case
// an operator-side override still logs them.
const suppressLogging = ` SETTINGS log_queries = 0, log_query_threads = 0, log_comment = 'chcli-top'`

// sqlProcesses lists non-idle queries from system.processes. The query text
// is collapsed to a single line with replaceRegexpAll so downstream rendering
// never has to deal with embedded newlines.
const sqlProcesses = `
SELECT
    query_id,
    user,
    toString(initial_address)                                  AS initial_address,
    concat(client_name, ' ', toString(client_version_major),
           '.', toString(client_version_minor))                AS client,
    if(initial_query_id = query_id, database, '')              AS database,
    elapsed,
    read_rows,
    read_bytes,
    memory_usage,
    replaceRegexpAll(query, '\\s+', ' ')                       AS query
FROM system.processes
WHERE query != ''
ORDER BY elapsed DESC
` + suppressLogging

// sqlHeader pivots several system tables into a single row with the headline
// metrics. ifNull handles servers without replicated tables (no rows in
// system.replicas) by returning -1.
const sqlHeader = `
SELECT
    (SELECT uptime())                                           AS uptime_s,
    version()                                                   AS version,
    (SELECT count() FROM system.processes WHERE query != '')    AS active_queries,
    ifNull((SELECT value FROM system.events WHERE event = 'Query'), 0)             AS queries_total,
    ifNull((SELECT value FROM system.events WHERE event = 'InsertedRows'), 0)      AS inserted_rows_total,
    ifNull((SELECT value FROM system.metrics WHERE metric = 'MemoryTracking'), 0)  AS mem_used,
    ifNull((SELECT value FROM system.asynchronous_metrics
            WHERE metric = 'OSMemoryTotal'), 0)                                    AS mem_total,
    ifNull((SELECT value FROM system.metrics WHERE metric = 'Query'), 0)           AS q_running,
    ifNull((SELECT value FROM system.metrics
            WHERE metric = 'BackgroundMergesAndMutationsPoolTask'), 0)             AS merges_running,
    ifNull((SELECT value FROM system.metrics WHERE metric = 'PartMutation'), 0)    AS mutations_running,
    ifNull((SELECT max(absolute_delay) FROM system.replicas), -1)                  AS replica_max_delay
` + suppressLogging
