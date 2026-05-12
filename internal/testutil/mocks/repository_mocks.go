package mocks

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockUserRepository mocks repository.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByLoginID(ctx context.Context, loginID string) (*models.User, error) {
	args := m.Called(ctx, loginID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) FindBySISUserID(ctx context.Context, sisUserID string) (*models.User, error) {
	args := m.Called(ctx, sisUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) List(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.User]), args.Error(1)
}

func (m *MockUserRepository) FindByResetToken(ctx context.Context, token string) (*models.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Search(ctx context.Context, searchTerm string, params repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	args := m.Called(ctx, searchTerm, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.User]), args.Error(1)
}

// MockCourseRepository mocks repository.CourseRepository
type MockCourseRepository struct {
	mock.Mock
}

func (m *MockCourseRepository) Create(ctx context.Context, course *models.Course) error {
	args := m.Called(ctx, course)
	return args.Error(0)
}

func (m *MockCourseRepository) FindByID(ctx context.Context, id uint) (*models.Course, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Course), args.Error(1)
}

func (m *MockCourseRepository) FindBySISCourseID(ctx context.Context, sisCourseID string) (*models.Course, error) {
	args := m.Called(ctx, sisCourseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Course), args.Error(1)
}

func (m *MockCourseRepository) Update(ctx context.Context, course *models.Course) error {
	args := m.Called(ctx, course)
	return args.Error(0)
}

func (m *MockCourseRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCourseRepository) List(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Course]), args.Error(1)
}

func (m *MockCourseRepository) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Course], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Course]), args.Error(1)
}

// MockEnrollmentRepository mocks repository.EnrollmentRepository
type MockEnrollmentRepository struct {
	mock.Mock
}

func (m *MockEnrollmentRepository) Create(ctx context.Context, enrollment *models.Enrollment) error {
	args := m.Called(ctx, enrollment)
	return args.Error(0)
}

func (m *MockEnrollmentRepository) FindByID(ctx context.Context, id uint) (*models.Enrollment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Enrollment), args.Error(1)
}

func (m *MockEnrollmentRepository) Update(ctx context.Context, enrollment *models.Enrollment) error {
	args := m.Called(ctx, enrollment)
	return args.Error(0)
}

func (m *MockEnrollmentRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Enrollment], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Enrollment]), args.Error(1)
}

func (m *MockEnrollmentRepository) ListByUserID(ctx context.Context, userID uint) ([]models.Enrollment, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Enrollment), args.Error(1)
}

func (m *MockEnrollmentRepository) FindByUserAndCourse(ctx context.Context, userID, courseID uint) (*models.Enrollment, error) {
	args := m.Called(ctx, userID, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Enrollment), args.Error(1)
}

func (m *MockEnrollmentRepository) CountByCourseIDs(ctx context.Context, courseIDs []uint) (map[uint]int64, error) {
	args := m.Called(ctx, courseIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uint]int64), args.Error(1)
}

// MockSectionRepository mocks repository.SectionRepository
type MockSectionRepository struct {
	mock.Mock
}

func (m *MockSectionRepository) Create(ctx context.Context, section *models.CourseSection) error {
	args := m.Called(ctx, section)
	return args.Error(0)
}

func (m *MockSectionRepository) FindByID(ctx context.Context, id uint) (*models.CourseSection, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CourseSection), args.Error(1)
}

func (m *MockSectionRepository) FindBySISSectionID(ctx context.Context, sisSectionID string) (*models.CourseSection, error) {
	args := m.Called(ctx, sisSectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CourseSection), args.Error(1)
}

func (m *MockSectionRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CourseSection], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.CourseSection]), args.Error(1)
}

// MockAssignmentRepository mocks repository.AssignmentRepository
type MockAssignmentRepository struct {
	mock.Mock
}

func (m *MockAssignmentRepository) Create(ctx context.Context, assignment *models.Assignment) error {
	args := m.Called(ctx, assignment)
	return args.Error(0)
}

func (m *MockAssignmentRepository) FindByID(ctx context.Context, id uint) (*models.Assignment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Assignment), args.Error(1)
}

func (m *MockAssignmentRepository) Update(ctx context.Context, assignment *models.Assignment) error {
	args := m.Called(ctx, assignment)
	return args.Error(0)
}

func (m *MockAssignmentRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAssignmentRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Assignment], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Assignment]), args.Error(1)
}

