package service

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type HomeData struct {
	Course       *models.Course           `json:"course"`
	Buttons      []models.CourseHomeButton `json:"buttons"`
	TodaysLesson *ResolvedLesson          `json:"todays_lesson"`
	ContinueURL  *ResolvedContinue        `json:"continue_url"`
}

type ResolvedLesson struct {
	LinkType string `json:"link_type"`
	LinkID   *uint  `json:"link_id"`
	LinkURL  string `json:"link_url"`
	Label    string `json:"label"`
}

type ResolvedContinue struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type CourseHomeService struct {
	courseRepo    repository.CourseRepository
	buttonRepo   repository.CourseHomeButtonRepository
	overrideRepo repository.TodaysLessonOverrideRepository
	visitRepo    repository.CourseVisitRepository
	moduleRepo   repository.ModuleRepository
}

func NewCourseHomeService(
	courseRepo repository.CourseRepository,
	buttonRepo repository.CourseHomeButtonRepository,
	overrideRepo repository.TodaysLessonOverrideRepository,
	visitRepo repository.CourseVisitRepository,
	moduleRepo repository.ModuleRepository,
) *CourseHomeService {
	return &CourseHomeService{
		courseRepo:    courseRepo,
		buttonRepo:   buttonRepo,
		overrideRepo: overrideRepo,
		visitRepo:    visitRepo,
		moduleRepo:   moduleRepo,
	}
}

func (s *CourseHomeService) GetHomeData(ctx context.Context, courseID, userID uint) (*HomeData, error) {
	course, err := s.courseRepo.FindByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	allButtons, err := s.buttonRepo.ListByCourseID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	var visibleButtons []models.CourseHomeButton
	for _, b := range allButtons {
		if b.Visible {
			visibleButtons = append(visibleButtons, b)
		}
	}

	todaysLesson, err := s.ResolveTodaysLesson(ctx, courseID)
	if err != nil {
		return nil, err
	}

	var continueURL *ResolvedContinue
	visit, err := s.visitRepo.FindByUserAndCourse(ctx, userID, courseID)
	if err == nil && visit != nil {
		continueURL = &ResolvedContinue{
			URL:   visit.LastURL,
			Title: visit.LastTitle,
		}
	}

	return &HomeData{
		Course:       course,
		Buttons:      visibleButtons,
		TodaysLesson: todaysLesson,
		ContinueURL:  continueURL,
	}, nil
}

func (s *CourseHomeService) ResolveTodaysLesson(ctx context.Context, courseID uint) (*ResolvedLesson, error) {
	today := time.Now().Truncate(24 * time.Hour)

	override, err := s.overrideRepo.FindByCourseAndDate(ctx, courseID, today)
	if err == nil && override != nil {
		return &ResolvedLesson{
			LinkType: override.LinkType,
			LinkID:   override.LinkID,
			LinkURL:  override.LinkURL,
			Label:    override.Label,
		}, nil
	}

	module, err := s.moduleRepo.FindActiveByDateRange(ctx, courseID, today)
	if err == nil && module != nil {
		return &ResolvedLesson{
			LinkType: "module",
			LinkID:   &module.ID,
			LinkURL:  "",
			Label:    module.Name,
		}, nil
	}

	return nil, nil
}

func (s *CourseHomeService) RecordVisit(ctx context.Context, userID, courseID uint, url, title string) error {
	visit := &models.CourseVisit{
		UserID:    userID,
		CourseID:  courseID,
		LastURL:   url,
		LastTitle: title,
	}
	return s.visitRepo.Upsert(ctx, visit)
}

// Button CRUD

func (s *CourseHomeService) GetButtonByID(ctx context.Context, id uint) (*models.CourseHomeButton, error) {
	return s.buttonRepo.FindByID(ctx, id)
}

func (s *CourseHomeService) CreateButton(ctx context.Context, button *models.CourseHomeButton) error {
	return s.buttonRepo.Create(ctx, button)
}

func (s *CourseHomeService) UpdateButton(ctx context.Context, button *models.CourseHomeButton) error {
	return s.buttonRepo.Update(ctx, button)
}

func (s *CourseHomeService) DeleteButton(ctx context.Context, id uint) error {
	return s.buttonRepo.Delete(ctx, id)
}

func (s *CourseHomeService) ListButtons(ctx context.Context, courseID uint) ([]models.CourseHomeButton, error) {
	return s.buttonRepo.ListByCourseID(ctx, courseID)
}

func (s *CourseHomeService) ReorderButtons(ctx context.Context, courseID uint, positions map[uint]int) error {
	return s.buttonRepo.BulkUpdatePositions(ctx, courseID, positions)
}

// Override CRUD

func (s *CourseHomeService) GetOverrideByID(ctx context.Context, id uint) (*models.TodaysLessonOverride, error) {
	return s.overrideRepo.FindByID(ctx, id)
}

func (s *CourseHomeService) CreateOverride(ctx context.Context, override *models.TodaysLessonOverride) error {
	return s.overrideRepo.Create(ctx, override)
}

func (s *CourseHomeService) UpdateOverride(ctx context.Context, override *models.TodaysLessonOverride) error {
	return s.overrideRepo.Update(ctx, override)
}

func (s *CourseHomeService) DeleteOverride(ctx context.Context, id uint) error {
	return s.overrideRepo.Delete(ctx, id)
}

func (s *CourseHomeService) ListOverrides(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.TodaysLessonOverride], error) {
	return s.overrideRepo.ListByCourseID(ctx, courseID, params)
}
