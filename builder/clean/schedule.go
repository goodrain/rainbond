package clean

import (
	"context"
	"fmt"
	"time"
	_ "time/tzdata" // Keep IANA zones available in images without system tzdata.
)

type dailySchedule struct {
	hour     int
	minute   int
	location *time.Location
}

func newDailySchedule(clock, zone string) (dailySchedule, error) {
	if len(clock) != 5 || clock[2] != ':' ||
		clock[0] < '0' || clock[0] > '9' || clock[1] < '0' || clock[1] > '9' ||
		clock[3] < '0' || clock[3] > '9' || clock[4] < '0' || clock[4] > '9' {
		return dailySchedule{}, fmt.Errorf("invalid clean-time %q: expected HH:MM", clock)
	}
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		return dailySchedule{}, fmt.Errorf("invalid clean-time %q: %w", clock, err)
	}
	if zone == "" || zone == "Local" {
		return dailySchedule{}, fmt.Errorf("invalid clean-timezone %q: use an explicit IANA zone or UTC", zone)
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return dailySchedule{}, fmt.Errorf("invalid clean-timezone %q: %w", zone, err)
	}
	return dailySchedule{hour: parsed.Hour(), minute: parsed.Minute(), location: location}, nil
}

// next returns the next scheduled wall-clock instant strictly after now. A
// nonexistent wall time is skipped for that local date. If a clock is repeated
// during a backward transition, only its first occurrence is eligible that day.
func (s dailySchedule) next(now time.Time) time.Time {
	year, month, day := now.In(s.location).Date()
	for {
		candidate := time.Date(year, month, day, s.hour, s.minute, 0, 0, s.location)
		matches := func(value time.Time) bool {
			y, m, d := value.In(s.location).Date()
			return y == year && m == month && d == day &&
				value.Hour() == s.hour && value.Minute() == s.minute
		}
		if matches(candidate) {
			// Date may select either occurrence of an ambiguous local time.
			// Recover the earlier occurrence from the preceding zone offset.
			start, _ := candidate.ZoneBounds()
			if !start.IsZero() {
				_, currentOffset := candidate.Zone()
				_, previousOffset := start.Add(-time.Nanosecond).In(s.location).Zone()
				if previousOffset > currentOffset {
					earlier := candidate.Add(time.Duration(currentOffset-previousOffset) * time.Second)
					if matches(earlier) {
						candidate = earlier
					}
				}
			}
			if candidate.After(now) {
				return candidate
			}
		}
		// Advance the calendar date independently of 23/25-hour local days.
		year, month, day = time.Date(year, month, day+1, 0, 0, 0, 0, time.UTC).Date()
	}
}

// runDaily performs one startup run, then waits for successive wall-clock runs.
// The injected wait must return when its duration expires or the context ends.
func runDaily(ctx context.Context, schedule dailySchedule, now func() time.Time,
	wait func(context.Context, time.Duration) error, run func(context.Context, time.Time)) {
	if ctx.Err() != nil {
		return
	}
	run(ctx, now())
	for ctx.Err() == nil {
		current := now()
		scheduled := schedule.next(current)
		if err := wait(ctx, scheduled.Sub(current)); err != nil {
			return
		}
		// Cancellation can race with an expiring timer; never start another run
		// solely because a successful wait won that race.
		if ctx.Err() != nil {
			return
		}
		run(ctx, scheduled)
	}
}
