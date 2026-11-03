package logx

import (
	"fmt"
	"testing"
)

func TestRingKeepsTheNewestThousandEntriesInOrder(t *testing.T) {
	logger, _, _ := newTestLogger(t, Options{})
	total := RingCapacity + 5
	for i := range total {
		logger.Info(fmt.Sprintf("m%d", i), nil)
	}
	entries := logger.Recent(0)
	if len(entries) != RingCapacity {
		t.Fatalf("ring holds %d entries, want %d", len(entries), RingCapacity)
	}
	if entries[0].Message != "m5" || entries[len(entries)-1].Message != fmt.Sprintf("m%d", total-1) {
		t.Fatalf("ring bounds = %q .. %q", entries[0].Message, entries[len(entries)-1].Message)
	}
}

func TestRecentLimitReturnsTheNewestEntries(t *testing.T) {
	logger, _, _ := newTestLogger(t, Options{})
	for i := range 4 {
		logger.Info(fmt.Sprintf("n%d", i), nil)
	}
	got := logger.Recent(2)
	if len(got) != 2 || got[0].Message != "n2" || got[1].Message != "n3" {
		t.Fatalf("Recent(2) = %+v", got)
	}
}

func TestRecentReturnsCopiesOfFields(t *testing.T) {
	logger, _, _ := newTestLogger(t, Options{})
	logger.Info("copy", F{"k": "v"})
	first := logger.Recent(0)
	first[0].Fields["k"] = "changed"
	if logger.Recent(0)[0].Fields["k"] != "v" {
		t.Fatal("a caller changed the ring through Recent")
	}
}
