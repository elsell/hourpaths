package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type intervalServiceRepository struct {
	testRepository
	goal               pathdomain.IntervalGoal
	goalErr            error
	goalReads          *[]struct{ participantID, pathID string }
	currentResult      CurrentTimerResult
	currentErr         error
	currentProjections *[]*IntervalProgressRequest
	startsWithProgress *[]StartTimerCommand
}

type mutationIntervalRepository struct {
	intervalServiceRepository
	echoProgress   bool
	resultProgress *IntervalProgress
	stops          *[]StopTimerCommand
	creates        *[]CreateManualActivityCommand
	updates        *[]UpdateActivityCommand
	deletes        *[]DeleteActivityCommand
}

func (f mutationIntervalRepository) progress(request *IntervalProgressRequest) *IntervalProgress {
	if f.echoProgress {
		return progressForRequest(request, 75)
	}
	return f.resultProgress
}

func (f mutationIntervalRepository) StopTimer(_ context.Context, command StopTimerCommand) (StopTimerResult, error) {
	if f.stops != nil {
		*f.stops = append(*f.stops, command)
	}
	timer, err := domain.StartTimer("timer-1", command.PathID, command.ParticipantID, command.StoppedAt.Add(-time.Minute), "Etc/UTC", command.StoppedAt)
	if err != nil {
		return StopTimerResult{}, err
	}
	entry, saved, err := timer.Stop(command.ActivityID, command.StoppedAt, command.RecordedAt)
	return StopTimerResult{Activity: entry, Saved: saved, AccumulatedSeconds: 120, IntervalProgress: f.progress(command.IntervalProgress)}, err
}

func (f mutationIntervalRepository) CreateManualActivity(_ context.Context, command CreateManualActivityCommand) (CreateManualActivityResult, error) {
	if f.creates != nil {
		*f.creates = append(*f.creates, command)
	}
	return CreateManualActivityResult{Activity: command.Activity, Version: 1, AccumulatedSeconds: 120, IntervalProgress: f.progress(command.IntervalProgress)}, nil
}

func (f mutationIntervalRepository) UpdateActivity(_ context.Context, command UpdateActivityCommand) (UpdateActivityResult, error) {
	if f.updates != nil {
		*f.updates = append(*f.updates, command)
	}
	prior := f.activity
	edited, revision, err := prior.EditByOwner(command.ParticipantID, command.Edit, command.UpdatedAt)
	return UpdateActivityResult{Activity: edited, Revision: revision, Version: 2, AccumulatedSeconds: 120, IntervalProgress: f.progress(command.IntervalProgress)}, err
}

func (f mutationIntervalRepository) DeleteActivity(_ context.Context, command DeleteActivityCommand) (DeleteActivityResult, error) {
	if f.deletes != nil {
		*f.deletes = append(*f.deletes, command)
	}
	return DeleteActivityResult{AccumulatedSeconds: 120, SessionCount: 1, RemovedFeedEventIDs: []string{"practice:" + command.ActivityID}, IntervalProgress: f.progress(command.IntervalProgress)}, nil
}

func (f testRepository) IntervalGoal(context.Context, string, string) (pathdomain.IntervalGoal, error) {
	return pathdomain.IntervalGoal{}, nil
}

func (f testRepository) CurrentProjection(_ context.Context, _, _ string, _ *IntervalProgressRequest) (CurrentTimerResult, error) {
	if f.getErr != nil && !errors.Is(f.getErr, ports.ErrNotFound) {
		return CurrentTimerResult{}, f.getErr
	}
	if f.totalErr != nil {
		return CurrentTimerResult{}, f.totalErr
	}
	result := CurrentTimerResult{AccumulatedSeconds: f.total}
	if !errors.Is(f.getErr, ports.ErrNotFound) && f.timer.ID != "" {
		result.Timer = &f.timer
	}
	return result, nil
}

func (f intervalServiceRepository) IntervalGoal(_ context.Context, participantID, pathID string) (pathdomain.IntervalGoal, error) {
	if f.goalReads != nil {
		*f.goalReads = append(*f.goalReads, struct{ participantID, pathID string }{participantID, pathID})
	}
	return f.goal, f.goalErr
}

