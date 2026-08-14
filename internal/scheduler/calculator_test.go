package scheduler

import (
	"testing"
	"time"
)

func TestNextDailyAfterOrdinaryAndBoundary(t *testing.T) {
	tests := []struct {
		name      string
		localTime string
		zone      string
		after     time.Time
		want      time.Time
	}{
		{
			name:      "Bangkok same day",
			localTime: "19:00", zone: "Asia/Bangkok",
			after: time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC),
			want:  time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		},
		{
			name:      "year boundary",
			localTime: "00:05", zone: "UTC",
			after: time.Date(2026, 12, 31, 23, 59, 0, 0, time.UTC),
			want:  time.Date(2027, 1, 1, 0, 5, 0, 0, time.UTC),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NextDailyAfter(test.localTime, test.zone, test.after)
			if err != nil {
				t.Fatalf("NextDailyAfter failed: %v", err)
			}
			if !got.Equal(test.want) {
				t.Fatalf("NextDailyAfter = %s, want %s", got, test.want)
			}
		})
	}
}

func TestNextDailyAfterDSTForwardSkipsNonexistentTime(t *testing.T) {
	after := time.Date(2026, 3, 8, 5, 0, 0, 0, time.UTC)
	got, err := NextDailyAfter("02:30", "America/New_York", after)
	if err != nil {
		t.Fatalf("NextDailyAfter failed: %v", err)
	}
	want := time.Date(2026, 3, 9, 6, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("NextDailyAfter = %s, want %s", got, want)
	}
}

func TestNextDailyAfterDSTBackwardUsesFirstOccurrenceOnly(t *testing.T) {
	beforeFirst := time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC)
	got, err := NextDailyAfter("01:30", "America/New_York", beforeFirst)
	if err != nil {
		t.Fatalf("NextDailyAfter before first failed: %v", err)
	}
	first := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	if !got.Equal(first) {
		t.Fatalf("first occurrence = %s, want %s", got, first)
	}

	afterFirstBeforeRepeated := time.Date(2026, 11, 1, 5, 45, 0, 0, time.UTC)
	got, err = NextDailyAfter("01:30", "America/New_York", afterFirstBeforeRepeated)
	if err != nil {
		t.Fatalf("NextDailyAfter after first failed: %v", err)
	}
	nextDay := time.Date(2026, 11, 2, 6, 30, 0, 0, time.UTC)
	if !got.Equal(nextDay) {
		t.Fatalf("post-fallback occurrence = %s, want next day %s", got, nextDay)
	}
}

func TestNextDailyAfterRejectsInvalidInputs(t *testing.T) {
	for _, test := range []struct {
		localTime string
		zone      string
	}{
		{localTime: "9:00", zone: "UTC"},
		{localTime: "24:00", zone: "UTC"},
		{localTime: "09:00", zone: "Mars/Olympus"},
		{localTime: "09:00", zone: "Local"},
	} {
		if _, err := NextDailyAfter(test.localTime, test.zone, time.Now()); err == nil {
			t.Fatalf("expected error for %q %q", test.localTime, test.zone)
		}
	}
}
