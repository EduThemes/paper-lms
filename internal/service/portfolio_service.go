package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type PortfolioService struct {
	portfolioRepo   repository.PortfolioRepository
	sectionRepo     repository.PortfolioSectionRepository
	artifactRepo    repository.PortfolioArtifactRepository
	reflectionRepo  repository.PortfolioReflectionRepository
	templateRepo    repository.PortfolioTemplateRepository
	commentRepo     repository.PortfolioCommentRepository
	submissionRepo  repository.SubmissionRepository
	assignmentRepo  repository.AssignmentRepository
	userRepo        repository.UserRepository
}

func NewPortfolioService(
	portfolioRepo repository.PortfolioRepository,
	sectionRepo repository.PortfolioSectionRepository,
	artifactRepo repository.PortfolioArtifactRepository,
	reflectionRepo repository.PortfolioReflectionRepository,
	templateRepo repository.PortfolioTemplateRepository,
	commentRepo repository.PortfolioCommentRepository,
	submissionRepo repository.SubmissionRepository,
	assignmentRepo repository.AssignmentRepository,
) *PortfolioService {
	return &PortfolioService{
		portfolioRepo:  portfolioRepo,
		sectionRepo:    sectionRepo,
		artifactRepo:   artifactRepo,
		reflectionRepo: reflectionRepo,
		templateRepo:   templateRepo,
		commentRepo:    commentRepo,
		submissionRepo: submissionRepo,
		assignmentRepo: assignmentRepo,
	}
}

// ---------------------------------------------------------------------------
// Portfolio CRUD
// ---------------------------------------------------------------------------

func (s *PortfolioService) CreatePortfolio(ctx context.Context, portfolio *models.Portfolio) error {
	if portfolio.Title == "" {
		return errors.New("portfolio title is required")
	}
	if portfolio.UserID == 0 {
		return errors.New("user_id is required")
	}

	portfolio.Slug = generateSlug(portfolio.Title)
	if portfolio.ThemeID == "" {
		portfolio.ThemeID = "clean-modern"
	}
	if portfolio.WorkflowState == "" {
		portfolio.WorkflowState = "draft"
	}

	return s.portfolioRepo.Create(ctx, portfolio)
}

// GetPortfolio returns a portfolio by ID, scoped to the caller's tenant.
// 13.1.D — accountID=0 means "no tenant scope" (internal callers only);
// handler-routed callers MUST pass callerAccountID(c).
func (s *PortfolioService) GetPortfolio(ctx context.Context, id, accountID uint) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, id, accountID)
	if err != nil || portfolio.WorkflowState == models.PortfolioDeleted {
		return nil, errors.New("portfolio not found")
	}
	return portfolio, nil
}

func (s *PortfolioService) GetBySlug(ctx context.Context, slug string) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindBySlug(ctx, slug)
	if err != nil || portfolio.WorkflowState == models.PortfolioDeleted {
		return nil, errors.New("portfolio not found")
	}
	return portfolio, nil
}

// SetUserRepository wires the user lookup used to show owner/commenter
// names. Optional.
func (s *PortfolioService) SetUserRepository(repo repository.UserRepository) {
	s.userRepo = repo
}

// UserNames resolves display names for the given user ids (best effort; an
// empty map when no user repository is wired). Lookups are by id only —
// callers pass ids they already took from tenant-scoped rows.
func (s *PortfolioService) UserNames(ctx context.Context, ids []uint) map[uint]string {
	names := map[uint]string{}
	if s.userRepo == nil || len(ids) == 0 {
		return names
	}
	// accountID=0: ids come from rows already loaded under the caller's
	// tenant scope (or a published public portfolio); only names are read.
	users, err := s.userRepo.FindByIDs(ctx, ids, 0)
	if err != nil {
		return names
	}
	for _, u := range users {
		names[u.ID] = u.Name
	}
	return names
}

// PortfolioUploadURL is the owner/tenant download route of an uploaded
// artifact file (the public view rewrites it to the slug-scoped route).
func PortfolioUploadURL(portfolioID, attachmentID uint) string {
	return fmt.Sprintf("/api/v1/portfolios/%d/files/%d", portfolioID, attachmentID)
}

// PortfolioHasUpload reports whether an artifact of the portfolio was
// created by the upload endpoint from the given attachment. visibleOnly
// (public viewers) ignores artifacts in a hidden section, so a hidden
// section's files cannot be downloaded by guessing their id.
func (s *PortfolioService) PortfolioHasUpload(ctx context.Context, portfolioID, attachmentID uint, visibleOnly bool) bool {
	res, err := s.artifactRepo.ListByPortfolioID(ctx, portfolioID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil || res == nil {
		return false
	}
	hidden := map[uint]bool{}
	if visibleOnly {
		sections, err := s.sectionRepo.ListByPortfolioID(ctx, portfolioID)
		if err != nil {
			return false
		}
		for _, sec := range sections {
			if !sec.IsVisible {
				hidden[sec.ID] = true
			}
		}
	}
	for _, a := range res.Items {
		if a.SourceType != "upload" || a.SourceID == nil || *a.SourceID != attachmentID {
			continue
		}
		if a.SectionID != nil && hidden[*a.SectionID] {
			continue
		}
		return true
	}
	return false
}

func (s *PortfolioService) GetByPublicURL(ctx context.Context, url string) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindByPublicURL(ctx, url)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}
	return portfolio, nil
}

func (s *PortfolioService) UpdatePortfolio(ctx context.Context, portfolio *models.Portfolio, accountID uint) error {
	_, err := s.portfolioRepo.FindByID(ctx, portfolio.ID, accountID)
	if err != nil {
		return errors.New("portfolio not found")
	}
	return s.portfolioRepo.Update(ctx, portfolio)
}

