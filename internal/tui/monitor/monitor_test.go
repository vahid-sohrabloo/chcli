package monitor

import "testing"

func TestKeyHintZeroValue(t *testing.T) {
	var k keyHint
	if k.Key != "" || k.Desc != "" {
		t.Fatalf("zero keyHint should be empty strings")
	}
}
