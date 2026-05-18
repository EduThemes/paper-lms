package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type PortfolioHandler struct {
	portfolioService *service.PortfolioService
	authz            *ResourceAuthorizer
}

func NewPortfolioHandler(portfolioService *service.PortfolioService, authz *ResourceAuthorizer) *PortfolioHandler {
	return &PortfolioHandler{portfolioService: portfolioService, authz: authz}
}

// ---------------------------------------------------------------------------
// JSON serializers
// ---------------------------------------------------------------------------

func portfolioToJSON(p *models.Portfolio) fiber.Map {
	return fiber.Map{
		"id":               p.ID,
		"user_id":          p.UserID,
		"title":            p.Title,
		"slug":             p.Slug,
		"description":      p.Description,
		"theme_id":         p.ThemeID,
		"custom_css":       p.CustomCSS,
		"header_image_url": p.HeaderImageURL,
		"avatar_url":       p.AvatarURL,
		"tagline":          p.Tagline,
		"contact_email":    p.ContactEmail,
		"linkedin_url":     p.LinkedInURL,
		"website_url":      p.WebsiteURL,
		"is_public":        p.IsPublic,
		"public_url":       p.PublicURL,
		"custom_domain":    p.CustomDomain,
		"workflow_state":   p.WorkflowState,
		"view_count":       p.ViewCount,
		"last_exported_at": p.LastExportedAt,
		"created_at":       p.CreatedAt,
		"updated_at":       p.UpdatedAt,
	}
}

func portfolioSectionToJSON(s *models.PortfolioSection) fiber.Map {
	return fiber.Map{
		"id":           s.ID,
		"portfolio_id": s.PortfolioID,
		"title":        s.Title,
		"section_type": s.SectionType,
		"content":      s.Content,
		"position":     s.Position,
		"is_visible":   s.IsVisible,
		"layout":       s.Layout,
		"created_at":   s.CreatedAt,
		"updated_at":   s.UpdatedAt,
	}
}

func portfolioArtifactToJSON(a *models.PortfolioArtifact) fiber.Map {
	return fiber.Map{
		"id":               a.ID,
		"portfolio_id":     a.PortfolioID,
		"section_id":       a.SectionID,
		"title":            a.Title,
		"description":      a.Description,
		"artifact_type":    a.ArtifactType,
		"content_url":      a.ContentURL,
		"thumbnail_url":    a.ThumbnailURL,
		"source_type":      a.SourceType,
		"source_course_id": a.SourceCourseID,
		"source_id":        a.SourceID,
		"file_type":        a.FileType,
		"file_size_bytes":  a.FileSizeBytes,
		"tags":             a.Tags,
		"outcome_ids":      a.OutcomeIDs,
		"position":         a.Position,
		"is_featured":      a.IsFeatured,
		"created_at":       a.CreatedAt,
		"updated_at":       a.UpdatedAt,
	}
}

func portfolioReflectionToJSON(r *models.PortfolioReflection) fiber.Map {
	return fiber.Map{
		"id":          r.ID,
		"artifact_id": r.ArtifactID,
		"user_id":     r.UserID,
		"prompt_text": r.PromptText,
		"content":     r.Content,
		"created_at":  r.CreatedAt,
		"updated_at":  r.UpdatedAt,
	}
}

func portfolioCommentToJSON(c *models.PortfolioComment) fiber.Map {
	return fiber.Map{
		"id":           c.ID,
		"portfolio_id": c.PortfolioID,
		"section_id":   c.SectionID,
		"artifact_id":  c.ArtifactID,
		"user_id":      c.UserID,
		"content":      c.Content,
		"parent_id":    c.ParentID,
		"created_at":   c.CreatedAt,
		"updated_at":   c.UpdatedAt,
	}
}

func portfolioTemplateToJSON(t *models.PortfolioTemplate) fiber.Map {
	return fiber.Map{
		"id":            t.ID,
		"account_id":    t.AccountID,
		"created_by_id": t.CreatedByID,
		"name":          t.Name,
		"description":   t.Description,
		"theme_id":      t.ThemeID,
		"sections":      t.Sections,
		"is_public":     t.IsPublic,
		"usage_count":   t.UsageCount,
		"created_at":    t.CreatedAt,
		"updated_at":    t.UpdatedAt,
	}
}

