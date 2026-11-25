package clean

import (
	"context"
	"errors"
	"testing"
	"time"
)

// capability_id: rainbond.image-clean.daily-schedule
func TestDailyScheduleNextWallClock(t *testing.T) {
	tests := []struct{ name, clock, zone, now, want string }{
		{"default same day", "03:00", "Asia/Shanghai", "2026-11-24T18:00:00Z", "2026-11-24T19:00:00Z"},
		{"immediately before", "03:00", "Asia/Shanghai", "2026-11-24T18:59:59.999999999Z", "2026-11-24T19:00:00Z"},
		{"exact instant waits next day", "03:00", "Asia/Shanghai", "2026-11-24T19:00:00Z", "2026-11-25T19:00:00Z"},
		{"after instant", "03:00", "Asia/Shanghai", "2026-11-24T19:00:01Z", "2026-11-25T19:00:00Z"},
		{"month boundary", "03:00", "Asia/Shanghai", "2026-11-30T20:00:00Z", "2026-12-01T19:00:00Z"},
		{"year boundary", "03:00", "Asia/Shanghai", "2026-12-31T19:00:00Z", "2027-01-01T19:00:00Z"},
		{"UTC override", "06:15", "UTC", "2026-11-24T05:00:00Z", "2026-11-24T06:15:00Z"},
		{"DST spring uses calendar day", "03:00", "America/New_York", "2026-03-07T08:00:00Z", "2026-03-08T07:00:00Z"},
		{"DST fall uses calendar day", "03:00", "America/New_York", "2026-10-31T07:00:00Z", "2026-11-01T08:00:00Z"},
		{"nonexistent wall time skips day", "02:30", "America/New_York", "2026-03-08T06:00:00Z", "2026-03-09T06:30:00Z"},
		{"repeated wall time first occurrence", "01:30", "America/New_York", "2026-11-01T04:00:00Z", "2026-11-01T05:30:00Z"},
		{"repeated wall time never twice", "01:30", "America/New_York", "2026-11-01T05:45:00Z", "2026-11-02T06:30:00Z"},
		{"European repeated time chooses first", "02:30", "Europe/Berlin", "2026-10-25T00:00:00Z", "2026-10-25T00:30:00Z"},
		{"European repeated time never twice", "02:30", "Europe/Berlin", "2026-10-25T01:00:00Z", "2026-10-26T01:30:00Z"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule, err := newDailySchedule(tt.clock, tt.zone)
			if err != nil {
				t.Fatal(err)
			}
			now, err := time.Parse(time.RFC3339Nano, tt.now)
			if err != nil {
				t.Fatal(err)
			}
			want, err := time.Parse(time.RFC3339Nano, tt.want)
			if err != nil {
				t.Fatal(err)
			}
			got := schedule.next(now.In(time.FixedZone("caller", -7*3600)))
			if !got.Equal(want) || !got.After(now) {
				t.Fatalf("next(%s) = %s; want %s", now, got, want)
			}
			if got.Location().String() != tt.zone || got.Format("15:04") != tt.clock {
				t.Fatalf("next run lost configured wall time/location: %s", got)
			}
		})
	}
}

// capability_id: rainbond.image-clean.daily-schedule-validation
func TestDailyScheduleRejectsInvalidConfiguration(t *testing.T) {
	for _, clock := range []string{"", "3:00", "03:0", "24:00", "12:60", "03:00:00", " 03:00", "03:00 ", "-1:00", "0a:00"} {
		t.Run("time="+clock, func(t *testing.T) {
			if _, err := newDailySchedule(clock, "Asia/Shanghai"); err == nil {
				t.Fatal("invalid time accepted")
			}
		})
	}
	for _, zone := range []string{"", "Local", "Invalid/Zone", " Asia/Shanghai", "+08:00", "UTC+8"} {
		t.Run("zone="+zone, func(t *testing.T) {
			if _, err := newDailySchedule("03:00", zone); err == nil {
				t.Fatal("invalid zone accepted")
			}
		})
	}
}

// capability_id: rainbond.image-clean.daily-run-loop
func TestRunDailyStartupAndScheduledWallClock(t *testing.T) {
	schedule, err := newDailySchedule("03:00", "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	current := time.Date(2026, 11, 25, 2, 0, 0, 0, location)
	startup := current
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var runs []time.Time
	var waits []time.Duration
	runDaily(ctx, schedule, func() time.Time { return current },
		func(ctx context.Context, duration time.Duration) error {
			waits = append(waits, duration)
			current = current.Add(duration)
			return nil
		}, func(ctx context.Context, scheduled time.Time) {
			runs = append(runs, scheduled)
			if len(runs) == 3 {
				cancel()
			}
		})
	if len(runs) != 3 || !runs[0].Equal(startup) {
		t.Fatalf("startup must run exactly once: %v", runs)
	}
	for i, day := range []int{25, 26} {
		want := time.Date(2026, 11, day, 3, 0, 0, 0, location)
		if !runs[i+1].Equal(want) {
			t.Fatalf("run %d = %s; want %s", i+1, runs[i+1], want)
		}
	}
	if len(waits) != 2 || waits[0] != time.Hour || waits[1] != 24*time.Hour {
		t.Fatalf("daily waits = %v", waits)
	}
}

// capability_id: rainbond.image-clean.daily-run-cancellation
func TestRunDailyCancellationAndWaitFailure(t *testing.T) {
	schedule, err := newDailySchedule("03:00", "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{"cancelled before startup", "cancelled during startup", "cancelled during wait", "wait error"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "cancelled before startup" {
				cancel()
			}
			runs, waits := 0, 0
			runDaily(ctx, schedule, func() time.Time { return time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC) },
				func(ctx context.Context, duration time.Duration) error {
					waits++
					if name == "cancelled during wait" {
						cancel()
						return nil
					}
					return errors.New("wait unavailable")
				}, func(ctx context.Context, scheduled time.Time) {
					runs++
					if name == "cancelled during startup" {
						cancel()
					}
				})
			wantRuns, wantWaits := 1, 1
			if name == "cancelled before startup" {
				wantRuns, wantWaits = 0, 0
			}
			if name == "cancelled during startup" {
				wantWaits = 0
			}
			if runs != wantRuns || waits != wantWaits {
				t.Fatalf("runs/waits=%d/%d; want %d/%d", runs, waits, wantRuns, wantWaits)
			}
		})
	}
}
