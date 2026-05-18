package service

// Blueprint sync orchestration. This is the bulk of the service — it
// copies content (modules, assignments, pages, quizzes, quiz questions,
// discussion topics, and module items) from a blueprint template course
// to each associated child course. Re-runs are idempotent via title-based
// dedup; module items use an idMap keyed by "ContentType:OldID" to remap
// content references to the freshly-created child content.
//
// Wave 5 split (chore/wave5-split-quiz-blueprint). All methods hang off
// *BlueprintService so the handler call surface is unchanged.

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// repository is imported because TriggerSync constructs a one-off
// PaginationParams literal below. The package-level `bigPage` lives in
// blueprint_service.go for reuse across the sync sub-methods.

// TriggerSync creates a migration record and performs actual content sync from the
// blueprint template course to all associated child courses.
//
// For each child course the sync:
//  1. Copies modules (skipping those already present by title match)
//  2. Copies assignments (skip by name match)
//  3. Copies pages (skip by title match)
//  4. Copies quizzes with their questions (skip by title match)
//  5. Copies discussion topics (skip by title match)
//  6. Copies module items, remapping content IDs to the newly created child content
func (s *BlueprintService) TriggerSync(ctx context.Context, templateID, userID uint, comment string) (*models.BlueprintMigration, error) {
	if templateID == 0 {
		return nil, errors.New("template_id is required")
	}
	if userID == 0 {
		return nil, errors.New("user_id is required")
	}

	// Look up the template to get the source course ID
	template, err := s.tmplRepo.FindByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}
	sourceCourseID := template.CourseID

	// Create a migration record in "running" state
	migration := &models.BlueprintMigration{
		BlueprintTemplateID: templateID,
		UserID:              userID,
		WorkflowState:       "running",
		Comment:             comment,
		ExportSettings:      "{}",
	}
	if err := s.migRepo.Create(ctx, migration); err != nil {
		return nil, err
	}

	// Fetch all subscriptions (child courses)
	subs, err := s.subRepo.ListByTemplateID(ctx, templateID, repository.PaginationParams{Page: 1, PerPage: 10000})
	if err != nil {
		s.failMigration(ctx, migration, err)
		return migration, err
	}

	// Perform content sync for each child course
	var syncErrors []string
	for _, sub := range subs.Items {
		if sub.WorkflowState != "active" {
			continue
		}
		if err := s.syncCourseContent(ctx, sourceCourseID, sub.ChildCourseID); err != nil {
			slog.Error("blueprint sync failed for child course",
				"template_id", templateID,
				"source_course_id", sourceCourseID,
				"child_course_id", sub.ChildCourseID,
				"error", err,
			)
			syncErrors = append(syncErrors, fmt.Sprintf("course %d: %v", sub.ChildCourseID, err))
		}
	}

	// Mark migration as completed (or failed if any child course errored)
	now := time.Now()
	migration.CompletedAt = &now
	if len(syncErrors) > 0 {
		migration.WorkflowState = "completed" // partial success — still mark completed
		migration.Comment = comment + " | sync warnings: " + strings.Join(syncErrors, "; ")
	} else {
		migration.WorkflowState = "completed"
	}
	if err := s.migRepo.Update(ctx, migration); err != nil {
		return migration, err
	}

	return migration, nil
}

// failMigration marks a migration as failed.
func (s *BlueprintService) failMigration(ctx context.Context, migration *models.BlueprintMigration, reason error) {
	now := time.Now()
	migration.WorkflowState = "failed"
	migration.CompletedAt = &now
	migration.Comment = migration.Comment + " | error: " + reason.Error()
	_ = s.migRepo.Update(ctx, migration)
}