func (s *PortfolioService) PublishPortfolio(ctx context.Context, id, accountID uint) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}

	portfolio.WorkflowState = "published"
	portfolio.IsPublic = true
	if portfolio.PublicURL == nil || *portfolio.PublicURL == "" {
		u := s.GeneratePublicURL(portfolio)
		portfolio.PublicURL = &u
	}

	if err := s.portfolioRepo.Update(ctx, portfolio); err != nil {
		return nil, err
	}
	return portfolio, nil
}

// UnpublishPortfolio takes a published portfolio back to draft and hides it
// from the public URL (B197: DELETE /portfolios/:id/publish had no route).
func (s *PortfolioService) UnpublishPortfolio(ctx context.Context, id, accountID uint) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}
	portfolio.WorkflowState = models.PortfolioDraft
	portfolio.IsPublic = false
	if err := s.portfolioRepo.Update(ctx, portfolio); err != nil {
		return nil, err
	}
	return portfolio, nil
}

func (s *PortfolioService) ArchivePortfolio(ctx context.Context, id, accountID uint) error {
	portfolio, err := s.portfolioRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return errors.New("portfolio not found")
	}

	portfolio.WorkflowState = models.PortfolioArchived
	portfolio.IsPublic = false
	return s.portfolioRepo.Update(ctx, portfolio)
}

// DeletePortfolio soft-deletes the portfolio (workflow_state=deleted), which
// removes it from the owner's list and from the public URL.
func (s *PortfolioService) DeletePortfolio(ctx context.Context, id, accountID uint) error {
	if _, err := s.portfolioRepo.FindByID(ctx, id, accountID); err != nil {
		return errors.New("portfolio not found")
	}
	return s.portfolioRepo.Delete(ctx, id)
}

// SetWorkflowState applies an owner-requested state change through
// PUT /portfolios/:id. Only draft <-> archived is accepted here; publishing
// goes through PublishPortfolio so the public URL is generated.
func (s *PortfolioService) SetWorkflowState(portfolio *models.Portfolio, state string) error {
	switch state {
	case models.PortfolioArchived:
		portfolio.WorkflowState = models.PortfolioArchived
		portfolio.IsPublic = false
	case models.PortfolioDraft:
		portfolio.WorkflowState = models.PortfolioDraft
		portfolio.IsPublic = false
	case models.PortfolioPublished:
		if portfolio.WorkflowState != models.PortfolioPublished {
			return errors.New("use POST /portfolios/:id/publish to publish")
		}
	default:
		return fmt.Errorf("invalid workflow_state %q", state)
	}
	return nil
}

// GetPortfolioDetail returns the portfolio's sections, artifacts and each
// artifact's reflections (B098: the editor needs them on load and after
// every save).
func (s *PortfolioService) GetPortfolioDetail(ctx context.Context, portfolioID uint) ([]models.PortfolioSection, []models.PortfolioArtifact, map[uint][]models.PortfolioReflection) {
	sections, _ := s.sectionRepo.ListByPortfolioID(ctx, portfolioID)
	var artifacts []models.PortfolioArtifact
	if res, err := s.artifactRepo.ListByPortfolioID(ctx, portfolioID, repository.PaginationParams{Page: 1, PerPage: 1000}); err == nil && res != nil {
		artifacts = res.Items
	}
	reflections := map[uint][]models.PortfolioReflection{}
	if s.reflectionRepo != nil {
		for _, a := range artifacts {
			if rs, err := s.reflectionRepo.ListByArtifactID(ctx, a.ID); err == nil && len(rs) > 0 {
				reflections[a.ID] = rs
			}
		}
	}
	return sections, artifacts, reflections
}

// DuplicatePortfolio copies a portfolio with its sections and artifacts into
// a new draft owned by the same user.
func (s *PortfolioService) DuplicatePortfolio(ctx context.Context, id, accountID uint) (*models.Portfolio, error) {
	src, err := s.portfolioRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}
	dup := *src
	dup.ID = 0
	dup.Title = src.Title + " (Copy)"
	dup.Slug = generateSlug(dup.Title)
	dup.WorkflowState = models.PortfolioDraft
	dup.IsPublic = false
	dup.PublicURL = nil
	// The copy is private and gets its own sharing link, if any (the
	// token is unique; copying it would also share the copy).
	dup.ShareToken = nil
	dup.CustomDomain = ""
	dup.ViewCount = 0
	dup.LastExportedAt = nil
	dup.CreatedAt = time.Time{}
	dup.UpdatedAt = time.Time{}
	if err := s.portfolioRepo.Create(ctx, &dup); err != nil {
		return nil, err
	}

	sections, artifacts, _ := s.GetPortfolioDetail(ctx, src.ID)
	sectionMap := map[uint]uint{}
	for _, sec := range sections {
		oldID := sec.ID
		sec.ID = 0
		sec.PortfolioID = dup.ID
		sec.CreatedAt, sec.UpdatedAt = time.Time{}, time.Time{}
		if err := s.sectionRepo.Create(ctx, &sec); err != nil {
			return nil, err
		}
		sectionMap[oldID] = sec.ID
	}
	for _, art := range artifacts {
		art.ID = 0
		art.PortfolioID = dup.ID
		if art.SectionID != nil {
			if newID, ok := sectionMap[*art.SectionID]; ok {
				art.SectionID = &newID
			} else {
				art.SectionID = nil
			}
		}
		// An uploaded file's link names its portfolio; point the copy at the
		// duplicate so it keeps working once the original is deleted.
		if art.SourceType == "upload" && art.SourceID != nil {
			art.ContentURL = PortfolioUploadURL(dup.ID, *art.SourceID)
		}
		art.CreatedAt, art.UpdatedAt = time.Time{}, time.Time{}
		if err := s.artifactRepo.Create(ctx, &art); err != nil {
			return nil, err
		}
	}
	return &dup, nil
}

