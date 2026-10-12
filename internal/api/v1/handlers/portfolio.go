package handlers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/settingsctx"
	"github.com/EduThemes/paper-lms/internal/storage"
)

type PortfolioHandler struct {
	portfolioService *service.PortfolioService
	authz            *ResourceAuthorizer
	// fileService backs artifact file uploads/downloads. Optional: when nil
	// the upload endpoint reports the feature as unavailable.
	fileService *service.FileService
}

func NewPortfolioHandler(portfolioService *service.PortfolioService, authz *ResourceAuthorizer) *PortfolioHandler {
	return &PortfolioHandler{portfolioService: portfolioService, authz: authz}
}

// SetFileService wires file storage for portfolio artifact uploads.
func (h *PortfolioHandler) SetFileService(fs *service.FileService) {
	h.fileService = fs
}

// requireOwnerOrAdmin wraps ResourceAuthorizer.RequireOwnerOrAdmin and
// returns wrote=true whenever a response was emitted — including the case
// where the authorizer wrote a 403 but returned a nil error — so callers
// always stop (the (result, wrote, err) pattern).
func (h *PortfolioHandler) requireOwnerOrAdmin(c *fiber.Ctx, ownerID uint) (bool, error) {
	if err := h.authz.RequireOwnerOrAdmin(c, ownerID); err != nil {
		return true, err
	}
	if c.Response().StatusCode() == fiber.StatusForbidden {
		return true, nil
	}
	return false, nil
}

// loadOwnedPortfolio is the repeated load + owner-or-admin authz lift used by
// most write handlers. Returns (portfolio, wrote, err) per the Fiber-style
// (result, wrote, err) helper pattern: when `wrote=true` the helper has
// already emitted an HTTP response and the caller MUST return `err`
// immediately. Eliminates 14 copies of the same prologue.
func (h *PortfolioHandler) loadOwnedPortfolio(c *fiber.Ctx, paramName string) (*models.Portfolio, bool, error) {
	portfolioID, err := c.ParamsInt(paramName)
	if err != nil {
		return nil, true, responses.BadRequest(c, "Invalid portfolio ID")
	}
	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return nil, true, responses.NotFound(c, "portfolio")
	}
	if wrote, err := h.requireOwnerOrAdmin(c, portfolio.UserID); wrote {
		return nil, true, err
	}
	return portfolio, false, nil
}

// loadViewablePortfolio loads a portfolio the caller may view: the owner or
// an admin, or anyone in the tenant when it is published and public.
func (h *PortfolioHandler) loadViewablePortfolio(c *fiber.Ctx, paramName string) (*models.Portfolio, bool, error) {
	portfolioID, err := c.ParamsInt(paramName)
	if err != nil {
		return nil, true, responses.BadRequest(c, "Invalid portfolio ID")
	}
	portfolio, err := h.portfolioService.GetPortfolio(c.Context(), uint(portfolioID), callerAccountID(c))
	if err != nil {
		return nil, true, responses.NotFound(c, "portfolio")
	}
	if isPubliclyVisible(portfolio) {
		return portfolio, false, nil
	}
	if wrote, err := h.requireOwnerOrAdmin(c, portfolio.UserID); wrote {
		return nil, true, err
	}
	return portfolio, false, nil
}

func isPubliclyVisible(p *models.Portfolio) bool {
	return p.IsPublic && p.WorkflowState == models.PortfolioPublished
}

// writeChildNotFound maps service errors for section/artifact lookups.
func writeChildNotFound(c *fiber.Ctx, err error, what string) error {
	if errors.Is(err, service.ErrPortfolioChildNotFound) {
		return responses.NotFound(c, what)
	}
	return responses.BadRequest(c, err.Error())
}

// safePortfolioURL accepts http(s) URLs and same-origin absolute paths.
// Portfolio links are rendered as href/src on the public page, so
// javascript:/data: URLs would be stored XSS.
func safePortfolioURL(raw string) bool {
	raw = strings.TrimSpace(normalizePortfolioURL(raw))
	if raw == "" {
		return true
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return (scheme == "http" || scheme == "https") && u.Host != ""
}

// normalizePortfolioURL adds https:// to a bare host link such as
// "linkedin.com/in/jane" so it is stored as a working absolute URL.
func normalizePortfolioURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "/") || strings.Contains(raw, ":") {
		return raw
	}
	if strings.Contains(raw, ".") {
		return "https://" + raw
	}
	return raw
}