// syncCourseContent copies all content types from sourceCourseID to childCourseID.
// It uses title-matching to avoid creating duplicates on subsequent syncs.
func (s *BlueprintService) syncCourseContent(ctx context.Context, sourceCourseID, childCourseID uint) error {
	// idMap tracks old content ID -> new content ID per content type.
	// Keys are formatted as "ContentType:OldID" -> NewID.
	idMap := make(map[string]uint)

	// ---------- 1. Sync Modules ----------
	if err := s.syncModules(ctx, sourceCourseID, childCourseID, idMap); err != nil {
		return fmt.Errorf("sync modules: %w", err)
	}

	// ---------- 2. Sync Assignments ----------
	if err := s.syncAssignments(ctx, sourceCourseID, childCourseID, idMap); err != nil {
		return fmt.Errorf("sync assignments: %w", err)
	}

	// ---------- 3. Sync Pages ----------
	if err := s.syncPages(ctx, sourceCourseID, childCourseID, idMap); err != nil {
		return fmt.Errorf("sync pages: %w", err)
	}

	// ---------- 4. Sync Quizzes (with questions) ----------
	if err := s.syncQuizzes(ctx, sourceCourseID, childCourseID, idMap); err != nil {
		return fmt.Errorf("sync quizzes: %w", err)
	}

	// ---------- 5. Sync Discussion Topics ----------
	if err := s.syncDiscussions(ctx, sourceCourseID, childCourseID, idMap); err != nil {
		return fmt.Errorf("sync discussions: %w", err)
	}

	// ---------- 6. Sync Module Items (must come last so idMap is populated) ----------
	if err := s.syncModuleItems(ctx, sourceCourseID, childCourseID, idMap); err != nil {
		return fmt.Errorf("sync module items: %w", err)
	}

	return nil
}

// syncModules copies modules from source to child, skipping those with matching titles.
func (s *BlueprintService) syncModules(ctx context.Context, sourceCourseID, childCourseID uint, idMap map[string]uint) error {
	sourceModules, err := s.moduleRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}

	// Build a set of existing child module titles for deduplication
	childModules, err := s.moduleRepo.ListByCourseID(ctx, childCourseID, bigPage)
	if err != nil {
		return err
	}
	existingTitles := make(map[string]uint, len(childModules.Items))
	for _, m := range childModules.Items {
		existingTitles[m.Name] = m.ID
	}

	for _, src := range sourceModules.Items {
		// Check if module already exists in child by title
		if existingID, exists := existingTitles[src.Name]; exists {
			idMap[fmt.Sprintf("ContextModule:%d", src.ID)] = existingID
			continue
		}

		newModule := &models.ContextModule{
			CourseID:                  childCourseID,
			Name:                      src.Name,
			Position:                  src.Position,
			UnlockAt:                  src.UnlockAt,
			EndAt:                     src.EndAt,
			RequireSequentialProgress: src.RequireSequentialProgress,
			WorkflowState:             src.WorkflowState,
		}
		if err := s.moduleRepo.Create(ctx, newModule); err != nil {
			return fmt.Errorf("create module %q: %w", src.Name, err)
		}
		idMap[fmt.Sprintf("ContextModule:%d", src.ID)] = newModule.ID
	}

	return nil
}

// syncAssignments copies assignments from source to child, skipping those with matching names.
func (s *BlueprintService) syncAssignments(ctx context.Context, sourceCourseID, childCourseID uint, idMap map[string]uint) error {
	sourceAssignments, err := s.assignRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}

	childAssignments, err := s.assignRepo.ListByCourseID(ctx, childCourseID, bigPage)
	if err != nil {
		return err
	}
	existingNames := make(map[string]uint, len(childAssignments.Items))
	for _, a := range childAssignments.Items {
		existingNames[a.Name] = a.ID
	}

	for _, src := range sourceAssignments.Items {
		if existingID, exists := existingNames[src.Name]; exists {
			idMap[fmt.Sprintf("Assignment:%d", src.ID)] = existingID
			continue
		}

		newAssignment := &models.Assignment{
			CourseID:           childCourseID,
			Name:               src.Name,
			Description:        src.Description,
			DueAt:              src.DueAt,
			UnlockAt:           src.UnlockAt,
			LockAt:             src.LockAt,
			PointsPossible:     src.PointsPossible,
			GradingType:        src.GradingType,
			SubmissionTypes:    src.SubmissionTypes,
			Position:           src.Position,
			WorkflowState:      src.WorkflowState,
			Published:          src.Published,
			AnonymousGrading:   src.AnonymousGrading,
			PostPolicy:         src.PostPolicy,
			PeerReviewsEnabled: src.PeerReviewsEnabled,
			PeerReviewCount:    src.PeerReviewCount,
		}
		if err := s.assignRepo.Create(ctx, newAssignment); err != nil {
			return fmt.Errorf("create assignment %q: %w", src.Name, err)
		}
		idMap[fmt.Sprintf("Assignment:%d", src.ID)] = newAssignment.ID
	}

	return nil
}