// ---------------------------------------------------------------------------
// Portfolio CRUD
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) ListUserPortfolios(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.portfolioService.ListUserPortfolios(c.Context(), uint(userID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch portfolios")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	portfolios := make([]fiber.Map, len(result.Items))
	for i, p := range result.Items {
		portfolios[i] = portfolioToJSON(&p)
	}

	return c.JSON(portfolios)
}

func (h *PortfolioHandler) CreatePortfolio(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		Portfolio struct {
			Title          string `json:"title"`
			Description    string `json:"description"`
			ThemeID        string `json:"theme_id"`
			CustomCSS      string `json:"custom_css"`
			HeaderImageURL string `json:"header_image_url"`
			AvatarURL      string `json:"avatar_url"`
			Tagline        string `json:"tagline"`
			ContactEmail   string `json:"contact_email"`
			LinkedInURL    string `json:"linkedin_url"`
			WebsiteURL     string `json:"website_url"`
		} `json:"portfolio"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	portfolio := &models.Portfolio{
		UserID:         uint(userID),
		Title:          input.Portfolio.Title,
		Description:    input.Portfolio.Description,
		ThemeID:        input.Portfolio.ThemeID,
		CustomCSS:      input.Portfolio.CustomCSS,
		HeaderImageURL: input.Portfolio.HeaderImageURL,
		AvatarURL:      input.Portfolio.AvatarURL,
		Tagline:        input.Portfolio.Tagline,
		ContactEmail:   input.Portfolio.ContactEmail,
		LinkedInURL:    input.Portfolio.LinkedInURL,
		WebsiteURL:     input.Portfolio.WebsiteURL,
	}

	if err := h.portfolioService.CreatePortfolio(c.Context(), portfolio); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioToJSON(portfolio))
}

func (h *PortfolioHandler) GetPortfolio(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	// Public portfolios are readable by anyone; otherwise owner or admin only
	if !portfolio.IsPublic {
		if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
			return err
		}
	}

	return c.JSON(portfolioToJSON(portfolio))
}

func (h *PortfolioHandler) UpdatePortfolio(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	var input struct {
		Portfolio struct {
			Title          *string `json:"title"`
			Description    *string `json:"description"`
			ThemeID        *string `json:"theme_id"`
			CustomCSS      *string `json:"custom_css"`
			HeaderImageURL *string `json:"header_image_url"`
			AvatarURL      *string `json:"avatar_url"`
			Tagline        *string `json:"tagline"`
			ContactEmail   *string `json:"contact_email"`
			LinkedInURL    *string `json:"linkedin_url"`
			WebsiteURL     *string `json:"website_url"`
			CustomDomain   *string `json:"custom_domain"`
		} `json:"portfolio"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Portfolio.Title != nil {
		portfolio.Title = *input.Portfolio.Title
	}
	if input.Portfolio.Description != nil {
		portfolio.Description = *input.Portfolio.Description
	}
	if input.Portfolio.ThemeID != nil {
		portfolio.ThemeID = *input.Portfolio.ThemeID
	}
	if input.Portfolio.CustomCSS != nil {
		portfolio.CustomCSS = *input.Portfolio.CustomCSS
	}
	if input.Portfolio.HeaderImageURL != nil {
		portfolio.HeaderImageURL = *input.Portfolio.HeaderImageURL
	}
	if input.Portfolio.AvatarURL != nil {
		portfolio.AvatarURL = *input.Portfolio.AvatarURL
	}
	if input.Portfolio.Tagline != nil {
		portfolio.Tagline = *input.Portfolio.Tagline
	}
	if input.Portfolio.ContactEmail != nil {
		portfolio.ContactEmail = *input.Portfolio.ContactEmail
	}
	if input.Portfolio.LinkedInURL != nil {
		portfolio.LinkedInURL = *input.Portfolio.LinkedInURL
	}
	if input.Portfolio.WebsiteURL != nil {
		portfolio.WebsiteURL = *input.Portfolio.WebsiteURL
	}
	if input.Portfolio.CustomDomain != nil {
		portfolio.CustomDomain = *input.Portfolio.CustomDomain
	}

	if err := h.portfolioService.UpdatePortfolio(c.Context(), portfolio, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not update portfolio")
	}

	return c.JSON(portfolioToJSON(portfolio))
}

func (h *PortfolioHandler) DeletePortfolio(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	if err := h.portfolioService.ArchivePortfolio(c.Context(), uint(portfolioID), callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not archive portfolio")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *PortfolioHandler) PublishPortfolio(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	// Fetch first to check ownership before publishing
	existing, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, existing.UserID); err != nil {
		return err
	}

	portfolio, err := h.portfolioService.PublishPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(portfolioToJSON(portfolio))
}

func (h *PortfolioHandler) GetPublicPortfolio(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	// Public endpoint: no auth, no tenant Locals. Pass accountID=0 so the
	// repo returns the row regardless of owning tenant; the
	// WorkflowState==published gate below is the visibility check.
	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), 0)
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if portfolio.WorkflowState != "published" {
		return responses.NotFound(c, "portfolio")
	}

	// Record the view
	_ = h.portfolioService.RecordView(c.Context(), portfolio.ID)

	// Return portfolio with sections and featured artifacts
	sections, _ := h.portfolioService.ListSections(c.Context(), portfolio.ID)

	sectionsJSON := make([]fiber.Map, len(sections))
	for i, s := range sections {
		sectionsJSON[i] = portfolioSectionToJSON(&s)
	}

	result := portfolioToJSON(portfolio)
	result["sections"] = sectionsJSON

	return c.JSON(result)
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) AddSection(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	var input struct {
		Section struct {
			Title       string `json:"title"`
			SectionType string `json:"section_type"`
			Content     string `json:"content"`
			Layout      string `json:"layout"`
			IsVisible   *bool  `json:"is_visible"`
		} `json:"section"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	isVisible := true
	if input.Section.IsVisible != nil {
		isVisible = *input.Section.IsVisible
	}

	section := &models.PortfolioSection{
		PortfolioID: uint(portfolioID),
		Title:       input.Section.Title,
		SectionType: input.Section.SectionType,
		Content:     input.Section.Content,
		Layout:      input.Section.Layout,
		IsVisible:   isVisible,
	}

	if err := h.portfolioService.AddSection(c.Context(), section); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioSectionToJSON(section))
}

func (h *PortfolioHandler) UpdateSection(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	sectionID, err := c.ParamsInt("section_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid section ID")
	}

	var input struct {
		Section struct {
			Title       *string `json:"title"`
			SectionType *string `json:"section_type"`
			Content     *string `json:"content"`
			Layout      *string `json:"layout"`
			IsVisible   *bool   `json:"is_visible"`
			Position    *int    `json:"position"`
		} `json:"section"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Fetch section to get existing data
	sections, _ := h.portfolioService.ListSections(c.Context(), uint(sectionID))
	var section *models.PortfolioSection
	for _, s := range sections {
		if s.ID == uint(sectionID) {
			section = &s
			break
		}
	}
	// Fallback: direct fetch via service
	if section == nil {
		// Build a minimal section to update
		section = &models.PortfolioSection{ID: uint(sectionID)}
	}

	if input.Section.Title != nil {
		section.Title = *input.Section.Title
	}
	if input.Section.SectionType != nil {
		section.SectionType = *input.Section.SectionType
	}
	if input.Section.Content != nil {
		section.Content = *input.Section.Content
	}
	if input.Section.Layout != nil {
		section.Layout = *input.Section.Layout
	}
	if input.Section.IsVisible != nil {
		section.IsVisible = *input.Section.IsVisible
	}
	if input.Section.Position != nil {
		section.Position = *input.Section.Position
	}

	if err := h.portfolioService.UpdateSection(c.Context(), section, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not update section")
	}

	return c.JSON(portfolioSectionToJSON(section))
}

func (h *PortfolioHandler) DeleteSection(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	sectionID, err := c.ParamsInt("section_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid section ID")
	}

	if err := h.portfolioService.RemoveSection(c.Context(), uint(sectionID), callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not delete section")
	}

	return c.JSON(fiber.Map{"delete": true})
}

