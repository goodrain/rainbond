package rbdcomponent

import (
	"testing"

	"github.com/spf13/pflag"
)

// capability_id: rainbond.image-clean.daily-schedule-flags
func TestChaosDailyCleanupFlags(t *testing.T) {
	cc := &ChaosConfig{}
	fs := pflag.NewFlagSet("chaos", pflag.ContinueOnError)
	AddChaosFlags(fs, cc)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	if cc.CleanTime != "03:00" || cc.CleanTimeZone != "Asia/Shanghai" {
		t.Fatalf("unexpected defaults time=%q zone=%q", cc.CleanTime, cc.CleanTimeZone)
	}
	legacy := fs.Lookup("clean-interval")
	if legacy == nil || legacy.Deprecated == "" {
		t.Fatal("legacy interval must remain registered and deprecated")
	}
	if err := fs.Set("clean-interval", "1"); err != nil {
		t.Fatal(err)
	}
	if cc.CleanInterval != 1 || cc.CleanTime != "03:00" || cc.CleanTimeZone != "Asia/Shanghai" {
		t.Fatal("legacy parsing changed the daily schedule")
	}
	if err := fs.Set("clean-time", "06:15"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("clean-timezone", "UTC"); err != nil {
		t.Fatal(err)
	}
	if cc.CleanTime != "06:15" || cc.CleanTimeZone != "UTC" {
		t.Fatal("daily overrides not retained")
	}
	if value, err := fs.GetString("clean-time"); err != nil || value != "06:15" {
		t.Fatalf("string flag contract: %q %v", value, err)
	}
}

// capability_id: rainbond.image-clean.daily-schedule-flag-validation
func TestChaosDailyCleanupFlagsRejectInvalidValues(t *testing.T) {
	tests := []struct{ flag, value string }{
		{"clean-time", ""}, {"clean-time", "3:00"}, {"clean-time", "24:00"},
		{"clean-time", "03:60"}, {"clean-time", "03:00:00"}, {"clean-time", " 03:00"},
		{"clean-timezone", ""}, {"clean-timezone", "Local"},
		{"clean-timezone", "Invalid/Zone"}, {"clean-timezone", " Asia/Shanghai"},
	}
	for _, tt := range tests {
		t.Run(tt.flag+"="+tt.value, func(t *testing.T) {
			cc := &ChaosConfig{}
			fs := pflag.NewFlagSet("chaos", pflag.ContinueOnError)
			AddChaosFlags(fs, cc)
			if err := fs.Set(tt.flag, tt.value); err == nil {
				t.Fatal("invalid daily schedule flag accepted")
			}
			if cc.CleanTime != "03:00" || cc.CleanTimeZone != "Asia/Shanghai" {
				t.Fatal("invalid flag changed prior configuration")
			}
		})
	}
}

// capability_id: rainbond.image-clean.safe-schedule-enable-flags
func TestChaosSafeScheduledCleanupEnableFlags(t *testing.T) {
	cc := &ChaosConfig{}
	fs := pflag.NewFlagSet("chaos", pflag.ContinueOnError)
	AddChaosFlags(fs, cc)
	if cc.CleanUp || !cc.ScheduledCleanUp {
		t.Fatalf("unsafe defaults: legacy=%v scheduled=%v", cc.CleanUp, cc.ScheduledCleanUp)
	}
	legacy := fs.Lookup("clean-up")
	if legacy == nil || legacy.Deprecated == "" {
		t.Fatal("legacy clean-up must be retained and deprecated")
	}
	if err := fs.Set("clean-up", "true"); err != nil {
		t.Fatal(err)
	}
	if !cc.CleanUp || !cc.ScheduledCleanUp {
		t.Fatal("legacy parsing changed safe scheduled switch")
	}
	if err := fs.Set("scheduled-clean-up", "false"); err != nil {
		t.Fatal(err)
	}
	if cc.ScheduledCleanUp {
		t.Fatal("safe scheduled cleanup cannot be disabled")
	}
	if err := fs.Set("scheduled-clean-up", "true"); err != nil {
		t.Fatal(err)
	}
	if !cc.ScheduledCleanUp {
		t.Fatal("safe scheduled cleanup override not retained")
	}
}
