package social

import (
	"testing"
	"time"
)

func TestGoalAchievementRequiresCanonicalKindTargetAndIntervalBounds(t *testing.T) {
	published := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	start, end := published.Add(-24*time.Hour), published
	interval, err := NewGoalAchievement("earned-1", "participant-1", "path-1", AchievementInterval, 3600, start, end, published)
	if err != nil || interval.Kind != AchievementInterval || interval.TargetSeconds != 3600 || interval.IntervalStartedAt != start || interval.IntervalEndedAt != end {
		t.Fatalf("interval achievement=%+v err=%v", interval, err)
	}
	overall, err := NewGoalAchievement("earned-2", "participant-1", "path-1", AchievementOverall, 10000, time.Time{}, time.Time{}, published)
	if err != nil || overall.Kind != AchievementOverall || !overall.IntervalStartedAt.IsZero() || !overall.IntervalEndedAt.IsZero() {
		t.Fatalf("overall achievement=%+v err=%v", overall, err)
	}

	for name, mutate := range map[string]func(*GoalAchievement){
		"missing id":        func(value *GoalAchievement) { value.ID = "" },
		"unknown kind":      func(value *GoalAchievement) { value.Kind = "streak" },
		"missing target":    func(value *GoalAchievement) { value.TargetSeconds = 0 },
		"reversed interval": func(value *GoalAchievement) { value.IntervalEndedAt = value.IntervalStartedAt },
		"non UTC":           func(value *GoalAchievement) { value.PublishedAt = value.PublishedAt.In(time.FixedZone("offset", 3600)) },
	} {
		t.Run(name, func(t *testing.T) {
			value := interval
			mutate(&value)
			if value.Valid() {
				t.Fatalf("invalid achievement accepted: %+v", value)
			}
		})
	}
}

func TestGoalAchievementSupportUsesStoredTargetIdentity(t *testing.T) {
	value, err := NewGoalAchievement("earned-1", "participant-1", "path-1", AchievementOverall, 60, time.Time{}, time.Time{}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if value.SupportedBy(59) || !value.SupportedBy(60) || !value.SupportedBy(120) {
		t.Fatal("achievement support did not use its stored target")
	}
}
