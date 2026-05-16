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

func (s *PortfolioService) GetPortfolio(ctx context.Context, id uint) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}
	return portfolio, nil
}

func (s *PortfolioService) GetBySlug(ctx context.Context, slug string) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}
	return portfolio, nil
}

func (s *PortfolioService) GetByPublicURL(ctx context.Context, url string) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindByPublicURL(ctx, url)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}
	return portfolio, nil
}

func (s *PortfolioService) UpdatePortfolio(ctx context.Context, portfolio *models.Portfolio) error {
	_, err := s.portfolioRepo.FindByID(ctx, portfolio.ID)
	if err != nil {
		return errors.New("portfolio not found")
	}
	return s.portfolioRepo.Update(ctx, portfolio)
}

func (s *PortfolioService) PublishPortfolio(ctx context.Context, id uint) (*models.Portfolio, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}

	portfolio.WorkflowState = "published"
	portfolio.IsPublic = true
	if portfolio.PublicURL == "" {
		portfolio.PublicURL = s.GeneratePublicURL(portfolio)
	}

	if err := s.portfolioRepo.Update(ctx, portfolio); err != nil {
		return nil, err
	}
	return portfolio, nil
}

func (s *PortfolioService) ArchivePortfolio(ctx context.Context, id uint) error {
	portfolio, err := s.portfolioRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("portfolio not found")
	}

	portfolio.WorkflowState = "archived"
	portfolio.IsPublic = false
	return s.portfolioRepo.Update(ctx, portfolio)
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

func (s *PortfolioService) UpdateSection(ctx context.Context, section *models.PortfolioSection) error {
	_, err := s.sectionRepo.FindByID(ctx, section.ID)
	if err != nil {
		return errors.New("section not found")
	}
	return s.sectionRepo.Update(ctx, section)
}

func (s *PortfolioService) RemoveSection(ctx context.Context, id uint) error {
	_, err := s.sectionRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("section not found")
	}
	return s.sectionRepo.Delete(ctx, id)
}

func (s *PortfolioService) ReorderSections(ctx context.Context, portfolioID uint, sectionIDs []uint) error {
	for i, sectionID := range sectionIDs {
		section, err := s.sectionRepo.FindByID(ctx, sectionID)
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

func (s *PortfolioService) AddArtifact(ctx context.Context, artifact *models.PortfolioArtifact) error {
	if artifact.Title == "" {
		return errors.New("artifact title is required")
	}
	if artifact.ArtifactType == "" {
		return errors.New("artifact_type is required")
	}
	if artifact.PortfolioID == 0 {
		return errors.New("portfolio_id is required")
	}
	return s.artifactRepo.Create(ctx, artifact)
}

func (s *PortfolioService) UpdateArtifact(ctx context.Context, artifact *models.PortfolioArtifact) error {
	_, err := s.artifactRepo.FindByID(ctx, artifact.ID)
	if err != nil {
		return errors.New("artifact not found")
	}
	return s.artifactRepo.Update(ctx, artifact)
}

func (s *PortfolioService) RemoveArtifact(ctx context.Context, id uint) error {
	_, err := s.artifactRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("artifact not found")
	}
	return s.artifactRepo.Delete(ctx, id)
}

func (s *PortfolioService) ListArtifacts(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioArtifact], error) {
	return s.artifactRepo.ListByPortfolioID(ctx, portfolioID, params)
}

func (s *PortfolioService) ImportFromCourse(ctx context.Context, portfolioID uint, courseID uint, submissionIDs []uint) ([]models.PortfolioArtifact, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}

	var imported []models.PortfolioArtifact

	for _, submissionID := range submissionIDs {
		submission, err := s.submissionRepo.FindByID(ctx, submissionID)
		if err != nil {
			continue // skip submissions that can't be found
		}

		// Look up the assignment name for the artifact title
		artifactTitle := fmt.Sprintf("Submission #%d", submissionID)
		assignment, assignErr := s.assignmentRepo.FindByID(ctx, submission.AssignmentID, 0)
		if assignErr == nil {
			artifactTitle = assignment.Name
		}

		sourceCourseID := courseID
		sourceID := submissionID
		artifact := models.PortfolioArtifact{
			PortfolioID:    portfolio.ID,
			Title:          artifactTitle,
			ArtifactType:   "course_work",
			SourceType:     "course_submission",
			SourceCourseID: &sourceCourseID,
			SourceID:       &sourceID,
		}

		// Copy submission URL if available
		if submission.URL != nil {
			artifact.ContentURL = *submission.URL
		}

		if err := s.artifactRepo.Create(ctx, &artifact); err != nil {
			continue
		}

		imported = append(imported, artifact)
	}

	return imported, nil
}