// MockSubmissionRepository mocks repository.SubmissionRepository
type MockSubmissionRepository struct {
	mock.Mock
}

func (m *MockSubmissionRepository) Create(ctx context.Context, submission *models.Submission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockSubmissionRepository) FindByID(ctx context.Context, id uint) (*models.Submission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) FindByAssignmentAndUser(ctx context.Context, assignmentID, userID uint) (*models.Submission, error) {
	args := m.Called(ctx, assignmentID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) Update(ctx context.Context, submission *models.Submission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockSubmissionRepository) ListByAssignmentID(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	args := m.Called(ctx, assignmentID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Submission]), args.Error(1)
}

func (m *MockSubmissionRepository) ListByUserAndCourse(ctx context.Context, userID, courseID uint) ([]models.Submission, error) {
	args := m.Called(ctx, userID, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) ListByUserAndAssignmentIDs(ctx context.Context, userID uint, assignmentIDs []uint) ([]models.Submission, error) {
	args := m.Called(ctx, userID, assignmentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) BulkListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Submission], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Submission]), args.Error(1)
}

func (m *MockSubmissionRepository) PostGradesByAssignment(ctx context.Context, assignmentID uint, postedAt *time.Time) error {
	args := m.Called(ctx, assignmentID, postedAt)
	return args.Error(0)
}

// MockAssignmentGroupRepository mocks repository.AssignmentGroupRepository
type MockAssignmentGroupRepository struct {
	mock.Mock
}

func (m *MockAssignmentGroupRepository) Create(ctx context.Context, group *models.AssignmentGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockAssignmentGroupRepository) FindByID(ctx context.Context, id uint) (*models.AssignmentGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AssignmentGroup), args.Error(1)
}

func (m *MockAssignmentGroupRepository) Update(ctx context.Context, group *models.AssignmentGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockAssignmentGroupRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAssignmentGroupRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AssignmentGroup], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.AssignmentGroup]), args.Error(1)
}

// MockAccessTokenRepository mocks repository.AccessTokenRepository
type MockAccessTokenRepository struct {
	mock.Mock
}

func (m *MockAccessTokenRepository) Create(ctx context.Context, token *models.AccessToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockAccessTokenRepository) FindByID(ctx context.Context, id uint) (*models.AccessToken, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccessToken), args.Error(1)
}

func (m *MockAccessTokenRepository) FindByToken(ctx context.Context, tokenHash string) (*models.AccessToken, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccessToken), args.Error(1)
}

func (m *MockAccessTokenRepository) FindByRefreshToken(ctx context.Context, refreshToken string) (*models.AccessToken, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccessToken), args.Error(1)
}

func (m *MockAccessTokenRepository) Update(ctx context.Context, token *models.AccessToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockAccessTokenRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAccessTokenRepository) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AccessToken], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.AccessToken]), args.Error(1)
}

func (m *MockAccessTokenRepository) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockModuleRepository mocks repository.ModuleRepository
type MockModuleRepository struct {
	mock.Mock
}

func (m *MockModuleRepository) Create(ctx context.Context, module *models.ContextModule) error {
	args := m.Called(ctx, module)
	return args.Error(0)
}

func (m *MockModuleRepository) FindByID(ctx context.Context, id uint) (*models.ContextModule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContextModule), args.Error(1)
}

func (m *MockModuleRepository) Update(ctx context.Context, module *models.ContextModule) error {
	args := m.Called(ctx, module)
	return args.Error(0)
}

func (m *MockModuleRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockModuleRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextModule], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.ContextModule]), args.Error(1)
}

func (m *MockModuleRepository) FindActiveByDateRange(ctx context.Context, courseID uint, date time.Time) (*models.ContextModule, error) {
	args := m.Called(ctx, courseID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContextModule), args.Error(1)
}

func (m *MockModuleRepository) ReorderModules(ctx context.Context, courseID uint, moduleIDs []uint) error {
	args := m.Called(ctx, courseID, moduleIDs)
	return args.Error(0)
}

// MockModuleItemRepository mocks repository.ModuleItemRepository
type MockModuleItemRepository struct {
	mock.Mock
}

func (m *MockModuleItemRepository) Create(ctx context.Context, item *models.ContentTag) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockModuleItemRepository) FindByID(ctx context.Context, id uint) (*models.ContentTag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContentTag), args.Error(1)
}