// normalizeURLPtr normalizes an optional URL field in place.
func normalizeURLPtr(p *string) {
	if p != nil {
		*p = normalizePortfolioURL(*p)
	}
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

// portfolioDetailJSON is the editor/read shape: the portfolio plus its
// sections and artifacts (B098 — the editor reads portfolio.sections and
// portfolio.artifacts on load and replaces its state with the PUT response).
// publicOnly drops hidden sections (and their artifacts) and rewrites
// uploaded-file links to the public, slug-scoped download route.
func (h *PortfolioHandler) portfolioDetailJSON(c *fiber.Ctx, p *models.Portfolio, publicOnly bool) fiber.Map {
	sections, artifacts, reflections := h.portfolioService.GetPortfolioDetail(c.Context(), p.ID)

	hidden := map[uint]bool{}
	sectionsJSON := make([]fiber.Map, 0, len(sections))
	for i := range sections {
		if publicOnly && !sections[i].IsVisible {
			hidden[sections[i].ID] = true
			continue
		}
		sectionsJSON = append(sectionsJSON, portfolioSectionToJSON(&sections[i]))
	}

	artifactsJSON := make([]fiber.Map, 0, len(artifacts))
	for i := range artifacts {
		a := &artifacts[i]
		if a.SectionID != nil && hidden[*a.SectionID] {
			continue
		}
		m := portfolioArtifactToJSON(a)
		if publicOnly && a.SourceType == "upload" && a.SourceID != nil {
			m["content_url"] = fmt.Sprintf("/api/v1/portfolios/public/%s/files/%d", url.PathEscape(p.Slug), *a.SourceID)
		}
		rs := reflections[a.ID]
		rsJSON := make([]fiber.Map, len(rs))
		for j := range rs {
			rsJSON[j] = portfolioReflectionToJSON(&rs[j])
		}
		m["reflections"] = rsJSON
		if len(rs) > 0 {
			m["reflection"] = rs[len(rs)-1].Content
		}
		artifactsJSON = append(artifactsJSON, m)
	}

	result := portfolioToJSON(p)
	result["sections"] = sectionsJSON
	result["artifacts"] = artifactsJSON
	if names := h.portfolioService.UserNames(c.Context(), []uint{p.UserID}); names[p.UserID] != "" {
		result["user_name"] = names[p.UserID]
	}
	return result
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
	for _, u := range []*string{&input.Portfolio.HeaderImageURL, &input.Portfolio.AvatarURL, &input.Portfolio.LinkedInURL, &input.Portfolio.WebsiteURL} {
		normalizeURLPtr(u)
		if !safePortfolioURL(*u) {
			return responses.BadRequest(c, "URLs must be http(s)")
		}
	}

	portfolio := &models.Portfolio{
		UserID:         uint(userID),
		Title:          input.Portfolio.Title,
		Description:    service.SanitizePlainText(input.Portfolio.Description),
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
	// Public portfolios are readable by anyone in the tenant; otherwise
	// owner or admin only.
	portfolio, wrote, err := h.loadViewablePortfolio(c, "id")
	if wrote {
		return err
	}
	// A non-owner reading a public portfolio gets the public view: hidden
	// sections (and their artifacts) stay private, exactly as on
	// /portfolios/public/:slug.
	return c.JSON(h.portfolioDetailJSON(c, portfolio, h.viewsAsPublic(c, portfolio)))
}

// viewsAsPublic reports whether the caller was let in only because the
// portfolio is published and public (they are neither its owner nor an
// admin of the owner's tenant), so they get the public view of it. Owners
// and admins always see everything — an admin sees a public portfolio the
// same way as a private one.
func (h *PortfolioHandler) viewsAsPublic(c *fiber.Ctx, p *models.Portfolio) bool {
	callerID, _ := c.Locals("user_id").(uint)
	if callerID == p.UserID || !isPubliclyVisible(p) {
		return false
	}
	if h.authz != nil && h.authz.RequireOwnerOrAdmin(c, p.UserID) == nil {
		return false
	}
	return true
}

func (h *PortfolioHandler) UpdatePortfolio(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
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
			WorkflowState  *string `json:"workflow_state"`
		} `json:"portfolio"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	for _, u := range []*string{input.Portfolio.HeaderImageURL, input.Portfolio.AvatarURL, input.Portfolio.LinkedInURL, input.Portfolio.WebsiteURL} {
		normalizeURLPtr(u)
		if u != nil && !safePortfolioURL(*u) {
			return responses.BadRequest(c, "URLs must be http(s)")
		}
	}

	if input.Portfolio.Title != nil {
		if strings.TrimSpace(*input.Portfolio.Title) == "" {
			return responses.BadRequest(c, "portfolio title is required")
		}
		portfolio.Title = *input.Portfolio.Title
	}
	if input.Portfolio.Description != nil {
		portfolio.Description = service.SanitizePlainText(*input.Portfolio.Description)
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
	// B197: "Archive" sends a workflow_state change, which used to be
	// silently ignored.
	if input.Portfolio.WorkflowState != nil {
		if err := h.portfolioService.SetWorkflowState(portfolio, *input.Portfolio.WorkflowState); err != nil {
			return responses.BadRequest(c, err.Error())
		}
	}

	if err := h.portfolioService.UpdatePortfolio(c.Context(), portfolio, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not update portfolio")
	}

	return c.JSON(h.portfolioDetailJSON(c, portfolio, false))
}

func (h *PortfolioHandler) DeletePortfolio(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	if err := h.portfolioService.DeletePortfolio(c.Context(), portfolio.ID, callerAccountID(c)); err != nil {
		return responses.InternalError(c, "Could not delete portfolio")
	}
	return c.JSON(fiber.Map{"delete": true})
}

func (h *PortfolioHandler) PublishPortfolio(c *fiber.Ctx) error {
	existing, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	portfolio, err := h.portfolioService.PublishPortfolio(c.Context(), existing.ID, callerAccountID(c))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(portfolioToJSON(portfolio))
}

// UnpublishPortfolio handles DELETE /portfolios/:id/publish (B197).
func (h *PortfolioHandler) UnpublishPortfolio(c *fiber.Ctx) error {
	existing, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	portfolio, err := h.portfolioService.UnpublishPortfolio(c.Context(), existing.ID, callerAccountID(c))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(portfolioToJSON(portfolio))
}

// DuplicatePortfolio handles POST /portfolios/:id/duplicate (B198: the UI's
// Duplicate used to call the import endpoint).
func (h *PortfolioHandler) DuplicatePortfolio(c *fiber.Ctx) error {
	existing, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	dup, err := h.portfolioService.DuplicatePortfolio(c.Context(), existing.ID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not duplicate portfolio")
	}
	return c.Status(fiber.StatusCreated).JSON(portfolioToJSON(dup))
}

// GetPublicPortfolio handles GET /portfolios/public/:slug (no auth).
//
// B094: the handler read c.ParamsInt("id") on a :slug route, so every
// visitor got 400. It now resolves the slug and only serves published,
// public portfolios, with hidden sections removed.
func (h *PortfolioHandler) GetPublicPortfolio(c *fiber.Ctx) error {
	portfolio, ok := h.loadPublicPortfolio(c)
	if !ok {
		return responses.NotFound(c, "portfolio")
	}

	// Record the view
	_ = h.portfolioService.RecordView(c.Context(), portfolio.ID)

	return c.JSON(h.portfolioDetailJSON(c, portfolio, true))
}

func (h *PortfolioHandler) loadPublicPortfolio(c *fiber.Ctx) (*models.Portfolio, bool) {
	slug := strings.TrimSpace(c.Params("slug"))
	if slug == "" {
		return nil, false
	}
	// Public endpoint: no auth, no tenant Locals. The slug lookup is
	// global; the published+public gate below is the visibility check.
	portfolio, err := h.portfolioService.GetBySlug(c.Context(), slug)
	if err != nil || !isPubliclyVisible(portfolio) {
		return nil, false
	}
	return portfolio, true
}

// ---------------------------------------------------------------------------
// Sections
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) AddSection(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
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
		PortfolioID: portfolio.ID,
		Title:       input.Section.Title,
		SectionType: input.Section.SectionType,
		// Typed in a textarea and rendered as text by the portfolio pages,
		// so it is plain text (B005): the HTML sanitizer entity-encoded it
		// and the public page showed "Don&#39;t". The plain-text sanitizer
		// leaves no markup at all, so the raw embed in the exported
		// site/PDF (generateStaticHTML) stays free of stored XSS.
		Content:   service.SanitizePlainText(input.Section.Content),
		Layout:    input.Section.Layout,
		IsVisible: isVisible,
	}
	if err := h.portfolioService.AddSection(c.Context(), section); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(portfolioSectionToJSON(section))
}

// UpdateSection handles PUT /portfolios/:id/sections/:section_id.
//
// B095: the old code looked the section up with ListSections(sectionID)
// (a section id passed as a portfolio id) and fell back to an empty
// &PortfolioSection{ID}, so db.Save zeroed every column the request did not
// send. B096: the section was never tied to the URL portfolio. The section
// is now loaded through the portfolio and only the sent fields change.
func (h *PortfolioHandler) UpdateSection(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
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

	section, err := h.portfolioService.GetSection(c.Context(), portfolio.ID, uint(sectionID), callerAccountID(c))
	if err != nil {
		return writeChildNotFound(c, err, "section")
	}

	if input.Section.Title != nil {
		section.Title = *input.Section.Title
	}
	if input.Section.SectionType != nil {
		section.SectionType = *input.Section.SectionType
	}
	if input.Section.Content != nil {
		// Sanitize on update too — see AddSection (plain text, no markup).
		section.Content = service.SanitizePlainText(*input.Section.Content)
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

	if err := h.portfolioService.UpdateSection(c.Context(), portfolio.ID, section, callerAccountID(c)); err != nil {
		return writeChildNotFound(c, err, "section")
	}

	return c.JSON(portfolioSectionToJSON(section))
}

func (h *PortfolioHandler) DeleteSection(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	sectionID, err := c.ParamsInt("section_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid section ID")
	}
	if err := h.portfolioService.RemoveSection(c.Context(), portfolio.ID, uint(sectionID), callerAccountID(c)); err != nil {
		if errors.Is(err, service.ErrPortfolioChildNotFound) {
			return responses.NotFound(c, "section")
		}
		return responses.InternalError(c, "Could not delete section")
	}
	return c.JSON(fiber.Map{"delete": true})
}

func (h *PortfolioHandler) ReorderSections(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
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
	if err := h.portfolioService.ReorderSections(c.Context(), portfolio.ID, input.Order, callerAccountID(c)); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"reorder": true})
}

// ---------------------------------------------------------------------------
// Artifacts
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) AddArtifact(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
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
	normalizeURLPtr(&input.Artifact.ContentURL)
	normalizeURLPtr(&input.Artifact.ThumbnailURL)
	if !safePortfolioURL(input.Artifact.ContentURL) || !safePortfolioURL(input.Artifact.ThumbnailURL) {
		return responses.BadRequest(c, "URLs must be http(s)")
	}
	// Uploaded-file artifacts are created only by the upload endpoint,
	// which ties the stored file to this portfolio.
	if input.Artifact.SourceType == "upload" {
		return responses.BadRequest(c, "use the upload endpoint for file artifacts")
	}

	artifact := &models.PortfolioArtifact{
		PortfolioID:   portfolio.ID,
		SectionID:     input.Artifact.SectionID,
		Title:         input.Artifact.Title,
		Description:   service.SanitizePlainText(input.Artifact.Description),
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

	if err := h.portfolioService.AddArtifact(c.Context(), artifact, callerAccountID(c)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioArtifactToJSON(artifact))
}

// UploadArtifactFile handles POST /portfolios/:id/artifacts/upload
// (multipart: file + title, description, tags, section_id, is_featured).
// The file is stored as the owner's personal attachment and the artifact
// links it through the portfolio-scoped download route.
func (h *PortfolioHandler) UploadArtifactFile(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	if h.fileService == nil {
		return responses.InternalError(c, "file uploads are not available")
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File is required")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer func() { _ = file.Close() }()

	var sectionID *uint
	if raw := strings.TrimSpace(c.FormValue("section_id")); raw != "" && raw != "null" {
		var v uint
		if _, err := fmt.Sscanf(raw, "%d", &v); err != nil || v == 0 {
			return responses.BadRequest(c, "Invalid section ID")
		}
		sectionID = &v
		if _, err := h.portfolioService.GetSection(c.Context(), portfolio.ID, v, callerAccountID(c)); err != nil {
			return responses.NotFound(c, "section")
		}
	}
	title := strings.TrimSpace(c.FormValue("title"))
	if title == "" {
		title = fileHeader.Filename
	}

	// Same content-type hardening as course uploads: don't trust a claimed
	// inline-safe image type whose bytes say otherwise.
	contentType := fileHeader.Header.Get("Content-Type")
	sniffBuf := make([]byte, 512)
	n, _ := io.ReadFull(file, sniffBuf)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return responses.InternalError(c, "Could not read uploaded file")
	}
	if n > 0 {
		sniffed := http.DetectContentType(sniffBuf[:n])
		if isInlineSafeMIME(contentType) && !isInlineSafeMIME(sniffed) &&
			!strings.HasPrefix(sniffed, "application/octet-stream") {
			contentType = sniffed
		}
	}

	attachment := &models.Attachment{
		ContextType: "User",
		ContextID:   portfolio.UserID,
		UserID:      portfolio.UserID,
		DisplayName: fileHeader.Filename,
		Filename:    fileHeader.Filename,
		ContentType: contentType,
		Size:        fileHeader.Size,
	}
	uploadCtx := settingsctx.WithAccountID(c.Context(), callerAccountID(c))
	if err := h.fileService.UploadFile(uploadCtx, attachment, file); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	artifactType := "document"
	switch {
	case strings.HasPrefix(contentType, "image/"):
		artifactType = "image"
	case strings.HasPrefix(contentType, "video/"):
		artifactType = "video"
	}
	attachmentID := attachment.ID
	artifact := &models.PortfolioArtifact{
		PortfolioID:   portfolio.ID,
		SectionID:     sectionID,
		Title:         title,
		Description:   service.SanitizePlainText(c.FormValue("description")),
		ArtifactType:  artifactType,
		ContentURL:    service.PortfolioUploadURL(portfolio.ID, attachment.ID),
		SourceType:    "upload",
		SourceID:      &attachmentID,
		FileType:      strings.TrimPrefix(strings.ToLower(filepath.Ext(fileHeader.Filename)), "."),
		FileSizeBytes: fileHeader.Size,
		Tags:          c.FormValue("tags"),
		IsFeatured:    c.FormValue("is_featured") == "true",
	}
	if err := h.portfolioService.AddArtifact(c.Context(), artifact, callerAccountID(c)); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(portfolioArtifactToJSON(artifact))
}

// DownloadArtifactFile handles GET /portfolios/:id/files/:file_id for the
// owner/admin, or anyone in the tenant once the portfolio is public.
func (h *PortfolioHandler) DownloadArtifactFile(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadViewablePortfolio(c, "id")
	if wrote {
		return err
	}
	return h.streamPortfolioFile(c, portfolio, callerAccountID(c), h.viewsAsPublic(c, portfolio))
}

// DownloadPublicArtifactFile handles GET /portfolios/public/:slug/files/:file_id
// (no auth) for published, public portfolios.
func (h *PortfolioHandler) DownloadPublicArtifactFile(c *fiber.Ctx) error {
	portfolio, ok := h.loadPublicPortfolio(c)
	if !ok {
		return responses.NotFound(c, "file")
	}
	// accountID=0: anonymous route with no tenant Locals. The attachment is
	// served only when it belongs to this published portfolio's owner and
	// backs one of its visible artifacts (checked in streamPortfolioFile).
	return h.streamPortfolioFile(c, portfolio, 0, true)
}

// streamPortfolioFile serves an attachment only when an artifact of this
// portfolio was created from it by the upload endpoint. visibleOnly (public
// viewers) also requires that artifact to be outside any hidden section.
func (h *PortfolioHandler) streamPortfolioFile(c *fiber.Ctx, portfolio *models.Portfolio, accountID uint, visibleOnly bool) error {
	if h.fileService == nil {
		return responses.NotFound(c, "file")
	}
	fileID, err := c.ParamsInt("file_id")
	if err != nil || fileID <= 0 {
		return responses.BadRequest(c, "Invalid file ID")
	}
	if !h.portfolioService.PortfolioHasUpload(c.Context(), portfolio.ID, uint(fileID), visibleOnly) {
		return responses.NotFound(c, "file")
	}
	attachment, err := h.fileService.GetAttachment(c.Context(), uint(fileID), accountID)
	if err != nil || attachment.ContextType != "User" || attachment.ContextID != portfolio.UserID || attachment.WorkflowState == "deleted" {
		return responses.NotFound(c, "file")
	}

	safeName := filepath.Base(attachment.Filename)
	safeName = strings.NewReplacer("\"", "", "\r", "", "\n", "").Replace(safeName)
	disposition := "attachment"
	if isInlineSafeMIME(attachment.ContentType) {
		disposition = "inline"
	}
	c.Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", disposition, safeName, url.PathEscape(safeName)))
	c.Set("X-Content-Type-Options", "nosniff")

	backend := h.fileService.StorageBackend()
	if _, isS3 := backend.(*storage.S3Backend); isS3 {
		downloadURL, err := h.fileService.GetFileURL(c.Context(), attachment.ID, accountID)
		if err != nil {
			return responses.InternalError(c, "Could not generate download URL")
		}
		return c.Redirect(downloadURL, fiber.StatusTemporaryRedirect)
	}
	reader, err := backend.Get(settingsctx.WithAccountID(c.Context(), accountID), attachment.StoragePath)
	if err != nil {
		return responses.InternalError(c, "Could not locate file")
	}
	// No defer Close: fasthttp streams the body after the handler returns
	// and closes the reader itself (io.Closer). Closing it here sent an
	// empty/failed download from local storage.
	c.Set("Content-Type", attachment.ContentType)
	return c.SendStream(reader)
}

// UpdateArtifact handles PUT /portfolios/:id/artifacts/:artifact_id.
// B095/B096: patch the stored artifact (loaded through the portfolio)
// instead of saving a zero-valued struct over it.
func (h *PortfolioHandler) UpdateArtifact(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
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
	normalizeURLPtr(input.Artifact.ContentURL)
	normalizeURLPtr(input.Artifact.ThumbnailURL)
	if (input.Artifact.ContentURL != nil && !safePortfolioURL(*input.Artifact.ContentURL)) ||
		(input.Artifact.ThumbnailURL != nil && !safePortfolioURL(*input.Artifact.ThumbnailURL)) {
		return responses.BadRequest(c, "URLs must be http(s)")
	}

	artifact, err := h.portfolioService.GetArtifact(c.Context(), portfolio.ID, uint(artifactID), callerAccountID(c))
	if err != nil {
		return writeChildNotFound(c, err, "artifact")
	}

	if input.Artifact.SectionID != nil {
		artifact.SectionID = input.Artifact.SectionID
		if *input.Artifact.SectionID == 0 {
			artifact.SectionID = nil
		}
	}
	if input.Artifact.Title != nil {
		artifact.Title = *input.Artifact.Title
	}
	if input.Artifact.Description != nil {
		artifact.Description = service.SanitizePlainText(*input.Artifact.Description)
	}
	if input.Artifact.ArtifactType != nil {
		artifact.ArtifactType = *input.Artifact.ArtifactType
	}
	if input.Artifact.ContentURL != nil && artifact.SourceType != "upload" {
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

	if err := h.portfolioService.UpdateArtifact(c.Context(), portfolio.ID, artifact, callerAccountID(c)); err != nil {
		return writeChildNotFound(c, err, "artifact")
	}

	return c.JSON(portfolioArtifactToJSON(artifact))
}

func (h *PortfolioHandler) DeleteArtifact(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	artifactID, err := c.ParamsInt("artifact_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid artifact ID")
	}
	if err := h.portfolioService.RemoveArtifact(c.Context(), portfolio.ID, uint(artifactID), callerAccountID(c)); err != nil {
		if errors.Is(err, service.ErrPortfolioChildNotFound) {
			return responses.NotFound(c, "artifact")
		}
		return responses.InternalError(c, "Could not delete artifact")
	}
	return c.JSON(fiber.Map{"delete": true})
}

// ---------------------------------------------------------------------------
// Reflections
// ---------------------------------------------------------------------------

func (h *PortfolioHandler) AddReflection(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	artifactID, err := c.ParamsInt("artifact_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid artifact ID")
	}
	callerID, ok := c.Locals("user_id").(uint)
	if !ok || callerID == 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		Reflection struct {
			PromptText string `json:"prompt_text"`
			Content    string `json:"content"`
		} `json:"reflection"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// B199: the author is the authenticated caller, never a body field.
	reflection := &models.PortfolioReflection{
		ArtifactID: uint(artifactID),
		UserID:     callerID,
		PromptText: input.Reflection.PromptText,
		Content:    input.Reflection.Content,
	}

	if err := h.portfolioService.AddReflection(c.Context(), portfolio.ID, reflection, callerAccountID(c)); err != nil {
		return writeChildNotFound(c, err, "artifact")
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioReflectionToJSON(reflection))
}

// ---------------------------------------------------------------------------
// Import from course
// ---------------------------------------------------------------------------

// ImportFromCourse handles POST /portfolios/:id/import.
//
// B198: the route has no :course_id, but the handler read
// c.ParamsInt("course_id") and required submission_ids the client never
// sends, so every import returned 400. The course now comes from the body
// and submission_ids is optional (default: all of the owner's submitted
// work in that course).
func (h *PortfolioHandler) ImportFromCourse(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}

	var input struct {
		CourseID      uint   `json:"course_id"`
		SubmissionIDs []uint `json:"submission_ids"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.CourseID == 0 && len(input.SubmissionIDs) == 0 {
		return responses.BadRequest(c, "course_id or submission_ids is required")
	}

	imported, err := h.portfolioService.ImportFromCourse(c.Context(), portfolio.ID, input.CourseID, input.SubmissionIDs, callerAccountID(c))
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
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	zipData, err := h.portfolioService.ExportAsStaticSite(c.Context(), portfolio.ID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not export portfolio")
	}
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", "attachment; filename=portfolio-export.zip")
	return c.Send(zipData)
}

// ExportAsPDF returns a real PDF document (B200: it used to return HTML that
// the client saved under a .pdf name).
func (h *PortfolioHandler) ExportAsPDF(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadOwnedPortfolio(c, "id")
	if wrote {
		return err
	}
	pdfData, err := h.portfolioService.ExportAsPDF(c.Context(), portfolio.ID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not export portfolio")
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", "attachment; filename=portfolio.pdf")
	return c.Send(pdfData)
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

// AddComment handles POST /portfolios/:id/comments.
//
// B199: comments are feedback from viewers, but the handler required
// owner-or-admin (so teachers and peers could never comment) and took the
// author's user_id from the body (so the owner could post as anyone). Any
// caller who can view the portfolio may now comment, as themselves.
func (h *PortfolioHandler) AddComment(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadViewablePortfolio(c, "id")
	if wrote {
		return err
	}
	callerID, ok := c.Locals("user_id").(uint)
	if !ok || callerID == 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		Comment struct {
			SectionID  *uint  `json:"section_id"`
			ArtifactID *uint  `json:"artifact_id"`
			Content    string `json:"content"`
			ParentID   *uint  `json:"parent_id"`
		} `json:"comment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Plain-text feedback: rendered as text, never as HTML, so it is not
	// passed through SanitizeHTML (identifier/plain-text rule).
	comment := &models.PortfolioComment{
		PortfolioID: portfolio.ID,
		SectionID:   input.Comment.SectionID,
		ArtifactID:  input.Comment.ArtifactID,
		UserID:      callerID,
		Content:     input.Comment.Content,
		ParentID:    input.Comment.ParentID,
	}

	if err := h.portfolioService.AddComment(c.Context(), comment, callerAccountID(c)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	out := portfolioCommentToJSON(comment)
	if names := h.portfolioService.UserNames(c.Context(), []uint{callerID}); names[callerID] != "" {
		out["user_name"] = names[callerID]
	}
	return c.Status(fiber.StatusCreated).JSON(out)
}

func (h *PortfolioHandler) ListComments(c *fiber.Ctx) error {
	portfolio, wrote, err := h.loadViewablePortfolio(c, "id")
	if wrote {
		return err
	}

	params := middleware.GetPagination(c)

	result, err := h.portfolioService.ListComments(c.Context(), portfolio.ID, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch comments")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	ids := make([]uint, 0, len(result.Items))
	for _, cm := range result.Items {
		ids = append(ids, cm.UserID)
	}
	names := h.portfolioService.UserNames(c.Context(), ids)

	comments := make([]fiber.Map, len(result.Items))
	for i, cm := range result.Items {
		comments[i] = portfolioCommentToJSON(&cm)
		if n := names[cm.UserID]; n != "" {
			comments[i]["user_name"] = n
		}
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

// CreateFromTemplate handles POST /portfolio_templates/:template_id/create.
// The route parameter is :template_id (the handler used to read :id and
// always 400), and the new portfolio belongs to the caller — never to a
// user_id from the body.
func (h *PortfolioHandler) CreateFromTemplate(c *fiber.Ctx) error {
	templateID, err := c.ParamsInt("template_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid template ID")
	}
	callerID, ok := c.Locals("user_id").(uint)
	if !ok || callerID == 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	portfolio, err := h.portfolioService.CreateFromTemplate(c.Context(), uint(templateID), callerAccountID(c), callerID)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(portfolioToJSON(portfolio))
}
