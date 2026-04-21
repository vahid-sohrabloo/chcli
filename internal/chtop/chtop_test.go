package chtop

import "testing"

func TestSnapshotZeroValue(t *testing.T) {
	var s Snapshot
	if s.Processes != nil {
		t.Fatalf("zero Snapshot.Processes should be nil, got %v", s.Processes)
	}
}