func (f intervalServiceRepository) CurrentProjection(_ context.Context, _, _ string, request *IntervalProgressRequest) (CurrentTimerResult, error) {
	if f.currentProjections != nil {
		*f.currentProjections = append(*f.currentProjections, request)
	}
	return f.currentResult, f.currentErr
}

func (f intervalServiceRepository) StartTimer(_ context.Context, command StartTimerCommand) (StartTimerResult, error) {
	if f.startsWithProgress != nil {
		*f.startsWithProgress = append(*f.startsWithProgress, command)
	}
	return StartTimerResult{Timer: command.Timer, IntervalProgress: progressForRequest(command.IntervalProgress, 75)}, nil
}

func progressForRequest(request *IntervalProgressRequest, accumulatedSeconds int64) *IntervalProgress {
	if request == nil {
		return nil
	}
	return &IntervalProgress{TargetSeconds: request.TargetSeconds, AccumulatedSeconds: accumulatedSeconds, Window: request.Window}
}

func TestCurrentTimerProjectsPresentIntervalGoalInParticipantCurrentTimeZone(t *testing.T) {
	var goalReads []struct{ participantID, pathID string }
	var projections []*IntervalProgressRequest
	goal := pathdomain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: pathdomain.RecurrenceDaily, Alignment: pathdomain.GoalAlignment{Hour: 0}}
	wantWindow := IntervalWindow{
		StartedAt: time.Date(2026, time.July, 21, 4, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC),
	}
	repository := intervalServiceRepository{
		goal: goal, goalReads: &goalReads, currentProjections: &projections,
		currentResult: CurrentTimerResult{AccumulatedSeconds: 900, IntervalProgress: &IntervalProgress{TargetSeconds: 60, AccumulatedSeconds: 75, Window: wantWindow}},
	}
	service := testService(repository)

	result, err := service.CurrentTimer(context.Background(), "Bearer valid", "path-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(goalReads) != 1 || goalReads[0].participantID != "user-1" || goalReads[0].pathID != "path-1" {
		t.Fatalf("interval goal reads = %+v", goalReads)
	}
	if len(projections) != 1 || projections[0] == nil || projections[0].TargetSeconds != 60 || projections[0].Window != wantWindow {
		t.Fatalf("current projection requests = %+v", projections)
	}
	if result.IntervalProgress == nil || result.IntervalProgress.AccumulatedSeconds != 75 || result.IntervalProgress.TargetSeconds != 60 {
		t.Fatalf("interval progress = %+v", result.IntervalProgress)
	}
}

func TestCurrentTimerOmitsAbsentIntervalGoalWithoutReadingProfile(t *testing.T) {
	var projections []*IntervalProgressRequest
	repository := intervalServiceRepository{currentProjections: &projections, currentResult: CurrentTimerResult{AccumulatedSeconds: 45}}
	service := testService(repository)
	service.Profiles = nil

	result, err := service.CurrentTimer(context.Background(), "Bearer valid", "path-1")
	if err != nil || result.IntervalProgress != nil {
		t.Fatalf("CurrentTimer() = (%+v, %v), want omitted interval progress", result, err)
	}
	if len(projections) != 1 || projections[0] != nil {
		t.Fatalf("current projection requests = %+v, want one nil request", projections)
	}
}

func TestCurrentTimerFailsClosedForMismatchedIntervalProjection(t *testing.T) {
	goal := pathdomain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: pathdomain.RecurrenceHourly, Alignment: pathdomain.GoalAlignment{Minute: 0}}
	bad := &IntervalProgress{TargetSeconds: 61, AccumulatedSeconds: 1, Window: IntervalWindow{StartedAt: testNow.Add(-time.Hour), EndedAt: testNow}}
	service := testService(intervalServiceRepository{goal: goal, currentResult: CurrentTimerResult{IntervalProgress: bad}})

	if _, err := service.CurrentTimer(context.Background(), "Bearer valid", "path-1"); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("CurrentTimer() error = %v, want invalid dependencies", err)
	}
}

