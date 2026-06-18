package xrayguard

import "time"

type Detector struct {
	opts    Options
	history []Sample
}

func NewDetector(opts Options) *Detector {
	opts = normalizeOptions(opts)
	return &Detector{opts: opts}
}

func (d *Detector) Observe(pid int, sample Sample) (Event, bool) {
	if sample.Timestamp.IsZero() {
		sample.Timestamp = time.Now().UTC()
	}
	d.history = append(d.history, sample)
	d.trim(sample.Timestamp)

	for _, before := range d.history {
		if before.Timestamp.Equal(sample.Timestamp) {
			continue
		}
		if event, ok := d.detect(pid, before, sample); ok {
			return event, true
		}
	}
	return Event{}, false
}

func (d *Detector) detect(pid int, before, after Sample) (Event, bool) {
	fdDelta := after.FDCount - before.FDCount
	if fdDelta < d.opts.MinFDDelta || after.FDCount < d.opts.MinFDCount {
		return Event{}, false
	}
	if after.FDCount <= 0 || after.SocketFDCount <= 0 {
		return Event{}, false
	}
	socketRatio := float64(after.SocketFDCount) / float64(after.FDCount)
	if socketRatio < d.opts.MinSocketRatio {
		return Event{}, false
	}
	if after.EstablishedTCPCount > d.opts.MaxEstablishedTCP && after.EstablishedTCPCount > after.FDCount/4 {
		return Event{}, false
	}

	return Event{
		Reason:              ReasonFDSpike,
		PID:                 pid,
		Before:              before,
		After:               after,
		Window:              after.Timestamp.Sub(before.Timestamp),
		FDDelta:             fdDelta,
		SocketRatioPercent:  int(socketRatio*100 + 0.5),
		EstablishedTCPCount: after.EstablishedTCPCount,
		Action:              d.opts.Action,
	}, true
}

func (d *Detector) trim(now time.Time) {
	cutoff := now.Add(-d.opts.Window)
	keep := 0
	for _, sample := range d.history {
		if !sample.Timestamp.Before(cutoff) {
			d.history[keep] = sample
			keep++
		}
	}
	d.history = d.history[:keep]
}

func normalizeOptions(opts Options) Options {
	defaults := DefaultOptions()
	if opts.Interval <= 0 {
		opts.Interval = defaults.Interval
	}
	if opts.Window <= 0 {
		opts.Window = defaults.Window
	}
	if opts.MinFDDelta <= 0 {
		opts.MinFDDelta = defaults.MinFDDelta
	}
	if opts.MinFDCount <= 0 {
		opts.MinFDCount = defaults.MinFDCount
	}
	if opts.MinSocketRatio <= 0 {
		opts.MinSocketRatio = defaults.MinSocketRatio
	}
	if opts.MaxEstablishedTCP <= 0 {
		opts.MaxEstablishedTCP = defaults.MaxEstablishedTCP
	}
	if opts.Action == "" {
		opts.Action = defaults.Action
	}
	if opts.Collector == nil {
		opts.Collector = defaults.Collector
	}
	return opts
}