func (m *MockModuleItemRepository) Update(ctx context.Context, item *models.ContentTag) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockModuleItemRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockModuleItemRepository) ListByModuleID(ctx context.Context, moduleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentTag], error) {
	args := m.Called(ctx, moduleID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.ContentTag]), args.Error(1)
}

func (m *MockModuleItemRepository) ReorderItems(ctx context.Context, moduleID uint, itemIDs []uint) error {
	args := m.Called(ctx, moduleID, itemIDs)
	return args.Error(0)
}

func (m *MockModuleItemRepository) MoveItemToModule(ctx context.Context, itemID uint, targetModuleID uint, position int) error {
	args := m.Called(ctx, itemID, targetModuleID, position)
	return args.Error(0)
}

// MockPageRepository mocks repository.PageRepository
type MockPageRepository struct {
	mock.Mock
}

func (m *MockPageRepository) Create(ctx context.Context, page *models.WikiPage) error {
	args := m.Called(ctx, page)
	return args.Error(0)
}

func (m *MockPageRepository) FindByID(ctx context.Context, id uint) (*models.WikiPage, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WikiPage), args.Error(1)
}

func (m *MockPageRepository) FindByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	args := m.Called(ctx, courseID, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WikiPage), args.Error(1)
}

func (m *MockPageRepository) Update(ctx context.Context, page *models.WikiPage) error {
	args := m.Called(ctx, page)
	return args.Error(0)
}

func (m *MockPageRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPageRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.WikiPage], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.WikiPage]), args.Error(1)
}

func (m *MockPageRepository) FindPublicByCourseAndURL(ctx context.Context, courseID uint, url string) (*models.WikiPage, error) {
	args := m.Called(ctx, courseID, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WikiPage), args.Error(1)
}

// MockQuizQuestionRepository mocks repository.QuizQuestionRepository
type MockQuizQuestionRepository struct {
	mock.Mock
}

func (m *MockQuizQuestionRepository) Create(ctx context.Context, question *models.QuizQuestion) error {
	args := m.Called(ctx, question)
	return args.Error(0)
}

func (m *MockQuizQuestionRepository) FindByID(ctx context.Context, id uint) (*models.QuizQuestion, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizQuestion), args.Error(1)
}

func (m *MockQuizQuestionRepository) Update(ctx context.Context, question *models.QuizQuestion) error {
	args := m.Called(ctx, question)
	return args.Error(0)
}

func (m *MockQuizQuestionRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuizQuestionRepository) ListByQuizID(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizQuestion], error) {
	args := m.Called(ctx, quizID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.QuizQuestion]), args.Error(1)
}

func (m *MockQuizQuestionRepository) ListByGroupID(ctx context.Context, groupID uint) ([]models.QuizQuestion, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizQuestion), args.Error(1)
}

// MockQuizSubmissionRepository mocks repository.QuizSubmissionRepository
type MockQuizSubmissionRepository struct {
	mock.Mock
}

func (m *MockQuizSubmissionRepository) Create(ctx context.Context, submission *models.QuizSubmission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockQuizSubmissionRepository) FindByID(ctx context.Context, id uint) (*models.QuizSubmission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmission), args.Error(1)
}

func (m *MockQuizSubmissionRepository) Update(ctx context.Context, submission *models.QuizSubmission) error {
	args := m.Called(ctx, submission)
	return args.Error(0)
}

func (m *MockQuizSubmissionRepository) FindByQuizAndUser(ctx context.Context, quizID, userID uint) (*models.QuizSubmission, error) {
	args := m.Called(ctx, quizID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmission), args.Error(1)
}

func (m *MockQuizSubmissionRepository) ListByUserAndQuizIDs(ctx context.Context, userID uint, quizIDs []uint) ([]models.QuizSubmission, error) {
	args := m.Called(ctx, userID, quizIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmission), args.Error(1)
}

func (m *MockQuizSubmissionRepository) ListByQuizID(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizSubmission], error) {
	args := m.Called(ctx, quizID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.QuizSubmission]), args.Error(1)
}

func (m *MockQuizSubmissionRepository) ListCompletedByQuizID(ctx context.Context, quizID uint) ([]models.QuizSubmission, error) {
	args := m.Called(ctx, quizID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmission), args.Error(1)
}

