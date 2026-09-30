package service

import (
	"time"

	"fluxguard/protocol"
)

func normalizeSamples(in []protocol.Sample, now time.Time) []protocol.Sample {
	out := make([]protocol.Sample, len(in))
	for i, sample := range in {
		if sample.ObservedAt.IsZero() {
			sample.ObservedAt = now
		}
		sample.Labels = copyLabels(sample.Labels)
		out[i] = sample
	}
	return out
}

func normalizeEvents(in []protocol.Event, now time.Time) []protocol.Event {
	out := make([]protocol.Event, len(in))
	for i, event := range in {
		if event.ObservedAt.IsZero() {
			event.ObservedAt = now
		}
		if event.Severity == "" {
			event.Severity = "INFO"
		}
		event.Labels = copyLabels(event.Labels)
		out[i] = event
	}
	return out
}

func copyLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