func (h *PortfolioHandler) ReorderSections(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	var input struct {
		Order []uint `json:"order"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.Order) == 0 {
		return responses.BadRequest(c, "order array is required")
	}

	if err := h.portfolioService.ReorderSections(c.Context(), uint(portfolioID), input.Order, callerAccountID(c)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"reorder": true})
}

// ---------------------------------------------------------------------------
// Artifacts
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) AddArtifact(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	var input struct {
		Artifact struct {
			SectionID     *uint  `json:"section_id"`
			Title         string `json:"title"`
			Description   string `json:"description"`
			ArtifactType  string `json:"artifact_type"`
			ContentURL    string `json:"content_url"`
			ThumbnailURL  string `json:"thumbnail_url"`
			SourceType    string `json:"source_type"`
			FileType      string `json:"file_type"`
			FileSizeBytes int64  `json:"file_size_bytes"`
			Tags          string `json:"tags"`
			OutcomeIDs    string `json:"outcome_ids"`
			IsFeatured    bool   `json:"is_featured"`
		} `json:"artifact"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	artifact := &models.PortfolioArtifact{
		PortfolioID:   uint(portfolioID),
		SectionID:     input.Artifact.SectionID,
		Title:         input.Artifact.Title,
		Description:   input.Artifact.Description,
		ArtifactType:  input.Artifact.ArtifactType,
		ContentURL:    input.Artifact.ContentURL,
		ThumbnailURL:  input.Artifact.ThumbnailURL,
		SourceType:    input.Artifact.SourceType,
		FileType:      input.Artifact.FileType,
		FileSizeBytes: input.Artifact.FileSizeBytes,
		Tags:          input.Artifact.Tags,
		OutcomeIDs:    input.Artifact.OutcomeIDs,
		IsFeatured:    input.Artifact.IsFeatured,
	}

	if err := h.portfolioService.AddArtifact(c.Context(), artifact); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioArtifactToJSON(artifact))
}

