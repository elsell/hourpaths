package activity

import (
	"testing"
	"time"
)

func TestEditOrderPreservesCausalityWhenClockMovesBackAndBreaksIndependentTies(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	first, err := NewActivityEditOrder(now, 0, "operation-a")
	if err != nil {
		t.Fatal(err)
	}
	next, err := NextActivityEditOrder(now.Add(-time.Hour), first, "operation-b")
	if err != nil || next.Compare(first) <= 0 || !next.AuthoredAt.Equal(first.AuthoredAt) || next.Counter != 1 {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	independent, err := NewActivityEditOrder(now, 0, "operation-z")
	if err != nil || independent.Compare(first) <= 0 || first.Compare(independent) >= 0 {
		t.Fatal("tie order is not stable")
	}
	newer, err := NewActivityEditOrder(now.Add(time.Second), 0, "operation-0")
	if err != nil || newer.Compare(next) <= 0 {
		t.Fatal("newer independent edit must win regardless of delivery order")
	}
}

func TestEditOrderRejectsMalformedStampAndCounterOverflow(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	for _, value := range []struct {
		instant time.Time
		counter int64
		id      string
	}{
		{time.Time{}, 0, "id"}, {now, -1, "id"}, {now, 9007199254740992, "id"}, {now, 0, ""}, {now, 0, "a\nb"},
	} {
		if _, err := NewActivityEditOrder(value.instant, value.counter, value.id); err == nil {
			t.Fatalf("accepted %+v", value)
		}
	}
	largest, err := NewActivityEditOrder(now, 9007199254740991, "id")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NextActivityEditOrder(now, largest, "next"); err == nil {
		t.Fatal("logical clock overflow accepted")
	}
}
