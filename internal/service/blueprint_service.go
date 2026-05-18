package service

// BlueprintService is the public facade for course-blueprint operations:
// template CRUD, associated-course management, sync orchestration, and
// the unsynced-changes report. The implementation is split across four
// sibling files for navigability — see blueprint_templates.go,
// blueprint_associations.go, blueprint_sync.go, and blueprint_changes.go.
// All exported methods continue to hang off this single struct so the
// handler call sites in internal/api/v1/handlers/blueprints.go remain
// untouched.
//
// This file owns ONLY:
//   - the BlueprintService struct + its repository fields
//   - the NewBlueprintService constructor
//
// All behavior lives in the sibling files. The split was made in Wave 5
// (chore/wave5-split-quiz-blueprint) because blueprint_service.go had
// grown to ~750 LOC and the sync orchestrator + 6 sub-syncs dominated
// the file at the expense of template / association / changes concerns.

import (
	"github.com/EduThemes/paper-lms/internal/repository"
)

type BlueprintService struct {
	tmplRepo   repository.BlueprintTemplateRepository
	subRepo    repository.BlueprintSubscriptionRepository
	migRepo    repository.BlueprintMigrationRepository
	moduleRepo repository.ModuleRepository
	itemRepo   repository.ModuleItemRepository
	assignRepo repository.AssignmentRepository
	pageRepo   repository.PageRepository
	quizRepo   repository.QuizRepository
	qqRepo     repository.QuizQuestionRepository
	discRepo   repository.DiscussionTopicRepository
}

func NewBlueprintService(
	tmplRepo repository.BlueprintTemplateRepository,
	subRepo repository.BlueprintSubscriptionRepository,
	migRepo repository.BlueprintMigrationRepository,
	moduleRepo repository.ModuleRepository,
	itemRepo repository.ModuleItemRepository,
	assignRepo repository.AssignmentRepository,
	pageRepo repository.PageRepository,
	quizRepo repository.QuizRepository,
	qqRepo repository.QuizQuestionRepository,
	discRepo repository.DiscussionTopicRepository,
) *BlueprintService {
	return &BlueprintService{
		tmplRepo:   tmplRepo,
		subRepo:    subRepo,
		migRepo:    migRepo,
		moduleRepo: moduleRepo,
		itemRepo:   itemRepo,
		assignRepo: assignRepo,
		pageRepo:   pageRepo,
		quizRepo:   quizRepo,
		qqRepo:     qqRepo,
		discRepo:   discRepo,
	}
}

// bigPage is a pagination param that fetches up to 10000 items (effectively all).
// Used by the sync + changes paths. Defined at package level so all blueprint_*.go
// files share the same constant.
var bigPage = repository.PaginationParams{Page: 1, PerPage: 10000}