func (h *PortfolioHandler) UpdateArtifact(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	artifactID, err := c.ParamsInt("artifact_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid artifact ID")
	}

	var input struct {
		Artifact struct {
			SectionID     *uint   `json:"section_id"`
			Title         *string `json:"title"`
			Description   *string `json:"description"`
			ArtifactType  *string `json:"artifact_type"`
			ContentURL    *string `json:"content_url"`
			ThumbnailURL  *string `json:"thumbnail_url"`
			FileType      *string `json:"file_type"`
			FileSizeBytes *int64  `json:"file_size_bytes"`
			Tags          *string `json:"tags"`
			OutcomeIDs    *string `json:"outcome_ids"`
			Position      *int    `json:"position"`
			IsFeatured    *bool   `json:"is_featured"`
		} `json:"artifact"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Build artifact with ID for update
	artifact := &models.PortfolioArtifact{ID: uint(artifactID)}

	if input.Artifact.SectionID != nil {
		artifact.SectionID = input.Artifact.SectionID
	}
	if input.Artifact.Title != nil {
		artifact.Title = *input.Artifact.Title
	}
	if input.Artifact.Description != nil {
		artifact.Description = *input.Artifact.Description
	}
	if input.Artifact.ArtifactType != nil {
		artifact.ArtifactType = *input.Artifact.ArtifactType
	}
	if input.Artifact.ContentURL != nil {
		artifact.ContentURL = *input.Artifact.ContentURL
	}
	if input.Artifact.ThumbnailURL != nil {
		artifact.ThumbnailURL = *input.Artifact.ThumbnailURL
	}
	if input.Artifact.FileType != nil {
		artifact.FileType = *input.Artifact.FileType
	}
	if input.Artifact.FileSizeBytes != nil {
		artifact.FileSizeBytes = *input.Artifact.FileSizeBytes
	}
	if input.Artifact.Tags != nil {
		artifact.Tags = *input.Artifact.Tags
	}
	if input.Artifact.OutcomeIDs != nil {
		artifact.OutcomeIDs = *input.Artifact.OutcomeIDs
	}
	if input.Artifact.Position != nil {
		artifact.Position = *input.Artifact.Position
	}
	if input.Artifact.IsFeatured != nil {
		artifact.IsFeatured = *input.Artifact.IsFeatured
	}

	if err := h.portfolioService.UpdateArtifact(c.Context(), artifact, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not update artifact")
	}

	return c.JSON(portfolioArtifactToJSON(artifact))
}

func (h *PortfolioHandler) DeleteArtifact(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	artifactID, err := c.ParamsInt("artifact_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid artifact ID")
	}

	if err := h.portfolioService.RemoveArtifact(c.Context(), uint(artifactID), callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not delete artifact")
	}

	return c.JSON(fiber.Map{"delete": true})
}

// ---------------------------------------------------------------------------
// Reflections
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) AddReflection(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	artifactID, err := c.ParamsInt("artifact_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid artifact ID")
	}

	var input struct {
		Reflection struct {
			PromptText string `json:"prompt_text"`
			Content    string `json:"content"`
			UserID     uint   `json:"user_id"`
		} `json:"reflection"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	reflection := &models.PortfolioReflection{
		ArtifactID: uint(artifactID),
		UserID:     input.Reflection.UserID,
		PromptText: input.Reflection.PromptText,
		Content:    input.Reflection.Content,
	}

	if err := h.portfolioService.AddReflection(c.Context(), reflection, callerAccountID(c)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioReflectionToJSON(reflection))
}

