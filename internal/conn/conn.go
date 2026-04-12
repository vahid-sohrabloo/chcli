package conn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/vahid-sohrabloo/chconn/v3"
	"github.com/vahid-sohrabloo/chconn/v3/types"
)

// Conn wraps a chconn native ClickHouse connection.
type Conn struct {
	raw     chconn.Conn
	connStr string
}

// ResultColumn holds the name and type of a query result column.
type ResultColumn struct {
	Name string
	Type string
}

const MaxRows = 2000 // max rows to keep in memory

// QueryResult holds the result of a SELECT query.
type QueryResult struct {
	Columns   []ResultColumn
	Rows      [][]string
	TotalRows int    // rows read (may be < actual if truncated)
	Truncated bool   // true if stopped early at MaxRows
	QueryID   string // ClickHouse query ID for cancellation
	Elapsed   time.Duration
}

// Connect establishes a connection to ClickHouse using the given connection string.
func Connect(ctx context.Context, connStr string) (*Conn, error) {
	raw, err := chconn.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &Conn{raw: raw, connStr: connStr}, nil
}

// QueryAll executes a SQL query and returns ALL rows (no row limit).
// Used for internal queries like schema loading.
func (c *Conn) QueryAll(ctx context.Context, sql string) (*QueryResult, error) {
	start := time.Now()

	rows, err := c.raw.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols := rows.Columns()
	result := &QueryResult{
		Columns: make([]ResultColumn, len(cols)),
	}
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

// Query executes a SQL query with an auto-generated query ID.
func (c *Conn) Query(ctx context.Context, sql string, progress ...*Progress) (*QueryResult, error) {
	return c.QueryWithID(ctx, sql, generateQueryID(), progress...)
}

// QueryWithID executes a SQL query with a specific query ID (for cancellation).
func (c *Conn) QueryWithID(ctx context.Context, sql string, queryID string, progress ...*Progress) (*QueryResult, error) {
	start := time.Now()

	opts := &chconn.QueryOptions{
		QueryID: queryID,
		Settings: chconn.Settings{
			{Name: "max_result_rows", Value: strconv.Itoa(MaxRows)},
			{Name: "result_overflow_mode", Value: "break"},
		},
	}

	// If a progress pointer is provided, attach callbacks.
	var prog *Progress
	if len(progress) > 0 && progress[0] != nil {
		prog = progress[0]
		prog.Metrics = make(map[string]int64)
		threadIDs := make(map[uint64]struct{})

		opts.OnProgress = func(p *chconn.Progress) {
			prog.ReadRows += p.ReadRows
			prog.ReadBytes += p.ReadBytes
			prog.TotalRows += p.TotalRows
			prog.TotalBytes += p.TotalBytes
			prog.WrittenRows += p.WriterRows
			prog.WrittenBytes += p.WrittenBytes
			prog.Elapsed = time.Since(start)
		}
		opts.OnProfileEvent = func(pe *chconn.ProfileEvent) {
			for i := range pe.Name.NumRow() {
				name := pe.Name.Row(i)
				val := pe.Value.Row(i)
				evType := pe.Type.Row(i)
				tid := pe.ThreadID.Row(i)
				if tid > 0 {
					threadIDs[tid] = struct{}{}
				}
				if evType == 2 {
					prog.Metrics[name] = val
				} else {
					prog.Metrics[name] += val
				}
				switch name {
				case "MemoryTrackerUsage":
					prog.MemoryUsage = val
					if val > prog.PeakMemory {
						prog.PeakMemory = val
					}
				case "UserTimeMicroseconds":
					prog.CPUUser = prog.Metrics[name]
				case "SystemTimeMicroseconds":
					prog.CPUSystem = prog.Metrics[name]
				case "OSReadBytes":
					prog.DiskRead = prog.Metrics[name]
				case "OSWriteBytes":
					prog.DiskWrite = prog.Metrics[name]
				}
			}
			prog.Threads = len(threadIDs)
			prog.Elapsed = time.Since(start)
		}
	}
	rows, err := c.raw.QueryWithOption(ctx, sql, opts)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	// Extract column metadata.
	cols := rows.Columns()
	result := &QueryResult{
		Columns: make([]ResultColumn, len(cols)),
	}
	for i, col := range cols {
		result.Columns[i] = ResultColumn{
			Name: string(col.Name()),
			Type: string(col.Type()),
		}
	}

	// Iterate rows — stop after MaxRows for fast display.
	for rows.Next() {
		result.TotalRows++
		vals := rows.Values()
		row := make([]string, len(vals))
		for i, v := range vals {
			row[i] = formatValue(v)
		}
		result.Rows = append(result.Rows, row)

		if result.TotalRows >= MaxRows {
			result.Truncated = true
			break // don't wait for remaining rows
		}
	}

	if !result.Truncated {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("rows: %w", err)
		}
	}

	result.Elapsed = time.Since(start)
	return result, nil
}

