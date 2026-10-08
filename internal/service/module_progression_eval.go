package service

import (
	"math"
	"sort"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ItemFacts is what the progression engine knows about one student's
// interaction with the content behind one module item. The service
// builds it from submissions, quiz submissions, discussion entries and
// module_item_user_actions; the evaluator only reads it.
type ItemFacts struct {
	Viewed      bool // opened the item (must_view)
	Done        bool // marked it done (must_mark_done)
	Submitted   bool // turned it in (must_submit)
	Contributed bool // posted to the discussion (must_contribute)
	Excused     bool // the scoreable work is excused: every requirement counts as met
	// Score is the best posted score usable for a min_score requirement
	// (nil when there is none yet).
	Score *float64
}

// ModuleEvalInput is everything EvaluateModuleProgress needs for one
// student in one course.
type ModuleEvalInput struct {
	Now time.Time
	// Modules are the published modules of the course, each with only
	// the items the student can see, both sorted by position.
	Modules []models.ContextModule
	// Prerequisites maps a module id to the ids of its prerequisite
	// modules. Ids that are not in Modules (deleted / unpublished
	// modules) are ignored, as Canvas does.
	Prerequisites map[uint][]uint
	// Facts is keyed by module item (content tag) id.
	Facts map[uint]ItemFacts
}

// ItemProgress is a student's state on one module item.
type ItemProgress struct {
	ItemID          uint     `json:"item_id"`
	Title           string   `json:"title"`
	RequirementType string   `json:"requirement_type,omitempty"`
	MinScore        *float64 `json:"min_score,omitempty"`
	// Completed: the item's requirement is met (false when it has none).
	Completed bool `json:"completed"`
	// Locked: the student may not open the item yet (its module is
	// locked, or sequential progress has not reached it).
	Locked bool `json:"locked"`
	// Score is the student's best score so far on a min_score item.
	Score *float64 `json:"score,omitempty"`
}

// ModuleProgress is a student's evaluated state on one module.
type ModuleProgress struct {
	ModuleID                  uint                          `json:"module_id"`
	Name                      string                        `json:"name"`
	State                     models.ModuleProgressionState `json:"state"`
	UnlockAt                  *time.Time                    `json:"unlock_at"`
	LockedUntilDate           bool                          `json:"locked_until_date"`
	PrerequisiteModuleIDs     []uint                        `json:"prerequisite_module_ids"`
	PrerequisitesUnmet        []uint                        `json:"prerequisites_unmet"`
	RequireSequentialProgress bool                          `json:"require_sequential_progress"`
	RequirementCount          int                           `json:"requirement_count"`
	// CurrentPosition is the position of the first item whose
	// requirement is still unmet in a sequential module (Canvas
	// current_position); items after it are locked. Nil when the module
	// is not sequential or every requirement is met.
	CurrentPosition *int           `json:"current_position"`
	RequirementsMet []uint         `json:"requirements_met"`
	Items           []ItemProgress `json:"items"`
}

// LockExplanation is a human-readable reason a module is locked, or ""
// when it is not.
func (m ModuleProgress) LockExplanation(moduleName func(uint) string) string {
	if m.State != models.ModuleProgressionLocked {
		return ""
	}
	if m.LockedUntilDate && m.UnlockAt != nil {
		return "This module will unlock on " + m.UnlockAt.UTC().Format("Jan 2, 2006 at 3:04 PM MST") + "."
	}
	if len(m.PrerequisitesUnmet) > 0 {
		names := ""
		for i, id := range m.PrerequisitesUnmet {
			if i > 0 {
				names += ", "
			}
			names += "\"" + moduleName(id) + "\""
		}
		return "Complete " + names + " to unlock this module."
	}
	return "This module is locked."
}

// RequirementMet reports whether a student with facts f meets the
// completion requirement on item (Canvas ContextModuleProgression
// requirement evaluation). An item without a requirement is never "met".
func RequirementMet(item models.ContentTag, f ItemFacts) bool {
	if item.CompletionType == "" {
		return false
	}
	if f.Excused && scoreableContentType(item.ContentType) {
		return true
	}
	switch item.CompletionType {
	case models.ModuleCompletionMustView:
		// Submitting or posting implies the student opened the item.
		return f.Viewed || f.Submitted || f.Contributed
	case models.ModuleCompletionMustMarkDone:
		return f.Done
	case models.ModuleCompletionMustContribute:
		return f.Contributed
	case models.ModuleCompletionMustSubmit:
		return f.Submitted
	case models.ModuleCompletionMinScore:
		if f.Score == nil || item.MinScore == nil {
			return false
		}
		return nearestScore(*f.Score) >= *item.MinScore
	}
	return false
}

// nearestScore rounds a score that is within floating-point noise of a
// whole number (Canvas near_enough?), so 6.9999999 meets a 7-point bar.
func nearestScore(s float64) float64 {
	if r := math.Round(s); math.Abs(s-r) < 1e-4 {
		return r
	}
	return s
}

func scoreableContentType(ct string) bool {
	switch ct {
	case "Assignment", "Quizzes::Quiz", "Quiz", "DiscussionTopic":
		return true
	}
	return false
}

// EvaluateModuleProgress computes a student's progression through every
// module of a course:
//
//   - A module is locked while its unlock_at is in the future or any of
//     its (published) prerequisite modules is not completed.
//   - An open module is completed when every completion requirement is
//     met — any one when requirement_count is 1, trivially when it has
//     none — started when at least one is met, unlocked otherwise.
//   - In a module that requires sequential progress, items after the
//     first item with an unmet requirement are locked.
func EvaluateModuleProgress(in ModuleEvalInput) []ModuleProgress {
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	byID := make(map[uint]*models.ContextModule, len(in.Modules))
	for i := range in.Modules {
		byID[in.Modules[i].ID] = &in.Modules[i]
	}

	results := make(map[uint]*ModuleProgress, len(in.Modules))
	visiting := make(map[uint]bool, len(in.Modules))

	var eval func(id uint) *ModuleProgress
	eval = func(id uint) *ModuleProgress {
		if r, ok := results[id]; ok {
			return r
		}
		mod := byID[id]
		if mod == nil {
			return nil
		}
		visiting[id] = true
		defer delete(visiting, id)

		mp := &ModuleProgress{
			ModuleID:                  mod.ID,
			Name:                      mod.Name,
			UnlockAt:                  mod.UnlockAt,
			RequireSequentialProgress: mod.RequireSequentialProgress,
			RequirementCount:          mod.RequirementCount,
			PrerequisiteModuleIDs:     []uint{},
			PrerequisitesUnmet:        []uint{},
			RequirementsMet:           []uint{},
		}

		locked := false
		if mod.UnlockAt != nil && mod.UnlockAt.After(now) {
			locked = true
			mp.LockedUntilDate = true
		}
		for _, pid := range in.Prerequisites[id] {
			if pid == id || byID[pid] == nil {
				continue
			}
			mp.PrerequisiteModuleIDs = append(mp.PrerequisiteModuleIDs, pid)
			// A cycle (A requires B requires A) can never be satisfied.
			if visiting[pid] {
				mp.PrerequisitesUnmet = append(mp.PrerequisitesUnmet, pid)
				locked = true
				continue
			}
			if pre := eval(pid); pre == nil || pre.State != models.ModuleProgressionCompleted {
				mp.PrerequisitesUnmet = append(mp.PrerequisitesUnmet, pid)
				locked = true
			}
		}

		items := append([]models.ContentTag(nil), mod.Items...)
		sort.SliceStable(items, func(a, b int) bool { return items[a].Position < items[b].Position })

		reqTotal, reqMet := 0, 0
		mp.Items = make([]ItemProgress, 0, len(items))
		for _, it := range items {
			ip := ItemProgress{ItemID: it.ID, Title: it.Title}
			if it.CompletionType != "" && it.ContentType != "ContextModuleSubHeader" {
				reqTotal++
				ip.RequirementType = it.CompletionType
				ip.MinScore = it.MinScore
				f := in.Facts[it.ID]
				if it.CompletionType == models.ModuleCompletionMinScore {
					ip.Score = f.Score
				}
				if !locked && RequirementMet(it, f) {
					ip.Completed = true
					reqMet++
					mp.RequirementsMet = append(mp.RequirementsMet, it.ID)
				}
			}
			mp.Items = append(mp.Items, ip)
		}

		switch {
		case locked:
			mp.State = models.ModuleProgressionLocked
		case reqTotal == 0 || reqMet == reqTotal || (mod.RequirementCount > 0 && reqMet >= mod.RequirementCount):
			mp.State = models.ModuleProgressionCompleted
		case reqMet > 0:
			mp.State = models.ModuleProgressionStarted
		default:
			mp.State = models.ModuleProgressionUnlocked
		}

		// Item locks.
		if locked {
			for i := range mp.Items {
				mp.Items[i].Locked = true
			}
		} else if mod.RequireSequentialProgress && mp.State != models.ModuleProgressionCompleted {
			gate := -1
			for i, ip := range mp.Items {
				if ip.RequirementType != "" && !ip.Completed {
					gate = i
					break
				}
			}
			if gate >= 0 {
				pos := items[gate].Position
				mp.CurrentPosition = &pos
				for i := gate + 1; i < len(mp.Items); i++ {
					mp.Items[i].Locked = true
				}
			}
		}

		results[id] = mp
		return mp
	}

	out := make([]ModuleProgress, 0, len(in.Modules))
	for _, m := range in.Modules {
		if r := eval(m.ID); r != nil {
			out = append(out, *r)
		}
	}
	return out
}