func (s *PortfolioService) ListUserPortfolios(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Portfolio], error) {
	return s.portfolioRepo.ListByUserID(ctx, userID, params)
}

func (s *PortfolioService) ListPublicPortfolios(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Portfolio], error) {
	return s.portfolioRepo.ListPublic(ctx, params)
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

func (s *PortfolioService) AddSection(ctx context.Context, section *models.PortfolioSection) error {
	if section.Title == "" {
		return errors.New("section title is required")
	}
	if section.SectionType == "" {
		return errors.New("section_type is required")
	}
	if section.PortfolioID == 0 {
		return errors.New("portfolio_id is required")
	}
	if section.Layout == "" {
		section.Layout = "standard"
	}

	// Auto-assign position at end
	existing, err := s.sectionRepo.ListByPortfolioID(ctx, section.PortfolioID)
	if err == nil {
		section.Position = len(existing)
	}

	return s.sectionRepo.Create(ctx, section)
}

// ErrPortfolioChildNotFound is returned when a section/artifact does not
// exist, is in another tenant, or belongs to a different portfolio than the
// one in the URL. Handlers map it to 404.
var ErrPortfolioChildNotFound = errors.New("not found in this portfolio")

// GetSection loads a section and verifies it belongs to portfolioID (B096:
// the owner of portfolio 10 could otherwise edit/delete a section of another
// user's portfolio by putting its id in the URL).
func (s *PortfolioService) GetSection(ctx context.Context, portfolioID, sectionID, accountID uint) (*models.PortfolioSection, error) {
	section, err := s.sectionRepo.FindByID(ctx, sectionID, accountID)
	if err != nil || section.PortfolioID != portfolioID {
		return nil, ErrPortfolioChildNotFound
	}
	return section, nil
}

// UpdateSection saves a section that was loaded with GetSection and patched.
// The parent tie is re-checked and the portfolio_id cannot be changed.
func (s *PortfolioService) UpdateSection(ctx context.Context, portfolioID uint, section *models.PortfolioSection, accountID uint) error {
	existing, err := s.GetSection(ctx, portfolioID, section.ID, accountID)
	if err != nil {
		return err
	}
	section.PortfolioID = existing.PortfolioID
	section.CreatedAt = existing.CreatedAt
	if section.Title == "" {
		return errors.New("section title is required")
	}
	if section.SectionType == "" {
		return errors.New("section_type is required")
	}
	return s.sectionRepo.Update(ctx, section)
}

func (s *PortfolioService) RemoveSection(ctx context.Context, portfolioID, id, accountID uint) error {
	if _, err := s.GetSection(ctx, portfolioID, id, accountID); err != nil {
		return err
	}
	// Unlink (not delete) the section's artifacts.
	if artifacts, err := s.artifactRepo.ListBySectionID(ctx, id); err == nil {
		for i := range artifacts {
			if artifacts[i].PortfolioID != portfolioID {
				continue
			}
			artifacts[i].SectionID = nil
			if err := s.artifactRepo.Update(ctx, &artifacts[i]); err != nil {
				return err
			}
		}
	}
	return s.sectionRepo.Delete(ctx, id)
}

func (s *PortfolioService) ReorderSections(ctx context.Context, portfolioID uint, sectionIDs []uint, accountID uint) error {
	for i, sectionID := range sectionIDs {
		section, err := s.sectionRepo.FindByID(ctx, sectionID, accountID)
		if err != nil {
			return fmt.Errorf("section %d not found", sectionID)
		}
		if section.PortfolioID != portfolioID {
			return fmt.Errorf("section %d does not belong to portfolio %d", sectionID, portfolioID)
		}
		section.Position = i
		if err := s.sectionRepo.Update(ctx, section); err != nil {
			return err
		}
	}
	return nil
}

func (s *PortfolioService) ListSections(ctx context.Context, portfolioID uint) ([]models.PortfolioSection, error) {
	return s.sectionRepo.ListByPortfolioID(ctx, portfolioID)
}

// ---------------------------------------------------------------------------
// Artifacts
// ---------------------------------------------------------------------------

func (s *PortfolioService) AddArtifact(ctx context.Context, artifact *models.PortfolioArtifact, accountID uint) error {
	if artifact.Title == "" {
		return errors.New("artifact title is required")
	}
	if artifact.ArtifactType == "" {
		return errors.New("artifact_type is required")
	}
	if artifact.PortfolioID == 0 {
		return errors.New("portfolio_id is required")
	}
	if artifact.SectionID != nil {
		if _, err := s.GetSection(ctx, artifact.PortfolioID, *artifact.SectionID, accountID); err != nil {
			return errors.New("section not found in this portfolio")
		}
	}
	if artifact.Position == 0 {
		if existing, err := s.artifactRepo.ListByPortfolioID(ctx, artifact.PortfolioID, repository.PaginationParams{Page: 1, PerPage: 1}); err == nil && existing != nil {
			artifact.Position = int(existing.TotalCount)
		}
	}
	return s.artifactRepo.Create(ctx, artifact)
}

// GetArtifact loads an artifact and verifies it belongs to portfolioID (B096).
func (s *PortfolioService) GetArtifact(ctx context.Context, portfolioID, artifactID, accountID uint) (*models.PortfolioArtifact, error) {
	artifact, err := s.artifactRepo.FindByID(ctx, artifactID, accountID)
	if err != nil || artifact.PortfolioID != portfolioID {
		return nil, ErrPortfolioChildNotFound
	}
	return artifact, nil
}

// UpdateArtifact saves an artifact that was loaded with GetArtifact and
// patched; the parent tie is re-checked and a new section must belong to the
// same portfolio.
func (s *PortfolioService) UpdateArtifact(ctx context.Context, portfolioID uint, artifact *models.PortfolioArtifact, accountID uint) error {
	existing, err := s.GetArtifact(ctx, portfolioID, artifact.ID, accountID)
	if err != nil {
		return err
	}
	artifact.PortfolioID = existing.PortfolioID
	artifact.CreatedAt = existing.CreatedAt
	if artifact.SectionID != nil {
		if _, err := s.GetSection(ctx, portfolioID, *artifact.SectionID, accountID); err != nil {
			return errors.New("section not found in this portfolio")
		}
	}
	return s.artifactRepo.Update(ctx, artifact)
}

func (s *PortfolioService) RemoveArtifact(ctx context.Context, portfolioID, id, accountID uint) error {
	if _, err := s.GetArtifact(ctx, portfolioID, id, accountID); err != nil {
		return err
	}
	return s.artifactRepo.Delete(ctx, id)
}

func (s *PortfolioService) ListArtifacts(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioArtifact], error) {
	return s.artifactRepo.ListByPortfolioID(ctx, portfolioID, params)
}

