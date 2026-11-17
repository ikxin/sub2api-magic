package timezone

import (
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	// Test with valid timezone
	err := Init("Asia/Shanghai")
	if err != nil {
		t.Fatalf("Init failed with valid timezone: %v", err)
	}

	// Verify time.Local was set
	if time.Local.String() != "Asia/Shanghai" {
		t.Errorf("time.Local not set correctly, got %s", time.Local.String())
	}

	// Verify our location variable
	if Location().String() != "Asia/Shanghai" {
		t.Errorf("Location() not set correctly, got %s", Location().String())
	}

	// Test Name()
	if Name() != "Asia/Shanghai" {
		t.Errorf("Name() not set correctly, got %s", Name())
	}
}

func TestInitInvalidTimezone(t *testing.T) {
	err := Init("Invalid/Timezone")
	if err == nil {
		t.Error("Init should fail with invalid timezone")
	}
}

func TestTimeNowAffected(t *testing.T) {
	// Reset to UTC first
	if err := Init("UTC"); err != nil {
		t.Fatalf("Init failed with UTC: %v", err)
	}
	utcNow := time.Now()

	// Switch to Shanghai (UTC+8)
	if err := Init("Asia/Shanghai"); err != nil {
		t.Fatalf("Init failed with Asia/Shanghai: %v", err)
	}
	shanghaiNow := time.Now()

	// The times should be the same instant, but different timezone representation
	// Shanghai should be 8 hours ahead in display
	_, utcOffset := utcNow.Zone()
	_, shanghaiOffset := shanghaiNow.Zone()

	expectedDiff := 8 * 3600 // 8 hours in seconds
	actualDiff := shanghaiOffset - utcOffset

	if actualDiff != expectedDiff {
		t.Errorf("Timezone offset difference incorrect: expected %d, got %d", expectedDiff, actualDiff)
	}
}

func TestToday(t *testing.T) {
	if err := Init("Asia/Shanghai"); err != nil {
		t.Fatalf("Init failed with Asia/Shanghai: %v", err)
	}

	today := Today()
	now := Now()

	// Today should be at 00:00:00
	if today.Hour() != 0 || today.Minute() != 0 || today.Second() != 0 {
		t.Errorf("Today() not at start of day: %v", today)
	}

	// Today should be same date as now
	if today.Year() != now.Year() || today.Month() != now.Month() || today.Day() != now.Day() {
		t.Errorf("Today() date mismatch: today=%v, now=%v", today, now)
	}
}

func TestStartOfDay(t *testing.T) {
	if err := Init("Asia/Shanghai"); err != nil {
		t.Fatalf("Init failed with Asia/Shanghai: %v", err)
	}

	// Create a time at 15:30:45
	testTime := time.Date(2024, 6, 15, 15, 30, 45, 123456789, Location())
	startOfDay := StartOfDay(testTime)

	expected := time.Date(2024, 6, 15, 0, 0, 0, 0, Location())
	if !startOfDay.Equal(expected) {
		t.Errorf("StartOfDay incorrect: expected %v, got %v", expected, startOfDay)
	}
}

func TestTruncateVsStartOfDay(t *testing.T) {
	// This test demonstrates why Truncate(24*time.Hour) can be problematic
	// and why StartOfDay is more reliable for timezone-aware code

	if err := Init("Asia/Shanghai"); err != nil {
		t.Fatalf("Init failed with Asia/Shanghai: %v", err)
	}

	now := Now()

	// Truncate operates on UTC, not local time
	truncated := now.Truncate(24 * time.Hour)

	// StartOfDay operates on local time
	startOfDay := StartOfDay(now)

	// These will likely be different for non-UTC timezones
	t.Logf("Now: %v", now)
	t.Logf("Truncate(24h): %v", truncated)
	t.Logf("StartOfDay: %v", startOfDay)

	// The truncated time may not be at local midnight
	// StartOfDay is always at local midnight
	if startOfDay.Hour() != 0 {
		t.Errorf("StartOfDay should be at hour 0, got %d", startOfDay.Hour())
	}
}

func TestDSTAwareness(t *testing.T) {
	// Test with a timezone that has DST (America/New_York)
	err := Init("America/New_York")
	if err != nil {
		t.Skipf("America/New_York timezone not available: %v", err)
	}

	// Just verify it doesn't crash
	_ = Today()
	_ = Now()
	_ = StartOfDay(Now())
}

func TestStartOfWeek_Boundaries(t *testing.T) {
	if err := Init("Asia/Shanghai"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = Init("UTC") })

	loc := Location()
	wantMon := time.Date(2026, 5, 18, 0, 0, 0, 0, loc) // 2026-05-18 是周一

	cases := []struct {
		name string
		in   time.Time
	}{
		{"friday", time.Date(2026, 5, 22, 14, 30, 0, 0, loc)},
		{"sunday", time.Date(2026, 5, 24, 10, 0, 0, 0, loc)},
		{"monday-self", time.Date(2026, 5, 18, 9, 15, 30, 0, loc)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StartOfWeek(c.in); !got.Equal(wantMon) {
				t.Errorf("StartOfWeek(%v) = %v, want %v", c.in, got, wantMon)
			}
		})
	}
}

func TestParseRangeBoundary(t *testing.T) {
	for _, tt := range []struct {
		name, value, zone, want string
		end                     bool
	}{
		{"date start", "2026-11-16", "Asia/Shanghai", "2026-11-15T16:00:00Z", false},
		{"date end", "2026-11-16", "Asia/Shanghai", "2026-11-16T16:00:00Z", true},
		{"timestamp start", "2026-11-15T15:42:13.456+08:00", "UTC", "2026-11-15T07:42:13.456Z", false},
		{"timestamp end", "2026-11-16T15:42:13.456+08:00", "Asia/Shanghai", "2026-11-16T07:42:13.456Z", true},
		{"timestamp midnight end", "2026-11-16T00:00:00Z", "UTC", "2026-11-16T00:00:00Z", true},
		{"spring DST end", "2026-03-08", "America/New_York", "2026-03-09T04:00:00Z", true},
		{"fall DST end", "2026-11-01", "America/New_York", "2026-11-02T05:00:00Z", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRangeBoundary(tt.value, tt.zone, tt.end)
			if err != nil {
				t.Fatal(err)
			}
			if got.UTC().Format(time.RFC3339Nano) != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
			formatted := FormatRangeBoundary(got, tt.end)
			reparsed, err := ParseRangeBoundary(formatted, tt.zone, tt.end)
			if err != nil || !reparsed.Equal(got) {
				t.Fatalf("boundary did not round trip: %s: %v", formatted, err)
			}
		})
	}
	for _, value := range []string{"bad", "2026-02-30", "2026-11-16T15:42:13"} {
		if _, err := ParseRangeBoundary(value, "UTC", true); err == nil {
			t.Errorf("accepted invalid boundary %q", value)
		}
	}
}