// ---------------------------------------------------------------------------
// Import from course
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) ImportFromCourse(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		SubmissionIDs []uint `json:"submission_ids"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.SubmissionIDs) == 0 {
		return responses.BadRequest(c, "submission_ids is required")
	}

	imported, err := h.portfolioService.ImportFromCourse(c.Context(), uint(portfolioID), uint(courseID), input.SubmissionIDs, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not import from course")
	}

	artifacts := make([]fiber.Map, len(imported))
	for i, a := range imported {
		artifacts[i] = portfolioArtifactToJSON(&a)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"imported_count": len(imported),
		"artifacts":      artifacts,
	})
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) ExportAsHTML(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	zipData, err := h.portfolioService.ExportAsStaticSite(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not export portfolio")
	}

	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", "attachment; filename=portfolio-export.zip")
	return c.Send(zipData)
}

func (h *PortfolioHandler) ExportAsPDF(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	htmlData, err := h.portfolioService.ExportAsPDF(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not export portfolio")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Content-Disposition", "attachment; filename=portfolio.html")
	return c.Send(htmlData)
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) AddComment(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
		return err
	}

	var input struct {
		Comment struct {
			SectionID  *uint  `json:"section_id"`
			ArtifactID *uint  `json:"artifact_id"`
			UserID     uint   `json:"user_id"`
			Content    string `json:"content"`
			ParentID   *uint  `json:"parent_id"`
		} `json:"comment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	comment := &models.PortfolioComment{
		PortfolioID: uint(portfolioID),
		SectionID:   input.Comment.SectionID,
		ArtifactID:  input.Comment.ArtifactID,
		UserID:      input.Comment.UserID,
		Content:     input.Comment.Content,
		ParentID:    input.Comment.ParentID,
	}

	if err := h.portfolioService.AddComment(c.Context(), comment); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioCommentToJSON(comment))
}

func (h *PortfolioHandler) ListComments(c *fiber.Ctx) error {
	portfolioID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid portfolio ID")
	}

	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "portfolio")
	}

	// Public portfolios' comments are readable by anyone; otherwise owner or admin only
	if !portfolio.IsPublic {
		if err := h.authz.RequireOwnerOrAdmin(c, portfolio.UserID); err != nil {
			return err
		}
	}

	params := middleware.GetPagination(c)

	result, err := h.portfolioService.ListComments(c.Context(), uint(portfolioID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch comments")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	comments := make([]fiber.Map, len(result.Items))
	for i, cm := range result.Items {
		comments[i] = portfolioCommentToJSON(&cm)
	}

	return c.JSON(comments)
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) ListTemplates(c *fiber.Ctx) error {
	params := middleware.GetPagination(c)

	result, err := h.portfolioService.ListTemplates(c.Context(), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch templates")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	templates := make([]fiber.Map, len(result.Items))
	for i, t := range result.Items {
		templates[i] = portfolioTemplateToJSON(&t)
	}

	return c.JSON(templates)
}

func (h *PortfolioHandler) CreateFromTemplate(c *fiber.Ctx) error {
	templateID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid template ID")
	}

	var input struct {
		UserID uint `json:"user_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.UserID == 0 {
		return responses.BadRequest(c, "user_id is required")
	}

	portfolio, err := h.portfolioService.CreateFromTemplate(c.Context(), uint(templateID), callerAccountID(c), input.UserID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioToJSON(portfolio))
}