// formatValue converts a ClickHouse value to a display string.
func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case types.UUID:
		return formatUUID(val)
	case *types.UUID:
		if val == nil {
			return "NULL"
		}
		return formatUUID(*val)
	case net.IP:
		return val.String()
	case []byte:
		return string(val)
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int8:
		return strconv.FormatInt(int64(val), 10)
	case int16:
		return strconv.FormatInt(int64(val), 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint8:
		return strconv.FormatUint(uint64(val), 10)
	case uint16:
		return strconv.FormatUint(uint64(val), 10)
	case uint32:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float32:
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(float64(val), 'f', -1, 32), "0"), ".")
	case float64:
		return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(val, 'f', -1, 64), "0"), ".")
	case []any:
		parts := make([]string, len(val))
		for i, elem := range val {
			parts[i] = formatValue(elem)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		pairs := make([]string, 0, len(val))
		for k, mv := range val {
			pairs = append(pairs, k+": "+formatValue(mv))
		}
		return "{" + strings.Join(pairs, ", ") + "}"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatUUID formats a [16]byte UUID as xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func formatUUID(u types.UUID) string {
	var buf [36]byte
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], u[10:16])
	return string(buf[:])
}

// GenerateQueryID creates a unique query ID.
func GenerateQueryID() string {
	return generateQueryID()
}

// Progress holds real-time query execution stats from ClickHouse.
type Progress struct {
	ReadRows     uint64
	ReadBytes    uint64
	TotalRows    uint64
	TotalBytes   uint64
	WrittenRows  uint64
	WrittenBytes uint64
	Elapsed      time.Duration    // client-side elapsed
	MemoryUsage  int64            // current memory (gauge)
	PeakMemory   int64            // peak memory seen
	CPUUser      int64            // UserTimeMicroseconds (cumulative)
	CPUSystem    int64            // SystemTimeMicroseconds (cumulative)
	DiskRead     int64            // OSReadBytes (cumulative)
	DiskWrite    int64            // OSWriteBytes (cumulative)
	Threads      int              // unique thread IDs seen
	Metrics      map[string]int64 // all ProfileEvent metrics
}