// ---------------------------------------------------------------------------
// Reflections
// ---------------------------------------------------------------------------

func (s *PortfolioService) AddReflection(ctx context.Context, reflection *models.PortfolioReflection) error {
	if reflection.Content == "" {
		return errors.New("reflection content is required")
	}
	if reflection.ArtifactID == 0 {
		return errors.New("artifact_id is required")
	}
	if reflection.UserID == 0 {
		return errors.New("user_id is required")
	}

	_, err := s.artifactRepo.FindByID(ctx, reflection.ArtifactID)
	if err != nil {
		return errors.New("artifact not found")
	}

	return s.reflectionRepo.Create(ctx, reflection)
}

func (s *PortfolioService) UpdateReflection(ctx context.Context, reflection *models.PortfolioReflection) error {
	_, err := s.reflectionRepo.FindByID(ctx, reflection.ID)
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

func (s *PortfolioService) AddComment(ctx context.Context, comment *models.PortfolioComment) error {
	if comment.Content == "" {
		return errors.New("comment content is required")
	}
	if comment.PortfolioID == 0 {
		return errors.New("portfolio_id is required")
	}
	if comment.UserID == 0 {
		return errors.New("user_id is required")
	}
	return s.commentRepo.Create(ctx, comment)
}

func (s *PortfolioService) ListComments(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioComment], error) {
	return s.commentRepo.ListByPortfolioID(ctx, portfolioID, params)
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func (s *PortfolioService) CreateFromTemplate(ctx context.Context, templateID uint, userID uint) (*models.Portfolio, error) {
	tmpl, err := s.templateRepo.FindByID(ctx, templateID)
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

func (s *PortfolioService) ExportAsStaticSite(ctx context.Context, portfolioID uint) ([]byte, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, portfolioID)
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

func (s *PortfolioService) ExportAsPDF(ctx context.Context, portfolioID uint) ([]byte, error) {
	portfolio, err := s.portfolioRepo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, errors.New("portfolio not found")
	}

	sections, _ := s.sectionRepo.ListByPortfolioID(ctx, portfolioID)
	allArtifacts, _ := s.artifactRepo.ListByPortfolioID(ctx, portfolioID, repository.PaginationParams{Page: 1, PerPage: 1000})

	var artifacts []models.PortfolioArtifact
	if allArtifacts != nil {
		artifacts = allArtifacts.Items
	}

	// Generate a print-ready HTML document that can be saved as PDF via browser
	html := s.generatePrintHTML(portfolio, sections, artifacts)

	return []byte(html), nil
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
	return fmt.Sprintf("p/%s-%d", portfolio.Slug, portfolio.ID)
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

func (s *PortfolioService) generatePrintHTML(portfolio *models.Portfolio, sections []models.PortfolioSection, artifacts []models.PortfolioArtifact) string {
	var sectionsHTML strings.Builder
	for _, sec := range sections {
		if !sec.IsVisible {
			continue
		}
		sectionsHTML.WriteString(fmt.Sprintf(`<section class="print-section"><h2>%s</h2><div>%s</div>`, escapeHTML(sec.Title), sec.Content))
		for _, art := range artifacts {
			if art.SectionID != nil && *art.SectionID == sec.ID {
				sectionsHTML.WriteString(fmt.Sprintf(`<div class="print-artifact"><h3>%s</h3><p>%s</p></div>`, escapeHTML(art.Title), escapeHTML(art.Description)))
			}
		}
		sectionsHTML.WriteString(`</section>`)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>%s</title>
  <style>
    @page { margin: 1in; }
    body { font-family: Georgia, 'Times New Roman', serif; color: #222; line-height: 1.6; max-width: 800px; margin: 0 auto; padding: 20px; }
    h1 { font-size: 2em; border-bottom: 2px solid #333; padding-bottom: 0.3em; }
    h2 { font-size: 1.4em; color: #444; margin-top: 1.5em; border-bottom: 1px solid #ccc; padding-bottom: 0.2em; }
    h3 { font-size: 1.1em; color: #555; }
    .print-section { page-break-inside: avoid; margin-bottom: 1.5em; }
    .print-artifact { margin-left: 1em; margin-bottom: 1em; }
    .tagline { font-style: italic; color: #666; }
    @media print { body { max-width: 100%%; padding: 0; } }
  </style>
</head>
<body>
  <h1>%s</h1>
  <p class="tagline">%s</p>
  <p>%s</p>
  %s
</body>
</html>
`, escapeHTML(portfolio.Title),
		escapeHTML(portfolio.Title),
		escapeHTML(portfolio.Tagline),
		escapeHTML(portfolio.Description),
		sectionsHTML.String(),
	)
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
