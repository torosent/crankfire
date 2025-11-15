package runner

import (
	"math"
	"time"
)

type patternPlan struct {
	segments []patternSegment
	duration time.Duration
	maxRate  float64
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
			plan.appendSegment(seg)
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
				plan.appendSegment(seg)
				offset += step.Duration
			}
		case LoadPatternTypeSpike:
			if pattern.Duration <= 0 {
				continue
			}
			seg := patternSegment{
				start:    offset,
				duration: pattern.Duration,
				fromRate: float64(pattern.RPS),
				toRate:   float64(pattern.RPS),
			}
			plan.appendSegment(seg)
			offset += pattern.Duration
		}
	}

	if len(plan.segments) == 0 {
		return nil
	}
	plan.duration = offset
	return plan
}

func (p *patternPlan) appendSegment(seg patternSegment) {
	p.segments = append(p.segments, seg)
	p.maxRate = math.Max(p.maxRate, math.Max(seg.fromRate, seg.toRate))
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
			return seg.fromRate, true
		}
		progress := float64(elapsed-seg.start) / float64(seg.duration)
		if progress < 0 {
			progress = 0
		} else if progress > 1 {
			progress = 1
		}
		return seg.fromRate + (seg.toRate-seg.fromRate)*progress, true
	}
	return 0, false
}

func (p *patternPlan) maxBurst() int {
	if p == nil {
		return 0
	}
	burst := int(math.Ceil(p.maxRate))
	if burst < 1 {
		burst = 1
	}
	return burst
}

func (p *patternPlan) totalDuration() time.Duration {
	if p == nil {
		return 0
	}
	return p.duration
}