// ImportFromCourse turns the portfolio owner's course submissions into
// course_work artifacts. With no submissionIDs every submitted submission of
// the owner in courseID is imported (the "Import from course" button sends
// only a course id — B198). Submissions that belong to anyone other than the
// portfolio owner, or to a different course than courseID (when given), are
// skipped so a student cannot copy a classmate's work into their portfolio.
func (s *PortfolioService) ImportFromCourse(ctx context.Context, portfolioID uint, courseID uint, submissionIDs []uint, accountID uint) ([]models.PortfolioArtifact, error) {
	return s.ImportSubmissions(ctx, portfolioID, courseID, submissionIDs, nil, accountID)
}

// ImportSubmissions is ImportFromCourse into an optional section (G155).
// Like Canvas "add a submission to an ePortfolio" it carries the work over:
// a course_work artifact with the submission's text or link, plus one file
// artifact per uploaded file (linked through the portfolio's file route —
// submission uploads are the student's own User files). A submission that
// is already in the portfolio is not imported twice.
func (s *PortfolioService) ImportSubmissions(ctx context.Context, portfolioID uint, courseID uint, submissionIDs []uint, sectionID *uint, accountID uint) ([]models.PortfolioArtifact, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, portfolioID, accountID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}
	if sectionID != nil {
		if _, err := s.GetSection(ctx, portfolio.ID, *sectionID, accountID); err != nil {
			return nil, ErrPortfolioChildNotFound
		}
	}
	alreadyImported := s.importedSubmissionIDs(ctx, portfolio.ID)

	var submissions []models.Submission
	if len(submissionIDs) == 0 {
		if courseID == 0 {
			return nil, errors.New("course_id or submission_ids is required")
		}
		subs, err := s.submissionRepo.ListByUserAndCourse(ctx, portfolio.UserID, courseID)
		if err != nil {
			return nil, err
		}
		for _, sub := range subs {
			if sub.SubmittedAt != nil {
				submissions = append(submissions, sub)
			}
		}
	} else {
		for _, submissionID := range submissionIDs {
			sub, err := s.submissionRepo.FindByID(ctx, submissionID, accountID)
			if err != nil {
				continue // skip submissions that can't be found
			}
			submissions = append(submissions, *sub)
		}
	}

	imported := []models.PortfolioArtifact{}

	for i := range submissions {
		submission := &submissions[i]
		if submission.UserID != portfolio.UserID || alreadyImported[submission.ID] {
			continue
		}
		submissionID := submission.ID

		// Look up the assignment name for the artifact title
		artifactTitle := fmt.Sprintf("Submission #%d", submissionID)
		sourceCourseID := courseID
		assignment, assignErr := s.assignmentRepo.FindByID(ctx, submission.AssignmentID, accountID)
		if assignErr == nil {
			if courseID != 0 && assignment.CourseID != courseID {
				continue
			}
			artifactTitle = assignment.Name
			sourceCourseID = assignment.CourseID
		} else if courseID == 0 {
			continue
		}

		sourceID := submissionID
		artifact := models.PortfolioArtifact{
			PortfolioID:    portfolio.ID,
			SectionID:      sectionID,
			Title:          artifactTitle,
			ArtifactType:   "course_work",
			SourceType:     "course_submission",
			SourceCourseID: &sourceCourseID,
			SourceID:       &sourceID,
		}

		// Copy submission URL / text body if available. The link is shown
		// as an href on the public page, so only web links are kept.
		if submission.URL != nil && isWebURL(*submission.URL) {
			artifact.ContentURL = *submission.URL
		}
		if submission.Body != nil {
			// Artifact descriptions are rendered as text (B005): keep the
			// submission's words, drop its markup.
			artifact.Description = SanitizePlainText(*submission.Body)
		}

		if err := s.artifactRepo.Create(ctx, &artifact); err != nil {
			continue
		}

		imported = append(imported, artifact)

		for _, f := range submissionFiles(submission) {
			attachmentID := f.ID
			fileArtifact := models.PortfolioArtifact{
				PortfolioID:    portfolio.ID,
				SectionID:      sectionID,
				Title:          artifactTitle + " — " + f.name(),
				ArtifactType:   artifactTypeForMIME(f.ContentType),
				ContentURL:     PortfolioUploadURL(portfolio.ID, attachmentID),
				SourceType:     "upload",
				SourceCourseID: &sourceCourseID,
				SourceID:       &attachmentID,
				FileType:       fileExt(f.name()),
				FileSizeBytes:  f.Size,
			}
			if err := s.artifactRepo.Create(ctx, &fileArtifact); err == nil {
				imported = append(imported, fileArtifact)
			}
		}
	}

	return imported, nil
}