// MockQuizSubmissionAnswerRepository mocks repository.QuizSubmissionAnswerRepository
type MockQuizSubmissionAnswerRepository struct {
	mock.Mock
}

func (m *MockQuizSubmissionAnswerRepository) Create(ctx context.Context, answer *models.QuizSubmissionAnswer) error {
	args := m.Called(ctx, answer)
	return args.Error(0)
}

func (m *MockQuizSubmissionAnswerRepository) BulkCreate(ctx context.Context, answers []models.QuizSubmissionAnswer) error {
	args := m.Called(ctx, answers)
	return args.Error(0)
}

func (m *MockQuizSubmissionAnswerRepository) FindByID(ctx context.Context, id uint) (*models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmissionAnswer), args.Error(1)
}

func (m *MockQuizSubmissionAnswerRepository) Update(ctx context.Context, answer *models.QuizSubmissionAnswer) error {
	args := m.Called(ctx, answer)
	return args.Error(0)
}

func (m *MockQuizSubmissionAnswerRepository) ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, submissionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmissionAnswer), args.Error(1)
}

func (m *MockQuizSubmissionAnswerRepository) FindBySubmissionAndQuestion(ctx context.Context, submissionID, questionID uint) (*models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, submissionID, questionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.QuizSubmissionAnswer), args.Error(1)
}

func (m *MockQuizSubmissionAnswerRepository) ListBySubmissionIDs(ctx context.Context, submissionIDs []uint) ([]models.QuizSubmissionAnswer, error) {
	args := m.Called(ctx, submissionIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizSubmissionAnswer), args.Error(1)
}

// MockSubmissionCommentRepository mocks repository.SubmissionCommentRepository
type MockSubmissionCommentRepository struct {
	mock.Mock
}

func (m *MockSubmissionCommentRepository) Create(ctx context.Context, comment *models.SubmissionComment) error {
	args := m.Called(ctx, comment)
	return args.Error(0)
}

func (m *MockSubmissionCommentRepository) ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.SubmissionComment, error) {
	args := m.Called(ctx, submissionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.SubmissionComment), args.Error(1)
}

// MockAttachmentRepository mocks repository.AttachmentRepository
type MockAttachmentRepository struct {
	mock.Mock
}

func (m *MockAttachmentRepository) Create(ctx context.Context, attachment *models.Attachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockAttachmentRepository) FindByID(ctx context.Context, id uint) (*models.Attachment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Attachment), args.Error(1)
}

func (m *MockAttachmentRepository) Update(ctx context.Context, attachment *models.Attachment) error {
	args := m.Called(ctx, attachment)
	return args.Error(0)
}

func (m *MockAttachmentRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAttachmentRepository) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	args := m.Called(ctx, contextType, contextID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Attachment]), args.Error(1)
}

func (m *MockAttachmentRepository) ListByFolderID(ctx context.Context, folderID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Attachment], error) {
	args := m.Called(ctx, folderID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Attachment]), args.Error(1)
}

// MockQuizRepository mocks repository.QuizRepository
type MockQuizRepository struct {
	mock.Mock
}

func (m *MockQuizRepository) Create(ctx context.Context, quiz *models.Quiz) error {
	args := m.Called(ctx, quiz)
	return args.Error(0)
}

func (m *MockQuizRepository) FindByID(ctx context.Context, id uint) (*models.Quiz, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Quiz), args.Error(1)
}

func (m *MockQuizRepository) Update(ctx context.Context, quiz *models.Quiz) error {
	args := m.Called(ctx, quiz)
	return args.Error(0)
}

func (m *MockQuizRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockQuizRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Quiz], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Quiz]), args.Error(1)
}

// MockLatePolicyRepository mocks repository.LatePolicyRepository
type MockLatePolicyRepository struct {
	mock.Mock
}

func (m *MockLatePolicyRepository) Create(ctx context.Context, policy *models.LatePolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockLatePolicyRepository) FindByCourseID(ctx context.Context, courseID uint) (*models.LatePolicy, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LatePolicy), args.Error(1)
}

func (m *MockLatePolicyRepository) Update(ctx context.Context, policy *models.LatePolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockLatePolicyRepository) Delete(ctx context.Context, courseID uint) error {
	args := m.Called(ctx, courseID)
	return args.Error(0)
}

// MockGradingPeriodGroupRepository mocks repository.GradingPeriodGroupRepository
type MockGradingPeriodGroupRepository struct {
	mock.Mock
}

