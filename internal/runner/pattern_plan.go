package runner

import (
	"math"
	"time"
)

// shortcut: a pattern rate below 1 rps runs at 1 rps (0 used to mean "no limit"
// and flooded the target); a real pause needs the scheduler to skip waits.
const minPatternRate = 1.0

type patternPlan struct {
	segments []patternSegment
}

type patternSegment struct {
	start    time.Duration
	duration time.Duration
	fromRate float64
	toRate   float64
}

func compilePatternPlan(patterns []LoadPattern) *patternPlan {
	if len(patterns) == 0 {
		return nil
	}

	plan := &patternPlan{}
	var offset time.Duration
	for _, pattern := range patterns {
		switch pattern.Type {
		case LoadPatternTypeRamp:
			if pattern.Duration <= 0 {
				continue
			}
			seg := patternSegment{
				start:    offset,
				duration: pattern.Duration,
				fromRate: float64(pattern.FromRPS),
				toRate:   float64(pattern.ToRPS),
			}
			plan.segments = append(plan.segments, seg)
			offset += pattern.Duration
		case LoadPatternTypeStep:
			for _, step := range pattern.Steps {
				if step.Duration <= 0 {
					continue
				}
				seg := patternSegment{
					start:    offset,
					duration: step.Duration,
					fromRate: float64(step.RPS),
					toRate:   float64(step.RPS),
				}
				plan.segments = append(plan.segments, seg)
				offset += step.Duration
			}
		case LoadPatternTypeSpike, LoadPatternTypeConstant:
			if pattern.Duration <= 0 {
				continue
			}
			seg := patternSegment{
				start:    offset,
				duration: pattern.Duration,
				fromRate: float64(pattern.RPS),
				toRate:   float64(pattern.RPS),
			}
			plan.segments = append(plan.segments, seg)
			offset += pattern.Duration
		}
	}

	if len(plan.segments) == 0 {
		return nil
	}
	return plan
}

func (p *patternPlan) rateAt(elapsed time.Duration) (float64, bool) {
	if p == nil || len(p.segments) == 0 {
		return 0, false
	}
	if elapsed < 0 {
		elapsed = 0
	}
	for _, seg := range p.segments {
		if elapsed < seg.start {
			continue
		}
		end := seg.start + seg.duration
		if elapsed >= end {
			continue
		}
		if seg.duration <= 0 {
			continue
		}
		if seg.fromRate == seg.toRate {
			return math.Max(seg.fromRate, minPatternRate), true
		}
		progress := float64(elapsed-seg.start) / float64(seg.duration)
		if progress < 0 {
			progress = 0
		} else if progress > 1 {
			progress = 1
		}
		return math.Max(seg.fromRate+(seg.toRate-seg.fromRate)*progress, minPatternRate), true
	}
	return 0, false
}
