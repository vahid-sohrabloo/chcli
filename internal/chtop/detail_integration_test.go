package chtop

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/vahid-sohrabloo/chcli/internal/conn"
)

func TestSmokeDetails(t *testing.T) {
	cs := os.Getenv("CHCLI_TEST_CONNSTR")
	if cs == "" {
		t.Skip("CHCLI_TEST_CONNSTR not set")
	}
	ctx := context.Background()
	c, err := conn.Connect(ctx, cs)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	must := func(sql string) {
		if err := c.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	must("DROP DATABASE IF EXISTS detail_smoke SYNC")
	must("CREATE DATABASE detail_smoke")
	must("CREATE TABLE detail_smoke.t (ts DateTime, v UInt64) ENGINE=MergeTree PARTITION BY toDate(ts) ORDER BY ts")
	must("INSERT INTO detail_smoke.t SELECT now()-number, number FROM numbers(1000)")
	defer c.Exec(ctx, "DROP DATABASE detail_smoke SYNC")

	d, err := FetchDatabaseDetail(ctx, c, "detail_smoke")
	if err != nil {
		t.Fatalf("db detail: %v", err)
	}
	t.Logf("db detail: engine=%s tables=%d bytes=%d", d.Engine, d.Tables, d.Bytes)

	td, err := FetchTableDetail(ctx, c, "detail_smoke", "t")
	if err != nil {
		t.Fatalf("table detail: %v", err)
	}
	t.Logf("table detail: engine=%s partkey=%q active=%d bytes=%d", td.Engine, td.PartitionKey, td.ActiveParts, td.Bytes)

	ps, err := FetchPartitions(ctx, c, "detail_smoke", "t", "")
	if err != nil {
		t.Fatalf("partitions: %v", err)
	}
	if len(ps) == 0 {
		t.Fatal("expected at least one partition")
	}
	pd, err := FetchPartitionDetail(ctx, c, "detail_smoke", "t", ps[0].Name)
	if err != nil {
		t.Fatalf("partition detail: %v", err)
	}
	t.Logf("partition %s: active=%d bytes=%d lvl=%v", ps[0].Name, pd.ActiveParts, pd.Bytes, pd.LevelCounts)

	prs, err := FetchParts(ctx, c, "detail_smoke", "t", ps[0].Name)
	if err != nil {
		t.Fatalf("parts: %v", err)
	}
	if len(prs) == 0 {
		t.Fatal("expected at least one part")
	}
	part, err := FetchPartDetail(ctx, c, "detail_smoke", "t", prs[0].Name)
	if err != nil {
		t.Fatalf("part detail: %v", err)
	}
	t.Logf("part %s: type=%s level=%d active=%v bytes=%d disk=%s", part.Name, part.PartType, part.Level, part.Active, part.Bytes, part.DiskName)
	_ = fmt.Sprintf
}