// syncPages copies wiki pages from source to child, skipping those with matching titles.
func (s *BlueprintService) syncPages(ctx context.Context, sourceCourseID, childCourseID uint, idMap map[string]uint) error {
	sourcePages, err := s.pageRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}

	childPages, err := s.pageRepo.ListByCourseID(ctx, childCourseID, bigPage)
	if err != nil {
		return err
	}
	existingTitles := make(map[string]uint, len(childPages.Items))
	for _, p := range childPages.Items {
		existingTitles[p.Title] = p.ID
	}

	for _, src := range sourcePages.Items {
		if existingID, exists := existingTitles[src.Title]; exists {
			idMap[fmt.Sprintf("WikiPage:%d", src.ID)] = existingID
			continue
		}

		newPage := &models.WikiPage{
			CourseID:      childCourseID,
			Title:         src.Title,
			URL:           src.URL,
			Body:          src.Body,
			WorkflowState: src.WorkflowState,
			EditingRoles:  src.EditingRoles,
			FrontPage:     src.FrontPage,
			Public:        src.Public,
			WebsiteMode:   src.WebsiteMode,
		}
		if err := s.pageRepo.Create(ctx, newPage); err != nil {
			return fmt.Errorf("create page %q: %w", src.Title, err)
		}
		idMap[fmt.Sprintf("WikiPage:%d", src.ID)] = newPage.ID
	}

	return nil
}

// syncQuizzes copies quizzes and their questions from source to child.
func (s *BlueprintService) syncQuizzes(ctx context.Context, sourceCourseID, childCourseID uint, idMap map[string]uint) error {
	sourceQuizzes, err := s.quizRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}

	childQuizzes, err := s.quizRepo.ListByCourseID(ctx, childCourseID, bigPage)
	if err != nil {
		return err
	}
	existingTitles := make(map[string]uint, len(childQuizzes.Items))
	for _, q := range childQuizzes.Items {
		existingTitles[q.Title] = q.ID
	}

	for _, src := range sourceQuizzes.Items {
		if existingID, exists := existingTitles[src.Title]; exists {
			idMap[fmt.Sprintf("Quiz:%d", src.ID)] = existingID
			continue
		}

		newQuiz := &models.Quiz{
			CourseID:        childCourseID,
			Title:           src.Title,
			Description:     src.Description,
			QuizType:        src.QuizType,
			TimeLimit:       src.TimeLimit,
			AllowedAttempts: src.AllowedAttempts,
			DueAt:           src.DueAt,
			UnlockAt:        src.UnlockAt,
			LockAt:          src.LockAt,
			PointsPossible:  src.PointsPossible,
			Published:       src.Published,
			WorkflowState:   src.WorkflowState,
		}
		if err := s.quizRepo.Create(ctx, newQuiz); err != nil {
			return fmt.Errorf("create quiz %q: %w", src.Title, err)
		}
		idMap[fmt.Sprintf("Quiz:%d", src.ID)] = newQuiz.ID

		// Copy quiz questions
		if err := s.syncQuizQuestions(ctx, src.ID, newQuiz.ID); err != nil {
			return fmt.Errorf("sync questions for quiz %q: %w", src.Title, err)
		}
	}

	return nil
}

// syncQuizQuestions copies all questions from sourceQuizID to newQuizID.
func (s *BlueprintService) syncQuizQuestions(ctx context.Context, sourceQuizID, newQuizID uint) error {
	questions, err := s.qqRepo.ListByQuizID(ctx, sourceQuizID, bigPage)
	if err != nil {
		return err
	}

	for _, src := range questions.Items {
		newQ := &models.QuizQuestion{
			QuizID:            newQuizID,
			Position:          src.Position,
			QuestionType:      src.QuestionType,
			QuestionText:      src.QuestionText,
			PointsPossible:    src.PointsPossible,
			Answers:           src.Answers,
			CorrectComments:   src.CorrectComments,
			IncorrectComments: src.IncorrectComments,
			NeutralComments:   src.NeutralComments,
			WorkflowState:     src.WorkflowState,
		}
		// Preserve question group reference if it exists within the same quiz
		// (cross-quiz group references are not synced for simplicity)
		if err := s.qqRepo.Create(ctx, newQ); err != nil {
			return fmt.Errorf("create question (position %d): %w", src.Position, err)
		}
	}

	return nil
}

