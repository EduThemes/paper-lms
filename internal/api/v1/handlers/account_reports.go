package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// AccountReportHandler serves Canvas's Account Reports API (G138):
//
//	GET    /accounts/:account_id/reports
//	GET    /accounts/:account_id/reports/:report
//	POST   /accounts/:account_id/reports/:report
//	GET    /accounts/:account_id/reports/:report/:id
//	PUT    /accounts/:account_id/reports/:report/:id/abort
//	DELETE /accounts/:account_id/reports/:report/:id
//	GET    /accounts/:account_id/reports/:report/:id/file
//
// Every route is admin-only and tenant-scoped through RequireAdmin.
type AccountReportHandler struct {
	svc *service.AccountReportService
}

// NewAccountReportHandler wires the handler.
func NewAccountReportHandler(svc *service.AccountReportService) *AccountReportHandler {
	return &AccountReportHandler{svc: svc}
}

func (h *AccountReportHandler) reportJSON(c *fiber.Ctx, r *models.AccountReport) fiber.Map {
	var params map[string]any
	if err := json.Unmarshal([]byte(r.Parameters), &params); err != nil || params == nil {
		params = map[string]any{}
	}
	m := fiber.Map{
		"id":         r.ID,
		"report":     r.ReportType,
		"status":     r.WorkflowState,
		"progress":   r.Progress,
		"parameters": params,
		"created_at": r.CreatedAt,
		"started_at": r.StartedAt,
		"ended_at":   r.EndedAt,
		"message":    r.Message,
		"file_url":   nil,
		"attachment": nil,
	}
	if r.StartedAt != nil && r.EndedAt != nil {
		m["run_time"] = r.EndedAt.Sub(*r.StartedAt).Seconds()
	}
	if r.WorkflowState == models.AccountReportComplete && r.FileKey != nil {
		url := fmt.Sprintf("%s/api/v1/accounts/%d/reports/%s/%d/file", c.BaseURL(), r.AccountID, r.ReportType, r.ID)
		m["file_url"] = url
		name, ctype := "", ""
		if r.FileName != nil {
			name = *r.FileName
		}
		if r.ContentType != nil {
			ctype = *r.ContentType
		}
		m["attachment"] = fiber.Map{
			"display_name": name,
			"filename":     name,
			"content-type": ctype,
			"size":         r.FileSize,
			"url":          url,
		}
	}
	return m
}

// reportRunParams resolves (:report, :id) for the per-run routes.
// Returns wrote=true when a response was already written.
func reportRunParams(c *fiber.Ctx) (string, uint, uint, bool) {
	accountID, ok := urlAccountScope(c)
	if !ok {
		_ = responses.NotFound(c, "account")
		return "", 0, 0, true
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		_ = responses.NotFound(c, "report")
		return "", 0, 0, true
	}
	return c.Params("report"), uint(id), accountID, false
}

// ListAvailable returns the report catalog with each report's last run.
func (h *AccountReportHandler) ListAvailable(c *fiber.Ctx) error {
	accountID, ok := urlAccountScope(c)
	if !ok {
		return responses.NotFound(c, "account")
	}
	reports, err := h.svc.Available(c.Context(), accountID)
	if err != nil {
		return responses.InternalError(c, "Could not list reports")
	}
	out := make([]fiber.Map, len(reports))
	for i, r := range reports {
		row := fiber.Map{
			"report":      r.Report,
			"title":       r.Title,
			"description": r.Description,
			"parameters":  r.Parameters,
			"last_run":    nil,
		}
		if r.LastRun != nil {
			row["last_run"] = h.reportJSON(c, r.LastRun)
		}
		out[i] = row
	}
	return c.JSON(out)
}

// startReportParams reads Canvas's parameters[...] from a JSON body or a
// form body (parameters[users]=true).
func startReportParams(c *fiber.Ctx) (map[string]any, error) {
	params := map[string]any{}
	if strings.HasPrefix(string(c.Request().Header.ContentType()), fiber.MIMEApplicationJSON) {
		if len(c.Body()) == 0 {
			return params, nil
		}
		var body struct {
			Parameters map[string]any `json:"parameters"`
		}
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return nil, err
		}
		if body.Parameters != nil {
			params = body.Parameters
		}
		return params, nil
	}
	collect := func(key, value []byte) {
		k := string(key)
		if inner, ok := strings.CutPrefix(k, "parameters["); ok {
			if name, ok := strings.CutSuffix(inner, "]"); ok && name != "" {
				params[name] = string(value)
			}
		}
	}
	c.Request().PostArgs().VisitAll(collect)
	c.Context().QueryArgs().VisitAll(collect)
	return params, nil
}