// QueryWithProgress executes a query and sends progress updates to the channel.
// The channel is closed when the query completes.
func (c *Conn) QueryWithProgress(ctx context.Context, sql string, progressCh chan<- Progress) (*QueryResult, error) {
	start := time.Now()

	var cumulative Progress
	cumulative.Metrics = make(map[string]int64)
	threadIDs := make(map[uint64]struct{})

	sendProgress := func() {
		cumulative.Elapsed = time.Since(start)
		cumulative.Threads = len(threadIDs)
		select {
		case progressCh <- cumulative:
		default:
		}
	}

	opts := &chconn.QueryOptions{
		OnProgress: func(p *chconn.Progress) {
			cumulative.ReadRows += p.ReadRows
			cumulative.ReadBytes += p.ReadBytes
			cumulative.TotalRows += p.TotalRows
			cumulative.TotalBytes += p.TotalBytes
			cumulative.WrittenRows += p.WriterRows
			cumulative.WrittenBytes += p.WrittenBytes
			sendProgress()
		},
		OnProfileEvent: func(pe *chconn.ProfileEvent) {
			for i := range pe.Name.NumRow() {
				name := pe.Name.Row(i)
				val := pe.Value.Row(i)
				evType := pe.Type.Row(i)
				tid := pe.ThreadID.Row(i)

				if tid > 0 {
					threadIDs[tid] = struct{}{}
				}

				if evType == 2 {
					// Gauge: use value directly (e.g., MemoryTrackerUsage).
					cumulative.Metrics[name] = val
				} else {
					// Increment: accumulate.
					cumulative.Metrics[name] += val
				}

				switch name {
				case "MemoryTrackerUsage":
					cumulative.MemoryUsage = val // gauge: current value
					if val > cumulative.PeakMemory {
						cumulative.PeakMemory = val
					}
				case "UserTimeMicroseconds":
					cumulative.CPUUser = cumulative.Metrics[name]
				case "SystemTimeMicroseconds":
					cumulative.CPUSystem = cumulative.Metrics[name]
				case "OSReadBytes":
					cumulative.DiskRead = cumulative.Metrics[name]
				case "OSWriteBytes":
					cumulative.DiskWrite = cumulative.Metrics[name]
				}
			}
			sendProgress()
		},
		OnProfile: func(p *chconn.Profile) {
			sendProgress()
		},
	}

	rows, err := c.raw.QueryWithOption(ctx, sql, opts)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols := rows.Columns()
	result := &QueryResult{
		Columns: make([]ResultColumn, len(cols)),
	}
	for i, col := range cols {
		result.Columns[i] = ResultColumn{
			Name: string(col.Name()),
			Type: string(col.Type()),
		}
	}

	for rows.Next() {
		result.TotalRows++
		if result.TotalRows <= MaxRows {
			vals := rows.Values()
			row := make([]string, len(vals))
			for i, v := range vals {
				row[i] = formatValue(v)
			}
			result.Rows = append(result.Rows, row)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	result.Elapsed = time.Since(start)
	return result, nil
}

// Exec executes a SQL statement that returns no rows.
func (c *Conn) Exec(ctx context.Context, sql string) error {
	if err := c.raw.Exec(ctx, sql); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.raw.Close()
}

// Raw returns the underlying chconn.Conn.
func (c *Conn) Raw() chconn.Conn {
	return c.raw
}

// generateQueryID creates a unique query ID for ClickHouse.
func generateQueryID() string {
	var b [8]byte
	rand.Read(b[:])
	return fmt.Sprintf("chcli-%x", b)
}

// KillQuery sends KILL QUERY to the ClickHouse server on a separate connection.
func (c *Conn) KillQuery(queryID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	killConn, err := chconn.Connect(ctx, c.connStr)
	if err != nil {
		return fmt.Errorf("kill query connect: %w", err)
	}
	defer killConn.Close()

	return killConn.Exec(ctx, fmt.Sprintf("KILL QUERY WHERE query_id = '%s'", queryID))
}

// LastQueryID returns the query ID that will be used for the current/next query.
// This is set during Query execution.

// ServerVersion returns the ClickHouse server version string.
func (c *Conn) ServerVersion() string {
	info := c.raw.ServerInfo()
	if info == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d.%d", info.MajorVersion, info.MinorVersion, info.ServerVersionPatch)
}

// Reconnect closes the current connection and opens a new one using the same
// connection string. It retries up to 3 times with increasing backoff.
func (c *Conn) Reconnect(ctx context.Context) error {
	_ = c.raw.Close()
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		raw, err := chconn.Connect(ctx, c.connStr)
		if err != nil {
			lastErr = err
			continue
		}
		c.raw = raw
		return nil
	}
	return fmt.Errorf("reconnect failed after 3 attempts: %w", lastErr)
}