// importedSubmissionIDs lists submissions already imported into the
// portfolio (course_submission artifacts).
func (s *PortfolioService) importedSubmissionIDs(ctx context.Context, portfolioID uint) map[uint]bool {
	out := map[uint]bool{}
	res, err := s.artifactRepo.ListByPortfolioID(ctx, portfolioID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil || res == nil {
		return out
	}
	for _, a := range res.Items {
		if a.SourceType == "course_submission" && a.SourceID != nil {
			out[*a.SourceID] = true
		}
	}
	return out
}

// submissionFile is one entry of Submission.Attachments (JSON written by
// the submission handler).
type submissionFile struct {
	ID          uint   `json:"id"`
	DisplayName string `json:"display_name"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

func (f submissionFile) name() string {
	if f.DisplayName != "" {
		return f.DisplayName
	}
	if f.Filename != "" {
		return f.Filename
	}
	return fmt.Sprintf("file %d", f.ID)
}

func submissionFiles(sub *models.Submission) []submissionFile {
	if sub == nil || sub.Attachments == nil || strings.TrimSpace(*sub.Attachments) == "" {
		return nil
	}
	var files []submissionFile
	if err := json.Unmarshal([]byte(*sub.Attachments), &files); err != nil {
		return nil
	}
	out := files[:0]
	for _, f := range files {
		if f.ID != 0 {
			out = append(out, f)
		}
	}
	return out
}

func artifactTypeForMIME(contentType string) string {
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	}
	return "document"
}

func fileExt(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 && i < len(name)-1 {
		return strings.ToLower(name[i+1:])
	}
	return ""
}

func isWebURL(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// ---------------------------------------------------------------------------
// Reflections
// ---------------------------------------------------------------------------

func (s *PortfolioService) AddReflection(ctx context.Context, portfolioID uint, reflection *models.PortfolioReflection, accountID uint) error {
	if reflection.Content == "" {
		return errors.New("reflection content is required")
	}
	if reflection.ArtifactID == 0 {
		return errors.New("artifact_id is required")
	}
	if reflection.UserID == 0 {
		return errors.New("user_id is required")
	}

	if _, err := s.GetArtifact(ctx, portfolioID, reflection.ArtifactID, accountID); err != nil {
		return err
	}

	return s.reflectionRepo.Create(ctx, reflection)
}

func (s *PortfolioService) UpdateReflection(ctx context.Context, reflection *models.PortfolioReflection, accountID uint) error {
	_, err := s.reflectionRepo.FindByID(ctx, reflection.ID, accountID)
	if err != nil {
		return errors.New("reflection not found")
	}
	return s.reflectionRepo.Update(ctx, reflection)
}

func (s *PortfolioService) ListReflections(ctx context.Context, artifactID uint) ([]models.PortfolioReflection, error) {
	return s.reflectionRepo.ListByArtifactID(ctx, artifactID)
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

func (s *PortfolioService) AddComment(ctx context.Context, comment *models.PortfolioComment, accountID uint) error {
	comment.Content = strings.TrimSpace(comment.Content)
	if comment.Content == "" {
		return errors.New("comment content is required")
	}
	if comment.PortfolioID == 0 {
		return errors.New("portfolio_id is required")
	}
	if comment.UserID == 0 {
		return errors.New("user_id is required")
	}
	if comment.SectionID != nil {
		if _, err := s.GetSection(ctx, comment.PortfolioID, *comment.SectionID, accountID); err != nil {
			return errors.New("section not found in this portfolio")
		}
	}
	if comment.ArtifactID != nil {
		if _, err := s.GetArtifact(ctx, comment.PortfolioID, *comment.ArtifactID, accountID); err != nil {
			return errors.New("artifact not found in this portfolio")
		}
	}
	if comment.ParentID != nil {
		parent, err := s.commentRepo.FindByID(ctx, *comment.ParentID, accountID)
		if err != nil || parent.PortfolioID != comment.PortfolioID {
			return errors.New("parent comment not found in this portfolio")
		}
	}
	return s.commentRepo.Create(ctx, comment)
}

func (s *PortfolioService) ListComments(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioComment], error) {
	return s.commentRepo.ListByPortfolioID(ctx, portfolioID, params)
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func (s *PortfolioService) CreateFromTemplate(ctx context.Context, templateID, accountID uint, userID uint) (*models.Portfolio, error) {
	tmpl, err := s.templateRepo.FindByID(ctx, templateID, accountID)
	if err != nil {
		return nil, errors.New("template not found")
	}

	portfolio := &models.Portfolio{
		UserID:        userID,
		Title:         tmpl.Name,
		ThemeID:       tmpl.ThemeID,
		Description:   tmpl.Description,
		WorkflowState: "draft",
	}
	portfolio.Slug = generateSlug(portfolio.Title)

	if err := s.portfolioRepo.Create(ctx, portfolio); err != nil {
		return nil, err
	}

	// Parse sections from template JSON and create them
	if tmpl.Sections != "" {
		var sectionDefs []struct {
			Title       string `json:"title"`
			SectionType string `json:"section_type"`
			Content     string `json:"content"`
			Layout      string `json:"layout"`
		}

		if err := json.Unmarshal([]byte(tmpl.Sections), &sectionDefs); err == nil {
			for i, sd := range sectionDefs {
				section := &models.PortfolioSection{
					PortfolioID: portfolio.ID,
					Title:       sd.Title,
					SectionType: sd.SectionType,
					Content:     sd.Content,
					Layout:      sd.Layout,
					Position:    i,
					IsVisible:   true,
				}
				if section.Layout == "" {
					section.Layout = "standard"
				}
				_ = s.sectionRepo.Create(ctx, section) // best-effort
			}
		}
	}

	// Increment template usage count
	tmpl.UsageCount++
	_ = s.templateRepo.Update(ctx, tmpl)

	return portfolio, nil
}

func (s *PortfolioService) ListTemplates(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioTemplate], error) {
	return s.templateRepo.ListPublic(ctx, params)
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

func (s *PortfolioService) ExportAsStaticSite(ctx context.Context, portfolioID, accountID uint) ([]byte, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, portfolioID, accountID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}

	sections, _ := s.sectionRepo.ListByPortfolioID(ctx, portfolioID)
	allArtifacts, _ := s.artifactRepo.ListByPortfolioID(ctx, portfolioID, repository.PaginationParams{Page: 1, PerPage: 1000})

	var artifacts []models.PortfolioArtifact
	if allArtifacts != nil {
		artifacts = allArtifacts.Items
	}

	// Generate HTML
	html := s.generateStaticHTML(portfolio, sections, artifacts)
	css := s.getThemeCSS(portfolio.ThemeID, portfolio.CustomCSS)
	readme := s.generateREADME(portfolio)

	// Create ZIP
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// index.html
	indexFile, _ := zw.Create("index.html")
	indexFile.Write([]byte(html))

	// style.css
	cssFile, _ := zw.Create("css/style.css")
	cssFile.Write([]byte(css))

	// README.md
	readmeFile, _ := zw.Create("README.md")
	readmeFile.Write([]byte(readme))

	zw.Close()

	// Update last exported time
	now := time.Now()
	portfolio.LastExportedAt = &now
	_ = s.portfolioRepo.Update(ctx, portfolio)

	return buf.Bytes(), nil
}

func (s *PortfolioService) ExportAsPDF(ctx context.Context, portfolioID, accountID uint) ([]byte, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, portfolioID, accountID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}

	sections, _ := s.sectionRepo.ListByPortfolioID(ctx, portfolioID)
	allArtifacts, _ := s.artifactRepo.ListByPortfolioID(ctx, portfolioID, repository.PaginationParams{Page: 1, PerPage: 1000})

	var artifacts []models.PortfolioArtifact
	if allArtifacts != nil {
		artifacts = allArtifacts.Items
	}

	// B200: return a real PDF document (application/pdf), not print HTML.
	return renderPDF(portfolio.Title, s.portfolioPDFLines(portfolio, sections, artifacts)), nil
}

// ---------------------------------------------------------------------------
// View tracking
// ---------------------------------------------------------------------------

func (s *PortfolioService) RecordView(ctx context.Context, portfolioID uint) error {
	return s.portfolioRepo.IncrementViewCount(ctx, portfolioID)
}

// ---------------------------------------------------------------------------
// Public URL
// ---------------------------------------------------------------------------

func (s *PortfolioService) GeneratePublicURL(portfolio *models.Portfolio) string {
	// Matches the routed public page (/portfolios/public/:slug — B196).
	return fmt.Sprintf("portfolios/public/%s", portfolio.Slug)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	// Replace non-alphanumeric characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "portfolio"
	}
	// Append timestamp suffix for uniqueness
	slug = fmt.Sprintf("%s-%d", slug, time.Now().UnixMilli())
	return slug
}

func (s *PortfolioService) generateStaticHTML(portfolio *models.Portfolio, sections []models.PortfolioSection, artifacts []models.PortfolioArtifact) string {
	// Build sections HTML
	var sectionsHTML strings.Builder
	for _, sec := range sections {
		if !sec.IsVisible {
			continue
		}
		sectionsHTML.WriteString(fmt.Sprintf(`    <section class="portfolio-section section-%s layout-%s" id="section-%d">
      <h2>%s</h2>
      <div class="section-content">%s</div>
`, sec.SectionType, sec.Layout, sec.ID, escapeHTML(sec.Title), sec.Content))

		// Render artifacts that belong to this section
		for _, art := range artifacts {
			if art.SectionID != nil && *art.SectionID == sec.ID {
				sectionsHTML.WriteString(s.renderArtifactHTML(&art))
			}
		}

		sectionsHTML.WriteString("    </section>\n")
	}

	// Render un-sectioned artifacts
	var unsectionedHTML strings.Builder
	for _, art := range artifacts {
		if art.SectionID == nil {
			unsectionedHTML.WriteString(s.renderArtifactHTML(&art))
		}
	}
	if unsectionedHTML.Len() > 0 {
		sectionsHTML.WriteString(fmt.Sprintf(`    <section class="portfolio-section section-artifacts">
      <h2>Portfolio Artifacts</h2>
      <div class="section-content">%s</div>
    </section>
`, unsectionedHTML.String()))
	}

	// Avatar / header
	avatarHTML := ""
	if portfolio.AvatarURL != "" {
		avatarHTML = fmt.Sprintf(`      <img src="%s" alt="Avatar" class="avatar" />`, escapeHTML(portfolio.AvatarURL))
	}
	headerImageHTML := ""
	if portfolio.HeaderImageURL != "" {
		headerImageHTML = fmt.Sprintf(`  <div class="header-image"><img src="%s" alt="Header" /></div>`, escapeHTML(portfolio.HeaderImageURL))
	}

	taglineHTML := ""
	if portfolio.Tagline != "" {
		taglineHTML = fmt.Sprintf(`      <p class="tagline">%s</p>`, escapeHTML(portfolio.Tagline))
	}

	contactHTML := s.generateContactHTML(portfolio)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>%s</title>
  <meta name="description" content="%s" />
  <link rel="stylesheet" href="css/style.css" />
</head>
<body class="theme-%s">
%s
  <header class="portfolio-header">
    <div class="header-content">
%s
      <h1>%s</h1>
%s
%s
    </div>
  </header>

  <main class="portfolio-main">
%s
  </main>

  <footer class="portfolio-footer">
    <p>Built with Paper LMS Portfolio</p>
  </footer>
</body>
</html>
`, escapeHTML(portfolio.Title),
		escapeHTML(portfolio.Description),
		escapeHTML(portfolio.ThemeID),
		headerImageHTML,
		avatarHTML,
		escapeHTML(portfolio.Title),
		taglineHTML,
		contactHTML,
		sectionsHTML.String(),
	)
}