func (m *MockGradingPeriodGroupRepository) Create(ctx context.Context, group *models.GradingPeriodGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGradingPeriodGroupRepository) FindByID(ctx context.Context, id uint) (*models.GradingPeriodGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingPeriodGroup), args.Error(1)
}

func (m *MockGradingPeriodGroupRepository) Update(ctx context.Context, group *models.GradingPeriodGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGradingPeriodGroupRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGradingPeriodGroupRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradingPeriodGroup], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GradingPeriodGroup]), args.Error(1)
}

// MockGradingPeriodRepository mocks repository.GradingPeriodRepository
type MockGradingPeriodRepository struct {
	mock.Mock
}

func (m *MockGradingPeriodRepository) Create(ctx context.Context, period *models.GradingPeriod) error {
	args := m.Called(ctx, period)
	return args.Error(0)
}

func (m *MockGradingPeriodRepository) FindByID(ctx context.Context, id uint) (*models.GradingPeriod, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingPeriod), args.Error(1)
}

func (m *MockGradingPeriodRepository) Update(ctx context.Context, period *models.GradingPeriod) error {
	args := m.Called(ctx, period)
	return args.Error(0)
}

func (m *MockGradingPeriodRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGradingPeriodRepository) ListByGroupID(ctx context.Context, groupID uint) ([]models.GradingPeriod, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GradingPeriod), args.Error(1)
}

// MockGradingStandardRepository mocks repository.GradingStandardRepository
type MockGradingStandardRepository struct {
	mock.Mock
}

func (m *MockGradingStandardRepository) Create(ctx context.Context, standard *models.GradingStandard) error {
	args := m.Called(ctx, standard)
	return args.Error(0)
}

func (m *MockGradingStandardRepository) FindByID(ctx context.Context, id uint) (*models.GradingStandard, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingStandard), args.Error(1)
}

func (m *MockGradingStandardRepository) Update(ctx context.Context, standard *models.GradingStandard) error {
	args := m.Called(ctx, standard)
	return args.Error(0)
}

func (m *MockGradingStandardRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGradingStandardRepository) ListByCourse(ctx context.Context, courseID uint) ([]models.GradingStandard, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GradingStandard), args.Error(1)
}

func (m *MockGradingStandardRepository) FindActiveByCourse(ctx context.Context, courseID uint) (*models.GradingStandard, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingStandard), args.Error(1)
}

// MockGroupMembershipRepository implements repository.GroupMembershipRepository for testing.
type MockGroupMembershipRepository struct {
	mock.Mock
}

func (m *MockGroupMembershipRepository) Create(ctx context.Context, membership *models.GroupMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

func (m *MockGroupMembershipRepository) FindByID(ctx context.Context, id uint) (*models.GroupMembership, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupMembership), args.Error(1)
}

func (m *MockGroupMembershipRepository) Update(ctx context.Context, membership *models.GroupMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

func (m *MockGroupMembershipRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupMembershipRepository) ListByGroupID(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupMembership], error) {
	args := m.Called(ctx, groupID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GroupMembership]), args.Error(1)
}

func (m *MockGroupMembershipRepository) FindByGroupAndUser(ctx context.Context, groupID, userID uint) (*models.GroupMembership, error) {
	args := m.Called(ctx, groupID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupMembership), args.Error(1)
}

func (m *MockGroupMembershipRepository) FindUserGroupInCategory(ctx context.Context, userID, groupCategoryID uint) (*models.Group, error) {
	args := m.Called(ctx, userID, groupCategoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

// FindByIDs mocks for batch fetcher methods (P1 INTERFACES_PATCH).

func (m *MockUserRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.User, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.User), args.Error(1)
}

func (m *MockAssignmentRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.Assignment, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Assignment), args.Error(1)
}

func (m *MockSubmissionRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.Submission, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) FindByAssignmentAndUserIDs(ctx context.Context, assignmentID uint, userIDs []uint) ([]models.Submission, error) {
	args := m.Called(ctx, assignmentID, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Submission), args.Error(1)
}

func (m *MockSubmissionRepository) RunInTransaction(ctx context.Context, fn func(txRepo repository.SubmissionRepository) error) error {
	return fn(m)
}

func (m *MockQuizQuestionRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.QuizQuestion, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.QuizQuestion), args.Error(1)
}

// PortfolioSectionRepository mock not yet written; add when first portfolio service test lands.
