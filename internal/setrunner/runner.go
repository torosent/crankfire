package setrunner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/torosent/crankfire/internal/config"
	"github.com/torosent/crankfire/internal/store"
)

// EventKind categorises live progress events emitted to optional channel.
type EventKind string

const (
	EventSetStarted   EventKind = "set_started"
	EventStageStarted EventKind = "stage_started"
	EventItemStarted  EventKind = "item_started"
	EventItemEnded    EventKind = "item_ended"
	EventStageEnded   EventKind = "stage_ended"
	EventSetEnded     EventKind = "set_ended"
)

type Event struct {
	Kind     EventKind
	Stage    string
	Item     string
	Result   *store.ItemResult
	Snapshot *MetricSnapshot
}

// ItemRun is what a Builder hands back: a Run thunk that executes the
// configured load test, a Snapshot accessor for live metrics, and a Cleanup.
type ItemRun struct {
	Run      func(ctx context.Context) (store.RunSummary, error)
	Snapshot func() MetricSnapshot
	Cleanup  func()
}

// Builder constructs a single item's runtime.
// tests inject fakes.
type Builder interface {
	Build(ctx context.Context, cfg config.Config, itemName string) (ItemRun, error)
}

type Runner struct {
	store   store.Store
	builder Builder
}

func New(s store.Store, b Builder) *Runner {
	return &Runner{store: s, builder: b}
}

func (r *Runner) Run(ctx context.Context, setID string, events chan<- Event) (store.SetRun, error) {
	set, err := r.store.GetSet(ctx, setID)
	if err != nil {
		return store.SetRun{}, fmt.Errorf("get set: %w", err)
	}
	run, err := r.store.CreateSetRun(ctx, setID)
	if err != nil {
		return store.SetRun{}, fmt.Errorf("create run: %w", err)
	}
	run.SetName = set.Name
	emit(events, Event{Kind: EventSetStarted})

	overallStatus := store.SetRunCompleted
	perItemSnaps := map[string]MetricSnapshot{}

stageLoop:
	for _, stage := range set.Stages {
		emit(events, Event{Kind: EventStageStarted, Stage: stage.Name})
		sr := store.StageResult{Name: stage.Name, StartedAt: time.Now().UTC()}
		results, anyFail := r.runStage(ctx, set, stage, run.Dir, events, perItemSnaps)
		sr.Items = results
		sr.EndedAt = time.Now().UTC()
		run.Stages = append(run.Stages, sr)
		emit(events, Event{Kind: EventStageEnded, Stage: stage.Name})

		if errors.Is(ctx.Err(), context.Canceled) {
			overallStatus = store.SetRunCancelled
			break stageLoop
		}
		if anyFail {
			onFail := stage.OnFailure
			if onFail == "" {
				onFail = store.OnFailureAbort
			}
			if onFail == store.OnFailureAbort {
				overallStatus = store.SetRunFailed
				break stageLoop
			}
		}
	}

	agg := aggregate(perItemSnaps)
	run.Thresholds = EvaluateThresholds(set.Thresholds, agg, perItemSnaps)
	run.AllThresholdsPassed = allPassed(run.Thresholds)
	if overallStatus == store.SetRunCompleted && !run.AllThresholdsPassed {
		overallStatus = store.SetRunFailed
	}
	run.Status = overallStatus
	run.EndedAt = time.Now().UTC()

	if err := r.store.FinalizeSetRun(ctx, run); err != nil {
		return run, fmt.Errorf("finalize: %w", err)
	}
	emit(events, Event{Kind: EventSetEnded})
	return run, nil
}

func (r *Runner) runStage(
	ctx context.Context,
	set store.Set,
	stage store.Stage,
	runDir string,
	events chan<- Event,
	perItem map[string]MetricSnapshot,
) ([]store.ItemResult, bool) {
	var (
		mu      sync.Mutex
		anyFail bool
		results = make([]store.ItemResult, len(stage.Items))
		wg      sync.WaitGroup
	)
	for i, item := range stage.Items {
		wg.Add(1)
		go func(idx int, item store.SetItem) {
			defer wg.Done()
			res := r.runItem(ctx, set, item, runDir, events)
			mu.Lock()
			results[idx] = res
			perItem[item.Name] = snapshotFromSummary(res.Summary)
			if res.Status != store.RunStatusCompleted {
				anyFail = true
			}
			mu.Unlock()
		}(i, item)
	}
	wg.Wait()
	return results, anyFail
}

func (r *Runner) runItem(ctx context.Context, set store.Set, item store.SetItem, runDir string, events chan<- Event) store.ItemResult {
	res := store.ItemResult{
		Name:      item.Name,
		SessionID: item.SessionID,
		StartedAt: time.Now().UTC(),
		Status:    store.RunStatusRunning,
	}
	emit(events, Event{Kind: EventItemStarted, Item: item.Name, Result: &res})

	sess, err := r.store.GetSession(ctx, item.SessionID)
	if err != nil {
		res.Status = store.RunStatusFailed
		res.Error = fmt.Sprintf("get session: %v", err)
		res.EndedAt = time.Now().UTC()
		emit(events, Event{Kind: EventItemEnded, Item: item.Name, Result: &res})
		return res
	}
	cfg := ApplyOverrides(sess.Config, item.Overrides)

	ir, err := r.builder.Build(ctx, cfg, item.Name)
	if err != nil {
		res.Status = store.RunStatusFailed
		res.Error = fmt.Sprintf("build: %v", err)
		res.EndedAt = time.Now().UTC()
		emit(events, Event{Kind: EventItemEnded, Item: item.Name, Result: &res})
		return res
	}
	defer ir.Cleanup()

	summary, err := ir.Run(ctx)
	res.EndedAt = time.Now().UTC()
	res.Summary = summary
	if err != nil {
		if errors.Is(err, context.Canceled) {
			res.Status = store.RunStatusCancelled
		} else {
			res.Status = store.RunStatusFailed
		}
		res.Error = err.Error()
	} else {
		res.Status = store.RunStatusCompleted
	}
	emit(events, Event{Kind: EventItemEnded, Item: item.Name, Result: &res})
	return res
}

func emit(ch chan<- Event, e Event) {
	if ch == nil {
		return
	}
	select {
	case ch <- e:
	default:
	}
}

func snapshotFromSummary(s store.RunSummary) MetricSnapshot {
	er := 0.0
	if s.TotalRequests > 0 {
		er = float64(s.Errors) / float64(s.TotalRequests)
	}
	rps := 0.0
	if s.DurationSec > 0 {
		rps = float64(s.TotalRequests) / s.DurationSec
	}
	return MetricSnapshot{
		P50:         s.P50Ms,
		P95:         s.P95Ms,
		P99:         s.P99Ms,
		ErrorRate:   er,
		RPS:         rps,
		TotalErrors: float64(s.Errors),
	}
}

func aggregate(per map[string]MetricSnapshot) MetricSnapshot {
	if len(per) == 0 {
		return MetricSnapshot{}
	}
	var agg MetricSnapshot
	for _, s := range per {
		if s.P50 > agg.P50 {
			agg.P50 = s.P50
		}
		if s.P95 > agg.P95 {
			agg.P95 = s.P95
		}
		if s.P99 > agg.P99 {
			agg.P99 = s.P99
		}
		if s.ErrorRate > agg.ErrorRate {
			agg.ErrorRate = s.ErrorRate
		}
		agg.RPS += s.RPS
		agg.TotalErrors += s.TotalErrors
	}
	return agg
}

func allPassed(rs []store.ThresholdResult) bool {
	for _, r := range rs {
		if !r.Passed {
			return false
		}
	}
	return true
}
