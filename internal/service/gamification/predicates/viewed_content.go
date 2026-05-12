package predicates

import (
	"context"
)

// ViewedContent tests whether the actor has visited a piece of content,
// optionally constrained by a minimum view count and/or cumulative time on
// page. The snapshot loader hydrates `ActorSnapshot.ViewedContent` from
// the `content_views` aggregate table (migration 000036), which the
// page-view emission middleware (Sprint D) increments on every render.
//
// MinViews defaults to 1 — the bare predicate "has viewed this page at
// least once." Higher values gate progression on repeated views (e.g.
// "review this lesson three times before unlocking the assessment").
// MinSecondsViewed gates on cumulative seconds across all views.
type ViewedContent struct {
	ContentID        uint `json:"content_id"`
	MinViews         int  `json:"min_views,omitempty"`         // default 1
	MinSecondsViewed int  `json:"min_seconds_viewed,omitempty"`
}

func (p ViewedContent) Kind() string { return "ViewedContent" }

func (p ViewedContent) Needs() Needs {
	return Needs{ContentIDs: []uint{p.ContentID}}
}

func (p ViewedContent) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	required := p.MinViews
	if required <= 0 {
		required = 1
	}

	trace := Trace{
		Kind: p.Kind(),
		Params: map[string]any{
			"content_id": p.ContentID,
			"min_views":  required,
		},
	}
	if p.MinSecondsViewed > 0 {
		trace.Params["min_seconds_viewed"] = p.MinSecondsViewed
	}

	state, ok := actor.ViewedContent[p.ContentID]
	if !ok {
		trace.Reason = "no recorded view for content"
		return false, trace
	}
	trace.Params["view_count"] = state.ViewCount
	trace.Params["total_seconds"] = state.TotalSeconds

	if state.ViewCount < required {
		trace.Reason = "view_count below MinViews"
		return false, trace
	}
	if p.MinSecondsViewed > 0 && state.TotalSeconds < int64(p.MinSecondsViewed) {
		trace.Reason = "total_seconds below MinSecondsViewed"
		return false, trace
	}

	trace.Result = true
	return true, trace
}
