package predicates

import (
	"context"
)

// ViewedContent tests whether the actor has visited a piece of content. Wave 1
// only checks for first-view presence — the snapshot stores the first-viewed
// timestamp keyed by content_id. A future MinSecondsViewed gate will land
// once the content-view loader tracks cumulative duration (Sprint C).
type ViewedContent struct {
	ContentID uint
	// MinSecondsViewed is parsed into the struct so rules can be authored
	// against it today, but Wave 1 silently treats it as 0 — see the Reason
	// trace when non-zero. Tracked under the same TODO as the snapshot
	// duration extension.
	MinSecondsViewed int
}

func (p ViewedContent) Kind() string { return "ViewedContent" }

func (p ViewedContent) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	trace := Trace{
		Kind: p.Kind(),
		Params: map[string]any{
			"content_id": p.ContentID,
		},
	}
	if p.MinSecondsViewed > 0 {
		trace.Params["min_seconds_viewed"] = p.MinSecondsViewed
	}

	_, ok := actor.ViewedContent[p.ContentID]
	if !ok {
		trace.Reason = "no recorded view for content"
		return false, trace
	}

	if p.MinSecondsViewed > 0 {
		// Duration-of-view is not yet tracked in the snapshot; presence-only
		// for Wave 1. Note this in the trace so debug output is honest.
		trace.Reason = "duration-of-view not yet tracked; presence-only match"
	}
	trace.Result = true
	return true, trace
}