func (s *PortfolioService) renderArtifactHTML(art *models.PortfolioArtifact) string {
	var b strings.Builder

	featuredClass := ""
	if art.IsFeatured {
		featuredClass = " featured"
	}

	b.WriteString(fmt.Sprintf(`      <article class="artifact artifact-%s%s">
        <h3>%s</h3>
`, escapeHTML(art.ArtifactType), featuredClass, escapeHTML(art.Title)))

	if art.ThumbnailURL != "" {
		b.WriteString(fmt.Sprintf(`        <img src="%s" alt="%s" class="artifact-thumbnail" />
`, escapeHTML(art.ThumbnailURL), escapeHTML(art.Title)))
	}

	if art.Description != "" {
		b.WriteString(fmt.Sprintf(`        <p class="artifact-description">%s</p>
`, escapeHTML(art.Description)))
	}

	if art.ContentURL != "" {
		b.WriteString(fmt.Sprintf(`        <a href="%s" class="artifact-link" target="_blank" rel="noopener noreferrer">View</a>
`, escapeHTML(art.ContentURL)))
	}

	// Render tags
	if art.Tags != "" {
		var tags []string
		if json.Unmarshal([]byte(art.Tags), &tags) == nil && len(tags) > 0 {
			b.WriteString(`        <div class="artifact-tags">`)
			for _, tag := range tags {
				b.WriteString(fmt.Sprintf(`<span class="tag">%s</span>`, escapeHTML(tag)))
			}
			b.WriteString("</div>\n")
		}
	}

	b.WriteString("      </article>\n")
	return b.String()
}

