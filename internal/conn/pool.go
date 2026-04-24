package conn

import (
	"context"
	"fmt"
	"time"

	"github.com/vahid-sohrabloo/chconn/v3"
	"github.com/vahid-sohrabloo/chconn/v3/chpool"
)

// Pool is a chconn connection pool for concurrent monitoring queries —
// Charts tab fan-out, Storage/Merges refresh, processes/header polls.
//
// The user REPL stays on its own session *Conn so USE/SET/named-SELECT
// state is preserved. The pool acquires a fresh conn per query and drops
// dead sockets transparently, so a server-side protocol error on one
// panel does not propagate "connection closed" to the others.
type Pool struct {
	raw chpool.Pool
}

// OpenPool opens a connection pool using the given connection string.
// It performs one Acquire so startup errors surface immediately rather
// than on the first tab switch.
func OpenPool(ctx context.Context, connStr string) (*Pool, error) {
	raw, err := chpool.New(connStr)
	if err != nil {
		return nil, fmt.Errorf("pool: %w", err)
	}
	c, err := raw.Acquire(ctx)
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("pool acquire: %w", err)
	}
	c.Release()
	return &Pool{raw: raw}, nil
}

// Close closes all pool connections and rejects future acquires.
func (p *Pool) Close() { p.raw.Close() }

// QueryAll runs sql through the pool and materializes every row.
// Satisfies chtop.Querier.
func (p *Pool) QueryAll(ctx context.Context, sql string) (*QueryResult, error) {
	return p.queryAll(ctx, sql)
}

// QueryAllWithParams runs parameterised sql through the pool.
// Satisfies chtop.ParamQuerier.
func (p *Pool) QueryAllWithParams(ctx context.Context, sql string, params ...chconn.Parameter) (*QueryResult, error) {
	return p.queryAll(ctx, sql, params...)
}

func (p *Pool) queryAll(ctx context.Context, sql string, params ...chconn.Parameter) (*QueryResult, error) {
	start := time.Now()
	rows, err := p.raw.Query(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols := rows.Columns()
	result := &QueryResult{Columns: make([]ResultColumn, len(cols))}
	for i, col := range cols {
		result.Columns[i] = ResultColumn{
			Name: string(col.Name()),
			Type: string(col.Type()),
		}
	}
	for rows.Next() {
		result.TotalRows++
		vals := rows.Values()
		row := make([]string, len(vals))
		for i, v := range vals {
			row[i] = formatValue(v)
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	result.Elapsed = time.Since(start)
	return result, nil
}
