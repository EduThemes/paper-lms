package service

// Blueprint sync of rubrics (with their assignment associations) and,
// on request, course settings (G069) — the parts of a course Canvas's
// master migrations carry beyond the content items in blueprint_sync.go.

import (
	"context"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// BlueprintSyncOptions are the per-sync choices (Canvas: "Include
// Course Settings").
type BlueprintSyncOptions struct {
	CopySettings bool
}

// SetRubricDeps wires rubric sync.
func (s *BlueprintService) SetRubricDeps(rubrics repository.RubricRepository, assocs repository.RubricAssociationRepository) {
	s.rubricRepo = rubrics
	s.rubricAssocRepo = assocs
}

func applyRubricFields(dst, src *models.Rubric) {
	dst.Title = src.Title
	dst.Description = src.Description
	dst.Data = src.Data
	dst.PointsPossible = src.PointsPossible
	dst.FreeFormCriterionComments = src.FreeFormCriterionComments
	dst.HideScoreTotal = src.HideScoreTotal
	dst.HidePoints = src.HidePoints
	dst.WorkflowState = src.WorkflowState
}

// syncRubrics copies / updates the blueprint's course rubrics.
func (s *BlueprintService) syncRubrics(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	if s.rubricRepo == nil || s.courseRepo == nil {
		return nil
	}
	// accountID=0: auth-internal — the blueprint's own course.
	course, err := s.courseRepo.FindByID(ctx, sourceCourseID, 0)
	if err != nil || course == nil {
		return nil
	}
	src, err := s.rubricRepo.ListByContext(ctx, "Course", sourceCourseID, course.AccountID, bigPage)
	if err != nil {
		return err
	}
	childRubrics, err := s.rubricRepo.ListByContext(ctx, "Course", sc.child, course.AccountID, bigPage)
	if err != nil {
		return err
	}
	byID := make(map[uint]*models.Rubric, len(childRubrics.Items))
	for i := range childRubrics.Items {
		byID[childRubrics.Items[i].ID] = &childRubrics.Items[i]
	}
	live := map[uint]bool{}
	for i := range src.Items {
		r := &src.Items[i]
		live[r.ID] = true
		tag := s.findTag(ctx, sc, "Rubric", r.ID)
		pushed := false
		var child *models.Rubric
		if tag != nil {
			child = byID[tag.ChildID]
		}
		switch {
		case child == nil:
			if tag != nil && !sc.sourceChanged(r.UpdatedAt) {
				continue
			}
			nr := &models.Rubric{ContextType: "Course", ContextID: sc.child}
			applyRubricFields(nr, r)
			if err := s.rubricRepo.Create(ctx, nr); err != nil {
				return fmt.Errorf("create rubric %q: %w", r.Title, err)
			}
			child = nr
			pushed = true
		case sc.shouldPush(r.UpdatedAt, tag, child.UpdatedAt, false):
			applyRubricFields(child, r)
			if err := s.rubricRepo.Update(ctx, child); err != nil {
				return fmt.Errorf("update rubric %q: %w", r.Title, err)
			}
			pushed = true
		}
		sc.idMap[contentKey("Rubric", r.ID)] = child.ID
		if err := s.noteSynced(ctx, sc, "Rubric", r.ID, child.ID, tag, pushed); err != nil {
			return err
		}
	}
	return s.syncDeletions(ctx, sc, "Rubric", live,
		func(id uint) (time.Time, bool) {
			if r, ok := byID[id]; ok {
				return r.UpdatedAt, true
			}
			return time.Time{}, false
		},
		// accountID=0: auth-internal — a copy this sync made.
		func(id uint) error { return s.rubricRepo.Delete(ctx, id, 0) })
}

// syncRubricAssociations attaches each assignment copy to the copy of
// its blueprint assignment's rubric (after assignments are synced).
func (s *BlueprintService) syncRubricAssociations(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	if s.rubricRepo == nil || s.rubricAssocRepo == nil {
		return nil
	}
	assignments, err := s.assignRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}
	for _, a := range assignments.Items {
		childAssignment, ok := sc.idMap[contentKey("Assignment", a.ID)]
		if !ok {
			continue
		}
		src, err := s.rubricAssocRepo.FindByAssociation(ctx, a.ID, "Assignment")
		if err != nil || src == nil {
			continue
		}
		childRubric, ok := sc.idMap[contentKey("Rubric", src.RubricID)]
		if !ok {
			continue
		}
		existing, err := s.rubricAssocRepo.FindByAssociation(ctx, childAssignment, "Assignment")
		if err == nil && existing != nil {
			if existing.RubricID == childRubric && existing.UseForGrading == src.UseForGrading {
				continue
			}
			existing.RubricID = childRubric
			existing.UseForGrading = src.UseForGrading
			existing.Purpose = src.Purpose
			if err := s.rubricAssocRepo.Update(ctx, existing); err != nil {
				return err
			}
			continue
		}
		na := &models.RubricAssociation{
			RubricID: childRubric, AssociationID: childAssignment, AssociationType: "Assignment",
			ContextType: "Course", ContextID: sc.child, Purpose: src.Purpose, UseForGrading: src.UseForGrading,
			HideScoreTotal: src.HideScoreTotal, HidePoints: src.HidePoints, HideOutcomeResults: src.HideOutcomeResults,
		}
		if na.Purpose == "" {
			na.Purpose = "grading"
		}
		if err := s.rubricAssocRepo.Create(ctx, na); err != nil {
			return err
		}
	}
	return nil
}

// syncSettings copies the blueprint's course settings to the child when
// the sync asked for them.
func (s *BlueprintService) syncSettings(ctx context.Context, sourceCourseID uint, sc *syncCtx) error {
	if !sc.copySettings || s.courseRepo == nil {
		return nil
	}
	// accountID=0: auth-internal — template course and an associated
	// course of the same account (B037).
	src, err := s.courseRepo.FindByID(ctx, sourceCourseID, 0)
	if err != nil || src == nil {
		return nil
	}
	child, err := s.courseRepo.FindByID(ctx, sc.child, 0)
	if err != nil || child == nil {
		return nil
	}
	if child.DefaultView == src.DefaultView && child.ApplyGroupWeights == src.ApplyGroupWeights &&
		child.License == src.License && child.NavigationTabs == src.NavigationTabs && child.UIMode == src.UIMode {
		return nil
	}
	// The syllabus' downstream-change check reads the course row's
	// updated_at: if the syllabus was in sync before this write, keep it
	// in sync after it.
	syllabusTag := s.findTag(ctx, sc, "Syllabus", sourceCourseID)
	restamp := syllabusTag != nil && !sc.downstreamChanged(syllabusTag, child.UpdatedAt)
	child.DefaultView = src.DefaultView
	child.ApplyGroupWeights = src.ApplyGroupWeights
	child.License = src.License
	child.NavigationTabs = src.NavigationTabs
	child.UIMode = src.UIMode
	if err := s.courseRepo.Update(ctx, child); err != nil {
		return err
	}
	if sc.results != nil {
		sc.results.Updated++
	}
	if restamp && s.tagRepo != nil {
		syllabusTag.SyncedAt = time.Now()
		return s.tagRepo.Upsert(ctx, syllabusTag)
	}
	return nil
}