func (s *PortfolioService) generateContactHTML(portfolio *models.Portfolio) string {
	var links []string
	if portfolio.ContactEmail != "" {
		links = append(links, fmt.Sprintf(`<a href="mailto:%s">Email</a>`, escapeHTML(portfolio.ContactEmail)))
	}
	if portfolio.LinkedInURL != "" {
		links = append(links, fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">LinkedIn</a>`, escapeHTML(portfolio.LinkedInURL)))
	}
	if portfolio.WebsiteURL != "" {
		links = append(links, fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">Website</a>`, escapeHTML(portfolio.WebsiteURL)))
	}
	if len(links) == 0 {
		return ""
	}
	return fmt.Sprintf(`      <nav class="contact-links">%s</nav>`, strings.Join(links, " | "))
}

func (s *PortfolioService) getThemeCSS(themeID string, customCSS string) string {
	// Base reset and responsive CSS
	base := `/* Paper LMS Portfolio - Auto-generated Theme */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html { font-size: 16px; scroll-behavior: smooth; }
body { min-height: 100vh; display: flex; flex-direction: column; }
img { max-width: 100%; height: auto; display: block; }
a { color: inherit; }

.portfolio-header { text-align: center; padding: 3rem 1.5rem; }
.header-content { max-width: 800px; margin: 0 auto; }
.header-image img { width: 100%; max-height: 300px; object-fit: cover; }
.avatar { width: 120px; height: 120px; border-radius: 50%; margin: 0 auto 1rem; object-fit: cover; }
.tagline { font-style: italic; margin-top: 0.5rem; }
.contact-links { margin-top: 1rem; }
.contact-links a { margin: 0 0.5rem; }

.portfolio-main { flex: 1; max-width: 1100px; margin: 0 auto; padding: 2rem 1.5rem; width: 100%; }
.portfolio-section { margin-bottom: 3rem; }
.portfolio-section h2 { margin-bottom: 1rem; }

.artifact { border: 1px solid #e0e0e0; border-radius: 8px; padding: 1.5rem; margin-bottom: 1.5rem; }
.artifact.featured { border-color: #f0c040; border-width: 2px; }
.artifact h3 { margin-bottom: 0.5rem; }
.artifact-thumbnail { max-width: 300px; border-radius: 4px; margin: 0.5rem 0; }
.artifact-description { margin: 0.5rem 0; }
.artifact-link { display: inline-block; margin-top: 0.5rem; font-weight: 600; }
.artifact-tags { margin-top: 0.75rem; }
.tag { display: inline-block; background: #f0f0f0; padding: 0.2rem 0.6rem; border-radius: 12px; font-size: 0.85rem; margin-right: 0.4rem; margin-bottom: 0.3rem; }

.layout-grid .section-content { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1.5rem; }
.layout-two-column .section-content { display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem; }
.layout-masonry .section-content { column-count: 2; column-gap: 1.5rem; }
.layout-masonry .section-content > * { break-inside: avoid; margin-bottom: 1.5rem; }
.layout-timeline .section-content { border-left: 3px solid #ccc; padding-left: 1.5rem; }

.portfolio-footer { text-align: center; padding: 2rem 1.5rem; font-size: 0.9rem; }

@media (max-width: 768px) {
  .layout-two-column .section-content { grid-template-columns: 1fr; }
  .layout-masonry .section-content { column-count: 1; }
  .portfolio-header { padding: 2rem 1rem; }
  .portfolio-main { padding: 1rem; }
}
`

	// Theme-specific styles
	var theme string
	switch themeID {
	case "creative-bold":
		theme = `
body.theme-creative-bold { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: #1a1a2e; color: #eee; }
body.theme-creative-bold .portfolio-header { background: linear-gradient(135deg, #e94560, #0f3460); color: #fff; }
body.theme-creative-bold h2 { color: #e94560; }
body.theme-creative-bold .artifact { background: #16213e; border-color: #0f3460; }
body.theme-creative-bold .tag { background: #0f3460; color: #eee; }
body.theme-creative-bold a { color: #e94560; }
body.theme-creative-bold .portfolio-footer { background: #16213e; color: #999; }
`
	case "academic-classic":
		theme = `
body.theme-academic-classic { font-family: Georgia, 'Times New Roman', serif; background: #faf8f5; color: #333; }
body.theme-academic-classic .portfolio-header { background: #2c3e50; color: #fff; }
body.theme-academic-classic h2 { color: #2c3e50; border-bottom: 1px solid #bdc3c7; padding-bottom: 0.3rem; }
body.theme-academic-classic .artifact { background: #fff; border-color: #ddd; }
body.theme-academic-classic .tag { background: #ecf0f1; color: #2c3e50; }
body.theme-academic-classic a { color: #2980b9; }
body.theme-academic-classic .portfolio-footer { background: #2c3e50; color: #bdc3c7; }
`
	case "minimal-dark":
		theme = `
body.theme-minimal-dark { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #121212; color: #e0e0e0; }
body.theme-minimal-dark .portfolio-header { background: #1e1e1e; border-bottom: 1px solid #333; }
body.theme-minimal-dark h2 { color: #bb86fc; }
body.theme-minimal-dark .artifact { background: #1e1e1e; border-color: #333; }
body.theme-minimal-dark .tag { background: #333; color: #e0e0e0; }
body.theme-minimal-dark a { color: #bb86fc; }
body.theme-minimal-dark .portfolio-footer { background: #1e1e1e; color: #666; }
`
	case "portfolio-developer":
		theme = `
body.theme-portfolio-developer { font-family: 'Fira Code', 'Courier New', monospace; background: #0d1117; color: #c9d1d9; }
body.theme-portfolio-developer .portfolio-header { background: #161b22; border-bottom: 1px solid #30363d; }
body.theme-portfolio-developer h2 { color: #58a6ff; }
body.theme-portfolio-developer .artifact { background: #161b22; border-color: #30363d; }
body.theme-portfolio-developer .tag { background: #21262d; color: #58a6ff; font-family: inherit; }
body.theme-portfolio-developer a { color: #58a6ff; }
body.theme-portfolio-developer .artifact.featured { border-color: #f0883e; }
body.theme-portfolio-developer .portfolio-footer { background: #161b22; color: #484f58; }
`
	default: // clean-modern
		theme = `
body.theme-clean-modern { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #ffffff; color: #333; }
body.theme-clean-modern .portfolio-header { background: #f7f7f7; border-bottom: 1px solid #e5e5e5; }
body.theme-clean-modern h2 { color: #1a73e8; }
body.theme-clean-modern .artifact { background: #fff; }
body.theme-clean-modern a { color: #1a73e8; }
body.theme-clean-modern .portfolio-footer { background: #f7f7f7; color: #999; border-top: 1px solid #e5e5e5; }
`
	}

	result := base + theme
	if customCSS != "" {
		result += "\n/* Custom CSS */\n" + customCSS
	}
	return result
}

func (s *PortfolioService) generateREADME(portfolio *models.Portfolio) string {
	return fmt.Sprintf(`# %s

%s

## Deploying Your Portfolio

This portfolio is a self-contained static website. You can host it anywhere that serves HTML files.

### GitHub Pages
1. Create a new GitHub repository
2. Upload all files from this ZIP to the repository
3. Go to Settings > Pages > Select "main" branch > Save
4. Your portfolio will be live at https://<username>.github.io/<repo-name>/

### Netlify
1. Go to https://app.netlify.com/drop
2. Drag and drop this entire folder
3. Your portfolio will be live instantly with a Netlify URL

### Vercel
1. Install the Vercel CLI: npm i -g vercel
2. Run "vercel" in this directory
3. Follow the prompts to deploy

### Any Web Host
Upload the contents of this ZIP to your web host's public directory (public_html, www, etc.).

## Files
- index.html — Your portfolio page
- css/style.css — Theme and layout styles
- README.md — This file

## Customization
Edit css/style.css to modify colors, fonts, and layout.
Edit index.html to change content directly.

---
*Exported from Paper LMS Portfolio*
`, escapeHTML(portfolio.Title), escapeHTML(portfolio.Description))
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