// Start begins a run of :report.
func (h *AccountReportHandler) Start(c *fiber.Ctx) error {
	accountID, ok := urlAccountScope(c)
	if !ok {
		return responses.NotFound(c, "account")
	}
	userID, err := getUserID(c)
	if err != nil {
		return responses.Unauthorized(c)
	}
	params, err := startReportParams(c)
	if err != nil {
		return responses.BadRequest(c, "Invalid parameters")
	}
	rep, err := h.svc.Start(c.Context(), c.Params("report"), params, userID, accountID)
	switch {
	case errors.Is(err, service.ErrUnknownAccountReport):
		return responses.NotFound(c, "report")
	case errors.Is(err, service.ErrAccountReportParams):
		return responses.BadRequest(c, err.Error())
	case err != nil:
		return responses.InternalError(c, "Could not start the report")
	}
	return c.JSON(h.reportJSON(c, rep))
}

// ListRuns lists the runs of :report, newest first.
func (h *AccountReportHandler) ListRuns(c *fiber.Ctx) error {
	accountID, ok := urlAccountScope(c)
	if !ok {
		return responses.NotFound(c, "account")
	}
	params := middleware.GetPagination(c)
	page, err := h.svc.List(c.Context(), c.Params("report"), params, accountID)
	if errors.Is(err, service.ErrUnknownAccountReport) {
		return responses.NotFound(c, "report")
	}
	if err != nil {
		return responses.InternalError(c, "Could not list report runs")
	}
	responses.SetPaginationHeaders(c, page.TotalCount, page.Page, page.PerPage)
	out := make([]fiber.Map, len(page.Items))
	for i := range page.Items {
		out[i] = h.reportJSON(c, &page.Items[i])
	}
	return c.JSON(out)
}

// GetRun returns one run's status.
func (h *AccountReportHandler) GetRun(c *fiber.Ctx) error {
	report, id, accountID, wrote := reportRunParams(c)
	if wrote {
		return nil
	}
	rep, err := h.svc.Get(c.Context(), report, id, accountID)
	if err != nil {
		return responses.NotFound(c, "report")
	}
	return c.JSON(h.reportJSON(c, rep))
}

// AbortRun stops an unfinished run.
func (h *AccountReportHandler) AbortRun(c *fiber.Ctx) error {
	report, id, accountID, wrote := reportRunParams(c)
	if wrote {
		return nil
	}
	rep, err := h.svc.Abort(c.Context(), report, id, accountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountReportNotFound) {
			return responses.NotFound(c, "report")
		}
		return responses.InternalError(c, "Could not abort the report")
	}
	return c.JSON(h.reportJSON(c, rep))
}

// DeleteRun deletes a run and its file.
func (h *AccountReportHandler) DeleteRun(c *fiber.Ctx) error {
	report, id, accountID, wrote := reportRunParams(c)
	if wrote {
		return nil
	}
	rep, err := h.svc.Delete(c.Context(), report, id, accountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountReportNotFound) {
			return responses.NotFound(c, "report")
		}
		return responses.InternalError(c, "Could not delete the report")
	}
	return c.JSON(h.reportJSON(c, rep))
}

// DownloadRun streams a complete run's file as an attachment.
func (h *AccountReportHandler) DownloadRun(c *fiber.Ctx) error {
	report, id, accountID, wrote := reportRunParams(c)
	if wrote {
		return nil
	}
	rep, err := h.svc.Get(c.Context(), report, id, accountID)
	if err != nil {
		return responses.NotFound(c, "report")
	}
	rc, err := h.svc.OpenFile(c.Context(), rep)
	if err != nil {
		return responses.NotFound(c, "report file")
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return responses.InternalError(c, "Could not read the report file")
	}
	name := "report.csv"
	if rep.FileName != nil {
		name = *rep.FileName
	}
	ctype := "text/csv"
	if rep.ContentType != nil {
		ctype = *rep.ContentType
	}
	c.Set(fiber.HeaderContentType, ctype)
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", name))
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(data)
}