func TestStartTimerCarriesCurrentIntervalProjectionOutsideIdempotencyIdentity(t *testing.T) {
	var first, second []StartTimerCommand
	goal := pathdomain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: pathdomain.RecurrenceHourly, Alignment: pathdomain.GoalAlignment{Minute: 0}}
	serviceA := testService(intervalServiceRepository{goal: goal, startsWithProgress: &first})
	serviceB := testService(intervalServiceRepository{goal: goal, startsWithProgress: &second})
	serviceB.Clock = testClock{now: testNow.Add(30 * time.Minute)}

	result, err := serviceA.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	if err != nil || result.IntervalProgress == nil || result.IntervalProgress.AccumulatedSeconds != 75 {
		t.Fatalf("StartTimer() = (%+v, %v)", result, err)
	}
	_, _ = serviceB.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	if len(first) != 1 || first[0].IntervalProgress == nil || len(second) != 1 || second[0].IntervalProgress == nil {
		t.Fatalf("start commands = %+v / %+v", first, second)
	}
	if !equalBytes(first[0].Idempotency.RequestHash, second[0].Idempotency.RequestHash) {
		t.Fatal("current projection changed mutation idempotency identity")
	}
}

func TestActivityMutationsCarryAndReturnCurrentIntervalProjection(t *testing.T) {
	goal := pathdomain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: pathdomain.RecurrenceDaily, Alignment: pathdomain.GoalAlignment{Hour: 0}}
	want := &IntervalProgressRequest{TargetSeconds: 60, Window: IntervalWindow{
		StartedAt: time.Date(2026, time.July, 21, 4, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC),
	}}
	original := intervalTestActivity(t, "Europe/Paris")

	t.Run("stop", func(t *testing.T) {
		var commands []StopTimerCommand
		service := testService(mutationIntervalRepository{
			intervalServiceRepository: intervalServiceRepository{goal: goal}, echoProgress: true, stops: &commands,
		})
		result, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-0001")
		if err != nil || len(commands) != 1 || !equalProgressRequest(commands[0].IntervalProgress, want) || result.IntervalProgress == nil || result.IntervalProgress.AccumulatedSeconds != 75 {
			t.Fatalf("StopTimer() = (%+v, %v), commands=%+v", result, err, commands)
		}
	})

	t.Run("create", func(t *testing.T) {
		var commands []CreateManualActivityCommand
		service := testService(mutationIntervalRepository{
			intervalServiceRepository: intervalServiceRepository{goal: goal}, echoProgress: true, creates: &commands,
		})
		result, err := service.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", intervalManualInput())
		if err != nil || len(commands) != 1 || !equalProgressRequest(commands[0].IntervalProgress, want) || result.IntervalProgress == nil || result.IntervalProgress.AccumulatedSeconds != 75 {
			t.Fatalf("CreateManualActivity() = (%+v, %v), commands=%+v", result, err, commands)
		}
	})

	t.Run("update uses current profile zone", func(t *testing.T) {
		var commands []UpdateActivityCommand
		var profileReads []string
		service := testService(mutationIntervalRepository{
			intervalServiceRepository: intervalServiceRepository{testRepository: testRepository{activity: original}, goal: goal},
			echoProgress:              true, updates: &commands,
		})
		service.Profiles = testProfile{zone: "America/New_York", users: &profileReads}
		result, err := service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", intervalManualInput())
		if err != nil || len(commands) != 1 || !equalProgressRequest(commands[0].IntervalProgress, want) || result.IntervalProgress == nil || result.IntervalProgress.AccumulatedSeconds != 75 {
			t.Fatalf("UpdateActivity() = (%+v, %v), commands=%+v", result, err, commands)
		}
		if len(profileReads) != 1 || profileReads[0] != "user-1" || commands[0].Edit.OccurrenceTimeZone != "Europe/Paris" {
			t.Fatalf("profile reads=%v edit=%+v", profileReads, commands[0].Edit)
		}
	})

	t.Run("delete", func(t *testing.T) {
		var commands []DeleteActivityCommand
		service := testService(mutationIntervalRepository{
			intervalServiceRepository: intervalServiceRepository{goal: goal}, echoProgress: true, deletes: &commands,
		})
		result, err := service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-001")
		if err != nil || len(commands) != 1 || !equalProgressRequest(commands[0].IntervalProgress, want) || result.IntervalProgress == nil || result.IntervalProgress.AccumulatedSeconds != 75 {
			t.Fatalf("DeleteActivity() = (%+v, %v), commands=%+v", result, err, commands)
		}
	})
}