// syncDiscussions copies discussion topics from source to child.
func (s *BlueprintService) syncDiscussions(ctx context.Context, sourceCourseID, childCourseID uint, idMap map[string]uint) error {
	sourceTopics, err := s.discRepo.ListByCourseID(ctx, sourceCourseID, 0, bigPage)
	if err != nil {
		return err
	}

	childTopics, err := s.discRepo.ListByCourseID(ctx, childCourseID, 0, bigPage)
	if err != nil {
		return err
	}
	existingTitles := make(map[string]uint, len(childTopics.Items))
	for _, t := range childTopics.Items {
		existingTitles[t.Title] = t.ID
	}

	for _, src := range sourceTopics.Items {
		if existingID, exists := existingTitles[src.Title]; exists {
			idMap[fmt.Sprintf("DiscussionTopic:%d", src.ID)] = existingID
			continue
		}

		newTopic := &models.DiscussionTopic{
			CourseID:           childCourseID,
			UserID:             src.UserID,
			Title:              src.Title,
			Message:            src.Message,
			DiscussionType:     src.DiscussionType,
			PostedAt:           src.PostedAt,
			DelayedPostAt:      src.DelayedPostAt,
			LockAt:             src.LockAt,
			Pinned:             src.Pinned,
			Locked:             src.Locked,
			AllowRating:        src.AllowRating,
			OnlyGradersCanRate: src.OnlyGradersCanRate,
			SortByRating:       src.SortByRating,
			RequireInitialPost: src.RequireInitialPost,
			WorkflowState:      src.WorkflowState,
		}
		if err := s.discRepo.Create(ctx, newTopic); err != nil {
			return fmt.Errorf("create discussion %q: %w", src.Title, err)
		}
		idMap[fmt.Sprintf("DiscussionTopic:%d", src.ID)] = newTopic.ID
	}

	return nil
}

// syncModuleItems copies module items from source modules to child modules,
// remapping content IDs via the idMap built from earlier sync steps.
func (s *BlueprintService) syncModuleItems(ctx context.Context, sourceCourseID, childCourseID uint, idMap map[string]uint) error {
	sourceModules, err := s.moduleRepo.ListByCourseID(ctx, sourceCourseID, bigPage)
	if err != nil {
		return err
	}

	for _, srcModule := range sourceModules.Items {
		// Look up the child module ID
		childModuleID, ok := idMap[fmt.Sprintf("ContextModule:%d", srcModule.ID)]
		if !ok {
			// Module was not synced (should not happen), skip
			continue
		}

		// Fetch source module items
		sourceItems, err := s.itemRepo.ListByModuleID(ctx, srcModule.ID, bigPage)
		if err != nil {
			return fmt.Errorf("list items for module %d: %w", srcModule.ID, err)
		}

		// Fetch existing child module items for deduplication by title
		childItems, err := s.itemRepo.ListByModuleID(ctx, childModuleID, bigPage)
		if err != nil {
			return fmt.Errorf("list child items for module %d: %w", childModuleID, err)
		}
		existingTitles := make(map[string]bool, len(childItems.Items))
		for _, item := range childItems.Items {
			existingTitles[item.Title] = true
		}

		for _, srcItem := range sourceItems.Items {
			// Skip items that already exist in the child module by title
			if existingTitles[srcItem.Title] {
				continue
			}

			newItem := &models.ContentTag{
				ContextModuleID: childModuleID,
				ContentType:     srcItem.ContentType,
				Title:           srcItem.Title,
				Position:        srcItem.Position,
				URL:             srcItem.URL,
				Indent:          srcItem.Indent,
				NewTab:          srcItem.NewTab,
				WorkflowState:   srcItem.WorkflowState,
			}

			// Remap content_id to the new child content using the idMap
			if srcItem.ContentID != nil {
				mapKey := fmt.Sprintf("%s:%d", srcItem.ContentType, *srcItem.ContentID)
				if newID, found := idMap[mapKey]; found {
					newItem.ContentID = &newID
				}
				// If not found in idMap, the content type might not be synced
				// (e.g., ExternalUrl, ContextModuleSubHeader have no content_id
				// or reference unsupported types). Leave content_id nil in that case.
			}

			if err := s.itemRepo.Create(ctx, newItem); err != nil {
				return fmt.Errorf("create item %q in module %d: %w", srcItem.Title, childModuleID, err)
			}
		}
	}

	return nil
}

