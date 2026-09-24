package mapper

import (
	"testing"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
)

func TestStoppedMapsSubsecondResultWithoutActivity(t *testing.T) {
	result := Stopped(application.StopTimerResult{Saved: false, AccumulatedSeconds: 47})

	if result.Running || result.Saved || !result.Subsecond || result.Activity != nil || result.Timer != nil || result.AccumulatedSeconds != 47 {
		t.Fatalf("subsecond stop mapping = %+v", result)
	}
}