func TestGoalIndependentMutationsOmitIntervalProjectionWithoutProfileRead(t *testing.T) {
	original := intervalTestActivity(t, "Europe/Paris")
	tests := []struct {
		name string
		run  func(*Service, *mutationIntervalRepository) error
	}{
		{
			name: "stop",
			run: func(service *Service, repository *mutationIntervalRepository) error {
				var commands []StopTimerCommand
				repository.stops = &commands
				service.Repository = *repository
				_, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-0001")
				if err == nil && (len(commands) != 1 || commands[0].IntervalProgress != nil) {
					t.Fatalf("stop commands=%+v", commands)
				}
				return err
			},
		},
		{
			name: "update",
			run: func(service *Service, repository *mutationIntervalRepository) error {
				var commands []UpdateActivityCommand
				repository.updates = &commands
				service.Repository = *repository
				_, err := service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", intervalManualInput())
				if err == nil && (len(commands) != 1 || commands[0].IntervalProgress != nil) {
					t.Fatalf("update commands=%+v", commands)
				}
				return err
			},
		},
		{
			name: "delete",
			run: func(service *Service, repository *mutationIntervalRepository) error {
				var commands []DeleteActivityCommand
				repository.deletes = &commands
				service.Repository = *repository
				_, err := service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-001")
				if err == nil && (len(commands) != 1 || commands[0].IntervalProgress != nil) {
					t.Fatalf("delete commands=%+v", commands)
				}
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var profileReads []string
			repository := mutationIntervalRepository{
				intervalServiceRepository: intervalServiceRepository{testRepository: testRepository{activity: original}}, echoProgress: true,
			}
			service := testService(repository)
			service.Profiles = testProfile{zone: "America/New_York", users: &profileReads}
			if err := test.run(service, &repository); err != nil {
				t.Fatal(err)
			}
			if len(profileReads) != 0 {
				t.Fatalf("profile reads=%v, want none without interval goal", profileReads)
			}
		})
	}
}

func TestActivityMutationsFailClosedForInvalidIntervalProjection(t *testing.T) {
	presentGoal := pathdomain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: pathdomain.RecurrenceDaily, Alignment: pathdomain.GoalAlignment{Hour: 0}}
	window := IntervalWindow{StartedAt: time.Date(2026, time.July, 21, 4, 0, 0, 0, time.UTC), EndedAt: time.Date(2026, time.July, 22, 4, 0, 0, 0, time.UTC)}
	wrong := &IntervalProgress{TargetSeconds: 61, AccumulatedSeconds: 1, Window: window}
	spurious := &IntervalProgress{TargetSeconds: 60, AccumulatedSeconds: 1, Window: window}
	original := intervalTestActivity(t, "Europe/Paris")

	tests := []struct {
		name     string
		goal     pathdomain.IntervalGoal
		progress *IntervalProgress
		run      func(*Service) error
	}{
		{name: "stop missing", goal: presentGoal, run: func(service *Service) error {
			_, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-0001")
			return err
		}},
		{name: "create mismatched", goal: presentGoal, progress: wrong, run: func(service *Service) error {
			_, err := service.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", intervalManualInput())
			return err
		}},
		{name: "update negative", goal: presentGoal, progress: &IntervalProgress{TargetSeconds: 60, AccumulatedSeconds: -1, Window: window}, run: func(service *Service) error {
			_, err := service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", intervalManualInput())
			return err
		}},
		{name: "delete spurious", progress: spurious, run: func(service *Service) error {
			_, err := service.DeleteActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "delete-request-001")
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := mutationIntervalRepository{
				intervalServiceRepository: intervalServiceRepository{testRepository: testRepository{activity: original}, goal: test.goal},
				resultProgress:            test.progress,
			}
			service := testService(repository)
			if err := test.run(service); !errors.Is(err, errInvalidDependencies) {
				t.Fatalf("mutation error=%v, want invalid dependencies", err)
			}
		})
	}
}

func intervalTestActivity(t *testing.T, occurrenceTimeZone string) domain.RecordedActivity {
	t.Helper()
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-1",
		StartedAt: testNow.Add(-2 * time.Hour), DurationSeconds: 60, OccurrenceTimeZone: occurrenceTimeZone,
	}, testNow.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func intervalManualInput() ManualActivityInput {
	return ManualActivityInput{LocalDate: "2026-07-21", LocalStartTime: "13:00:00", DurationSeconds: 60}
}

func equalProgressRequest(got, want *IntervalProgressRequest) bool {
	return got != nil && want != nil && *got == *want
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
