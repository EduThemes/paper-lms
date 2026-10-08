package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Deep linking target kinds.
const (
	DeepLinkTargetModule     = "module"
	DeepLinkTargetAssignment = "assignment"
	DeepLinkTargetEditor     = "editor"
	DeepLinkTargetLink       = "link"
	// DeepLinkTargetSubmission is homework_submission: a student submits
	// content chosen in the tool to an assignment.
	DeepLinkTargetSubmission = "submission"
)

// ValidDeepLinkTarget reports whether kind is a supported target.
func ValidDeepLinkTarget(kind string) bool {
	switch kind {
	case DeepLinkTargetModule, DeepLinkTargetAssignment, DeepLinkTargetEditor, DeepLinkTargetLink, DeepLinkTargetSubmission:
		return true
	}
	return false
}

const ltiDeepLinkDataTTL = time.Hour

// deepLinkingSettings builds the deep_linking_settings claim of an
// LtiDeepLinkingRequest (LTI DL 2.0 §4.4.1). The opaque data value is a
// platform-signed JWT naming the user, course, tool and target, so the
// return endpoint (which the tool's browser form posts to without a
// Paper session) can trust where the content goes.
func (s *LTIService) deepLinkingSettings(h ltiLaunchHint, devKeyID uint) (map[string]interface{}, error) {
	target := h.DeepLink
	if target == nil || !ValidDeepLinkTarget(target.Kind) {
		return nil, ltiInvalid("invalid deep linking target")
	}
	acceptTypes := []string{"ltiResourceLink", "link"}
	targets := []string{"iframe", "window"}
	multiple := true
	autoCreate := false
	switch target.Kind {
	case DeepLinkTargetEditor:
		acceptTypes = []string{"ltiResourceLink", "link", "html", "image", "file"}
		targets = []string{"embed", "iframe", "window"}
	case DeepLinkTargetAssignment:
		acceptTypes = []string{"ltiResourceLink"}
		multiple = false
	case DeepLinkTargetSubmission:
		acceptTypes = []string{"link", "file"}
		targets = []string{"window"}
		multiple = false
	case DeepLinkTargetModule:
		autoCreate = true
	}
	now := time.Now()
	data, err := s.signPlatformJWT(jwt.MapClaims{
		"iss": s.PlatformIssuer, "aud": ltiPurposeDeepLinkData,
		"iat": now.Unix(), "exp": now.Add(ltiDeepLinkDataTTL).Unix(), "jti": randomHex(8),
		"uid": h.UserID, "acct": h.AccountID, "cid": h.CourseID, "tool": h.ToolID, "dk": devKeyID,
		"pl": h.Placement, "kind": target.Kind, "mid": target.ModuleID, "aid": target.AssignmentID,
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"deep_link_return_url":                 s.DeepLinkReturnURL(),
		"accept_types":                         acceptTypes,
		"accept_presentation_document_targets": targets,
		"accept_multiple":                      multiple,
		"auto_create":                          autoCreate,
		"data":                                 data,
	}, nil
}

// DeepLinkLineItem is a content item's lineItem (AGS-linked gradebook column).
type DeepLinkLineItem struct {
	Label        string  `json:"label"`
	ScoreMaximum float64 `json:"scoreMaximum"`
	ResourceID   string  `json:"resourceId"`
	Tag          string  `json:"tag"`
}

// DeepLinkContentItem is one content item of an LtiDeepLinkingResponse.
type DeepLinkContentItem struct {
	Type   string            `json:"type"`
	Title  string            `json:"title"`
	Text   string            `json:"text"`
	URL    string            `json:"url"`
	HTML   string            `json:"html"`
	Custom map[string]string `json:"-"`
	// RawCustom keeps custom values of any JSON type before stringifying.
	RawCustom map[string]interface{} `json:"custom"`
	LineItem  *DeepLinkLineItem      `json:"lineItem"`
	Iframe    *struct {
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Src    string `json:"src"`
	} `json:"iframe"`
	Window *struct {
		TargetName string `json:"targetName"`
	} `json:"window"`
}

// DeepLinkReturnedItem is what the browser receives for a content item.
type DeepLinkReturnedItem struct {
	Type             string `json:"type"`
	Title            string `json:"title,omitempty"`
	Text             string `json:"text,omitempty"`
	URL              string `json:"url,omitempty"`
	HTML             string `json:"html,omitempty"`
	ResourceLinkUUID string `json:"resource_link_id,omitempty"`
	ToolID           uint   `json:"tool_id,omitempty"`
	ModuleItemID     uint   `json:"module_item_id,omitempty"`
	AssignmentID     uint   `json:"assignment_id,omitempty"`
	Width            int    `json:"width,omitempty"`
	Height           int    `json:"height,omitempty"`
	NewWindow        bool   `json:"new_window,omitempty"`
}

// DeepLinkResult summarizes a processed deep linking response.
type DeepLinkResult struct {
	Kind         string                 `json:"kind"`
	CourseID     uint                   `json:"course_id"`
	ModuleID     uint                   `json:"module_id,omitempty"`
	AssignmentID uint                   `json:"assignment_id,omitempty"`
	Items        []DeepLinkReturnedItem `json:"items"`
	Msg          string                 `json:"msg,omitempty"`
	ErrorMsg     string                 `json:"errormsg,omitempty"`
	Ignored      int                    `json:"ignored,omitempty"`
}

// ErrDeepLinkInvalid wraps every rejection of a deep linking response.
var ErrDeepLinkInvalid = errors.New("invalid deep linking response")

func dlInvalid(msg string) error { return fmt.Errorf("%w: %s", ErrDeepLinkInvalid, msg) }

const (
	claimMessageType  = "https://purl.imsglobal.org/spec/lti/claim/message_type"
	claimVersion      = "https://purl.imsglobal.org/spec/lti/claim/version"
	claimDeploymentID = "https://purl.imsglobal.org/spec/lti/claim/deployment_id"
	claimDLData       = "https://purl.imsglobal.org/spec/lti-dl/claim/data"
	claimDLItems      = "https://purl.imsglobal.org/spec/lti-dl/claim/content_items"
	claimDLMsg        = "https://purl.imsglobal.org/spec/lti-dl/claim/msg"
	claimDLErrorMsg   = "https://purl.imsglobal.org/spec/lti-dl/claim/errormsg"
)

// HandleDeepLinkingResponse verifies an LtiDeepLinkingResponse JWT (G130)
// and applies its content items to the target the request named:
//
//   - module: each ltiResourceLink becomes a ContextExternalTool module
//     item (with its resource link; a lineItem makes it a graded
//     external_tool assignment instead), each link an ExternalUrl item.
//   - assignment: the selected ltiResourceLink becomes the assignment's
//     external tool (and its lineItem sets the points).
//   - editor / link: links are stored and returned for the browser to
//     insert; html/image/file items are returned as-is (the HTML is
//     sanitized when the edited content is saved).
//
// The tool's signature, aud, nonce (single use), deployment and the
// platform-signed data claim are all verified; the user must still be
// course staff.
func (s *LTIService) HandleDeepLinkingResponse(ctx context.Context, rawJWT string) (*DeepLinkResult, error) {
	if err := s.requireAdvantage(); err != nil {
		return nil, err
	}
	s.RefreshPlatformKeys(ctx)
	claims, devKey, _, err := s.toolSignedClaims(ctx, rawJWT, "")
	if err != nil {
		return nil, dlInvalid(err.Error())
	}
	if iss, _ := claims["iss"].(string); iss != devKey.ClientID {
		return nil, dlInvalid("iss must be the tool's client_id")
	}
	if !audContains(claims, s.PlatformIssuer) {
		return nil, dlInvalid("aud must be the platform issuer")
	}
	if mt, _ := claims[claimMessageType].(string); mt != "LtiDeepLinkingResponse" {
		return nil, dlInvalid("message_type must be LtiDeepLinkingResponse")
	}
	if v, _ := claims[claimVersion].(string); v != "1.3.0" {
		return nil, dlInvalid("version must be 1.3.0")
	}
	nonce, _ := claims["nonce"].(string)
	if err := s.consumeJTI(ctx, "lti-dl", devKey.ClientID, nonce, 2*time.Hour); err != nil {
		return nil, dlInvalid("nonce is missing or already used")
	}
	dataStr, _ := claims[claimDLData].(string)
	data, err := s.verifyPlatformJWT(dataStr, ltiPurposeDeepLinkData)
	if err != nil {
		return nil, dlInvalid("data claim is missing or invalid")
	}
	if claimUint(data, "dk") != devKey.ID {
		return nil, dlInvalid("response does not come from the requested tool")
	}
	toolID := claimUint(data, "tool")
	if dep, _ := claims[claimDeploymentID].(string); dep != strconv.FormatUint(uint64(toolID), 10) {
		return nil, dlInvalid("deployment_id does not match the request")
	}

	userID, accountID, courseID := claimUint(data, "uid"), claimUint(data, "acct"), claimUint(data, "cid")
	kind := claimString(data, "kind")
	if kind == DeepLinkTargetSubmission {
		if !s.userIsActiveStudent(ctx, userID, courseID, accountID) {
			return nil, ErrLTILaunchNotFound
		}
	} else if !s.userIsCourseStaff(ctx, userID, courseID, accountID) {
		return nil, ErrLTILaunchNotFound
	}
	tool, err := s.adv.Store.FindActiveToolForCourse(ctx, toolID, courseID, accountID)
	if err != nil || tool == nil || tool.DeveloperKeyID != devKey.ID {
		return nil, ErrLTILaunchNotFound
	}

	res := &DeepLinkResult{Kind: kind, CourseID: courseID, Items: []DeepLinkReturnedItem{}}
	res.Msg, _ = claims[claimDLMsg].(string)
	res.ErrorMsg, _ = claims[claimDLErrorMsg].(string)

	items, err := decodeContentItems(claims[claimDLItems])
	if err != nil {
		return nil, dlInvalid(err.Error())
	}

	switch kind {
	case DeepLinkTargetModule:
		res.ModuleID = claimUint(data, "mid")
		err = s.applyDeepLinkToModule(ctx, res, items, tool, devKey, accountID, claimString(data, "pl"))
	case DeepLinkTargetAssignment:
		res.AssignmentID = claimUint(data, "aid")
		err = s.applyDeepLinkToAssignment(ctx, res, items, tool, accountID)
	case DeepLinkTargetSubmission:
		res.AssignmentID = claimUint(data, "aid")
		err = s.applyDeepLinkToSubmission(ctx, res, items, userID, accountID)
	default:
		err = s.applyDeepLinkToEditor(ctx, res, items, tool, claimString(data, "pl"))
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func decodeContentItems(raw interface{}) ([]DeepLinkContentItem, error) {
	if raw == nil {
		return nil, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, errors.New("content_items is not valid")
	}
	var items []DeepLinkContentItem
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, errors.New("content_items must be an array of objects")
	}
	if len(items) > 50 {
		return nil, errors.New("too many content items")
	}
	for i := range items {
		// An ltiResourceLink's url overrides the tool's launch URL; one
		// that is not an https URL is dropped (the tool default is used).
		if items[i].Type == "ltiResourceLink" && items[i].URL != "" && !validToolURL(items[i].URL) {
			items[i].URL = ""
		}
		items[i].Custom = map[string]string{}
		for k, v := range items[i].RawCustom {
			switch tv := v.(type) {
			case string:
				items[i].Custom[k] = tv
			default:
				vb, _ := json.Marshal(tv)
				items[i].Custom[k] = string(vb)
			}
		}
	}
	return items, nil
}

// userIsCourseStaff reports whether the user teaches/assists/designs the
// course or is an admin of its tenant.
func (s *LTIService) userIsCourseStaff(ctx context.Context, userID, courseID, accountID uint) bool {
	if userID == 0 || courseID == 0 || accountID == 0 {
		return false
	}
	if _, err := s.courseRepo.FindByID(ctx, courseID, accountID); err != nil {
		return false
	}
	if s.adv.UserRepo != nil {
		if u, err := s.adv.UserRepo.FindByID(ctx, userID, accountID); err == nil && u != nil &&
			(u.Role == models.RoleAdmin || u.Role == models.RoleSuperUser) {
			return true
		}
	}
	e, err := s.enrollRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	if err != nil || e == nil || e.WorkflowState != "active" {
		return false
	}
	switch e.Type {
	case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment":
		return true
	}
	return false
}

func customJSON(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func itemTitle(it DeepLinkContentItem, fallback string) string {
	t := strings.TrimSpace(it.Title)
	if t == "" {
		t = strings.TrimSpace(it.Text)
	}
	if t == "" {
		t = fallback
	}
	if len(t) > 255 {
		t = t[:255]
	}
	return SanitizePlainText(t)
}

func (s *LTIService) applyDeepLinkToModule(ctx context.Context, res *DeepLinkResult, items []DeepLinkContentItem, tool *models.ContextExternalTool, devKey *models.DeveloperKey, accountID uint, placement string) error {
	if s.adv.ModuleRepo == nil || s.adv.ModuleItemRepo == nil {
		return errors.New("modules are not configured")
	}
	module, err := s.adv.ModuleRepo.FindByID(ctx, res.ModuleID, accountID)
	if err != nil || module == nil || module.CourseID != res.CourseID {
		return ErrLTILaunchNotFound
	}
	for _, it := range items {
		pos, _ := s.adv.Store.NextModuleItemPosition(ctx, module.ID)
		switch it.Type {
		case "ltiResourceLink":
			title := itemTitle(it, tool.Name)
			if it.LineItem != nil && it.LineItem.ScoreMaximum > 0 && s.adv.AssignmentRepo != nil {
				a, _, aErr := s.createToolAssignment(ctx, res.CourseID, accountID, tool, devKey, title, it.URL, it.Custom, it.LineItem)
				if aErr != nil {
					return aErr
				}
				aid := a.ID
				tag := &models.ContentTag{
					ContextModuleID: module.ID, Title: title, ContentType: "Assignment", ContentID: &aid,
					Position: pos, WorkflowState: models.ContentTagUnpublished,
				}
				if err := s.adv.ModuleItemRepo.Create(ctx, tag); err != nil {
					return err
				}
				res.Items = append(res.Items, DeepLinkReturnedItem{Type: it.Type, Title: title, ModuleItemID: tag.ID, AssignmentID: a.ID, ToolID: tool.ID})
				continue
			}
			toolID := tool.ID
			tag := &models.ContentTag{
				ContextModuleID: module.ID, Title: title, ContentType: ContentTypeExternalTool, ContentID: &toolID,
				URL: it.URL, Position: pos, NewTab: it.Window != nil, WorkflowState: models.ContentTagUnpublished,
			}
			if err := s.adv.ModuleItemRepo.Create(ctx, tag); err != nil {
				return err
			}
			tagID := tag.ID
			link := &models.LTIResourceLink{
				ContextExternalToolID: tool.ID, ContextType: "Course", ContextID: res.CourseID,
				ResourceLinkID: uuid.NewString(), LookupUUID: uuid.NewString(), Title: title, URL: it.URL,
				CustomParameters: customJSON(it.Custom), ContentTagID: &tagID, Placement: firstNonEmpty(placement, LTIPlacementLinkSelection),
			}
			if err := s.adv.Store.CreateResourceLink(ctx, link); err != nil {
				return err
			}
			res.Items = append(res.Items, DeepLinkReturnedItem{Type: it.Type, Title: title, ModuleItemID: tag.ID, ResourceLinkUUID: link.ResourceLinkID, ToolID: tool.ID})
		case "link":
			if !validToolURL(it.URL) {
				res.Ignored++
				continue
			}
			title := itemTitle(it, it.URL)
			tag := &models.ContentTag{
				ContextModuleID: module.ID, Title: title, ContentType: "ExternalUrl", URL: it.URL,
				Position: pos, NewTab: true, WorkflowState: models.ContentTagUnpublished,
			}
			if err := s.adv.ModuleItemRepo.Create(ctx, tag); err != nil {
				return err
			}
			res.Items = append(res.Items, DeepLinkReturnedItem{Type: it.Type, Title: title, URL: it.URL, ModuleItemID: tag.ID})
		default:
			res.Ignored++
		}
	}
	return nil
}

func (s *LTIService) applyDeepLinkToAssignment(ctx context.Context, res *DeepLinkResult, items []DeepLinkContentItem, tool *models.ContextExternalTool, accountID uint) error {
	for _, it := range items {
		if it.Type != "ltiResourceLink" {
			res.Ignored++
			continue
		}
		link, err := s.AttachToolToAssignment(ctx, AssignmentToolInput{
			AccountID: accountID, CourseID: res.CourseID, AssignmentID: res.AssignmentID, ToolID: tool.ID,
			URL: it.URL, Title: itemTitle(it, tool.Name), Custom: it.Custom, LineItem: it.LineItem,
		})
		if err != nil {
			return err
		}
		res.Items = append(res.Items, DeepLinkReturnedItem{
			Type: it.Type, Title: link.Title, URL: it.URL, ResourceLinkUUID: link.ResourceLinkID,
			ToolID: tool.ID, AssignmentID: res.AssignmentID,
		})
		break // accept_multiple is false for assignments
	}
	return nil
}

// userIsActiveStudent reports whether the user is an active student of
// the course (homework_submission is a student placement).
func (s *LTIService) userIsActiveStudent(ctx context.Context, userID, courseID, accountID uint) bool {
	if userID == 0 || courseID == 0 || accountID == 0 {
		return false
	}
	e, err := s.enrollRepo.FindByUserAndCourse(ctx, userID, courseID, accountID)
	return err == nil && e != nil && e.WorkflowState == "active" && e.Type == "StudentEnrollment"
}

// applyDeepLinkToSubmission submits the first link/file item as the
// student's online_url submission (G131 homework_submission). The normal
// submit path enforces publication, the accepted types and lock dates.
func (s *LTIService) applyDeepLinkToSubmission(ctx context.Context, res *DeepLinkResult, items []DeepLinkContentItem, userID, accountID uint) error {
	if s.adv.Submitter == nil {
		return errors.New("submissions are not configured")
	}
	for _, it := range items {
		if (it.Type != "link" && it.Type != "file") || !validToolURL(it.URL) {
			res.Ignored++
			continue
		}
		subType := "online_url"
		u := it.URL
		sub := &models.Submission{AssignmentID: res.AssignmentID, UserID: userID, SubmissionType: &subType, URL: &u}
		if err := s.adv.Submitter.Submit(ctx, sub, res.CourseID, accountID); err != nil {
			return dlInvalid(err.Error())
		}
		res.Items = append(res.Items, DeepLinkReturnedItem{Type: it.Type, Title: itemTitle(it, it.URL), URL: it.URL, AssignmentID: res.AssignmentID})
		return nil
	}
	return dlInvalid("the tool returned nothing to submit")
}

func (s *LTIService) applyDeepLinkToEditor(ctx context.Context, res *DeepLinkResult, items []DeepLinkContentItem, tool *models.ContextExternalTool, placement string) error {
	for _, it := range items {
		switch it.Type {
		case "ltiResourceLink":
			title := itemTitle(it, tool.Name)
			link := &models.LTIResourceLink{
				ContextExternalToolID: tool.ID, ContextType: "Course", ContextID: res.CourseID,
				ResourceLinkID: uuid.NewString(), LookupUUID: uuid.NewString(), Title: title, URL: it.URL,
				CustomParameters: customJSON(it.Custom), Placement: firstNonEmpty(placement, LTIPlacementEditorButton),
			}
			if err := s.adv.Store.CreateResourceLink(ctx, link); err != nil {
				return err
			}
			out := DeepLinkReturnedItem{
				Type: it.Type, Title: title, ResourceLinkUUID: link.ResourceLinkID, ToolID: tool.ID,
				URL: fmt.Sprintf("/courses/%d/external_tools/%d?resource_link_id=%s", res.CourseID, tool.ID, link.ResourceLinkID),
			}
			if it.Iframe != nil {
				out.Width, out.Height = it.Iframe.Width, it.Iframe.Height
			}
			out.NewWindow = it.Window != nil
			res.Items = append(res.Items, out)
		case "link", "image", "file":
			if !validToolURL(it.URL) {
				res.Ignored++
				continue
			}
			res.Items = append(res.Items, DeepLinkReturnedItem{Type: it.Type, Title: itemTitle(it, it.URL), Text: it.Text, URL: it.URL})
		case "html":
			if res.Kind != DeepLinkTargetEditor || strings.TrimSpace(it.HTML) == "" {
				res.Ignored++
				continue
			}
			res.Items = append(res.Items, DeepLinkReturnedItem{Type: it.Type, HTML: SanitizeHTML(it.HTML)})
		default:
			res.Ignored++
		}
	}
	return nil
}

// --- assignment external tools (G131 assignment_selection, G128) -------------

// AssignmentToolInput configures an assignment's external tool.
type AssignmentToolInput struct {
	AccountID    uint
	CourseID     uint
	AssignmentID uint
	ToolID       uint
	URL          string
	Title        string
	Custom       map[string]string
	LineItem     *DeepLinkLineItem
}

// AttachToolToAssignment makes an assignment an external_tool assignment
// launching the given tool: it stores (or updates) the assignment's
// resource link, sets submission_types to external_tool and makes sure
// the assignment has its default AGS line item, owned by the tool, so
// the tool can post scores to the gradebook (G128).
func (s *LTIService) AttachToolToAssignment(ctx context.Context, in AssignmentToolInput) (*models.LTIResourceLink, error) {
	if err := s.requireAdvantage(); err != nil {
		return nil, err
	}
	if s.adv.AssignmentRepo == nil {
		return nil, errors.New("assignments are not configured")
	}
	a, err := s.adv.AssignmentRepo.FindByID(ctx, in.AssignmentID, in.AccountID)
	if err != nil || a == nil || a.CourseID != in.CourseID || a.WorkflowState == models.AssignmentDeleted {
		return nil, ErrLTILaunchNotFound
	}
	tool, err := s.adv.Store.FindActiveToolForCourse(ctx, in.ToolID, in.CourseID, in.AccountID)
	if err != nil || tool == nil {
		return nil, ErrLTILaunchNotFound
	}
	if in.URL != "" && !validToolURL(in.URL) {
		return nil, ltiInvalid("launch URL must be an https URL")
	}

	link, findErr := s.adv.Store.FindResourceLinkByAssignment(ctx, a.ID, in.AccountID)
	if findErr == nil && link != nil && link.ContextExternalToolID != tool.ID {
		// Switching to another tool: the previous tool loses the
		// assignment's line items (unless it is the same tool, installed
		// in another context).
		prev, pErr := s.adv.Store.FindActiveToolForCourse(ctx, link.ContextExternalToolID, in.CourseID, in.AccountID)
		if pErr != nil || prev == nil || prev.DeveloperKeyID != tool.DeveloperKeyID || tool.DeveloperKeyID == 0 {
			if err := s.adv.Store.ReleaseAssignmentLineItems(ctx, a.ID); err != nil {
				return nil, err
			}
		}
	}
	if findErr != nil || link == nil {
		aid := a.ID
		link = &models.LTIResourceLink{
			ContextType: "Course", ContextID: in.CourseID, ResourceLinkID: uuid.NewString(),
			LookupUUID: uuid.NewString(), AssignmentID: &aid, Placement: LTIPlacementAssignmentSelection,
		}
	}
	link.ContextExternalToolID = tool.ID
	link.URL = in.URL
	link.Title = firstNonEmpty(in.Title, a.Name)
	link.CustomParameters = customJSON(in.Custom)
	if link.ID == 0 {
		err = s.adv.Store.CreateResourceLink(ctx, link)
	} else {
		err = s.adv.Store.UpdateResourceLink(ctx, link)
	}
	if err != nil {
		return nil, err
	}

	changed := false
	if a.SubmissionTypes != "external_tool" {
		a.SubmissionTypes = "external_tool"
		changed = true
	}
	if in.LineItem != nil && in.LineItem.ScoreMaximum > 0 {
		pts := in.LineItem.ScoreMaximum
		a.PointsPossible = &pts
		changed = true
	}
	if changed {
		if err := s.adv.AssignmentRepo.Update(ctx, a); err != nil {
			return nil, err
		}
	}
	if err := s.ensureAssignmentLineItem(ctx, a, link, tool, in.LineItem); err != nil {
		return nil, err
	}
	return link, nil
}

// ensureAssignmentLineItem creates or re-points the default line item of
// an external_tool assignment.
func (s *LTIService) ensureAssignmentLineItem(ctx context.Context, a *models.Assignment, link *models.LTIResourceLink, tool *models.ContextExternalTool, li *DeepLinkLineItem) error {
	if s.adv.LineItemRepo == nil {
		return nil
	}
	maxScore := 100.0
	if a.PointsPossible != nil && *a.PointsPossible > 0 {
		maxScore = *a.PointsPossible
	}
	var owner *uint
	if tool.DeveloperKeyID != 0 {
		dk := tool.DeveloperKeyID
		owner = &dk
	}
	item, err := s.adv.LineItemRepo.FindByAssignmentID(ctx, a.ID)
	if err != nil || item == nil {
		aid := a.ID
		item = &models.LTILineItem{CourseID: a.CourseID, AssignmentID: &aid, Label: a.Name, ScoreMaximum: maxScore}
		if li != nil {
			item.ResourceID, item.Tag = li.ResourceID, li.Tag
			if li.Label != "" {
				item.Label = li.Label
			}
		}
		item.ResourceLinkIDStr = link.ResourceLinkID
		item.DeveloperKeyID = owner
		return s.adv.LineItemRepo.Create(ctx, item)
	}
	item.ResourceLinkIDStr = link.ResourceLinkID
	item.DeveloperKeyID = owner
	item.ScoreMaximum = maxScore
	if li != nil {
		if li.ResourceID != "" {
			item.ResourceID = li.ResourceID
		}
		if li.Tag != "" {
			item.Tag = li.Tag
		}
	}
	return s.adv.LineItemRepo.Update(ctx, item)
}

// DetachToolFromAssignment removes an assignment's external tool link.
func (s *LTIService) DetachToolFromAssignment(ctx context.Context, accountID, courseID, assignmentID uint) error {
	if err := s.requireAdvantage(); err != nil {
		return err
	}
	a, err := s.adv.AssignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil || a == nil || a.CourseID != courseID {
		return ErrLTILaunchNotFound
	}
	if err := s.adv.Store.DeleteResourceLinksByAssignment(ctx, a.ID, accountID); err != nil {
		return err
	}
	// The detached tool loses its line items: without this it could keep
	// listing them and posting scores to the assignment over AGS.
	if err := s.adv.Store.ReleaseAssignmentLineItems(ctx, a.ID); err != nil {
		return err
	}
	if a.SubmissionTypes == "external_tool" {
		a.SubmissionTypes = "none"
		return s.adv.AssignmentRepo.Update(ctx, a)
	}
	return nil
}

// AssignmentToolView describes an assignment's external tool.
type AssignmentToolView struct {
	ToolID           uint   `json:"tool_id"`
	ToolName         string `json:"tool_name"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	ResourceLinkUUID string `json:"resource_link_id"`
	LineItemID       uint   `json:"line_item_id,omitempty"`
	Published        bool   `json:"published"`
}

// GetAssignmentTool returns the assignment's external tool, or
// ErrLTILaunchNotFound when it has none.
func (s *LTIService) GetAssignmentTool(ctx context.Context, accountID, courseID, assignmentID uint) (*AssignmentToolView, error) {
	if err := s.requireAdvantage(); err != nil {
		return nil, err
	}
	if s.adv.AssignmentRepo == nil {
		return nil, ErrLTILaunchNotFound
	}
	a, err := s.adv.AssignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil || a == nil || a.CourseID != courseID || a.WorkflowState == models.AssignmentDeleted {
		return nil, ErrLTILaunchNotFound
	}
	link, err := s.adv.Store.FindResourceLinkByAssignment(ctx, assignmentID, accountID)
	if err != nil || link == nil || link.ContextID != courseID {
		return nil, ErrLTILaunchNotFound
	}
	v := &AssignmentToolView{ToolID: link.ContextExternalToolID, URL: link.URL, Title: link.Title, ResourceLinkUUID: link.ResourceLinkID, Published: a.IsPublished()}
	if tool, tErr := s.adv.Store.FindActiveToolForCourse(ctx, link.ContextExternalToolID, courseID, accountID); tErr == nil && tool != nil {
		v.ToolName = tool.Name
	}
	if s.adv.LineItemRepo != nil {
		if li, liErr := s.adv.LineItemRepo.FindByAssignmentID(ctx, assignmentID); liErr == nil && li != nil {
			v.LineItemID = li.ID
		}
	}
	return v, nil
}

// createToolAssignment creates a published external_tool assignment for
// a tool (AGS line item creation, deep-linked graded content) and its
// default line item. Returns the assignment and the line item.
func (s *LTIService) createToolAssignment(ctx context.Context, courseID, accountID uint, tool *models.ContextExternalTool, devKey *models.DeveloperKey, title, url string, custom map[string]string, li *DeepLinkLineItem) (*models.Assignment, *models.LTILineItem, error) {
	pts := 100.0
	if li != nil && li.ScoreMaximum > 0 {
		pts = li.ScoreMaximum
	}
	a := &models.Assignment{
		CourseID: courseID, Name: title, PointsPossible: &pts, GradingType: "points",
		SubmissionTypes: "external_tool", WorkflowState: models.AssignmentPublished, Published: true,
	}
	if err := s.adv.AssignmentRepo.Create(ctx, a); err != nil {
		return nil, nil, err
	}
	link, err := s.AttachToolToAssignment(ctx, AssignmentToolInput{
		AccountID: accountID, CourseID: courseID, AssignmentID: a.ID, ToolID: tool.ID,
		URL: url, Title: title, Custom: custom, LineItem: li,
	})
	if err != nil {
		return nil, nil, err
	}
	_ = link
	var item *models.LTILineItem
	if s.adv.LineItemRepo != nil {
		item, _ = s.adv.LineItemRepo.FindByAssignmentID(ctx, a.ID)
	}
	return a, item, nil
}

// ModuleToolItemInput adds an external tool link to a module.
type ModuleToolItemInput struct {
	AccountID uint
	CourseID  uint
	ModuleID  uint
	ToolID    uint
	Title     string
	URL       string
	NewTab    bool
	Position  int
}

// CreateExternalToolModuleItem creates a ContextExternalTool module item
// (Canvas module item type ExternalTool) and its resource link (G131).
func (s *LTIService) CreateExternalToolModuleItem(ctx context.Context, in ModuleToolItemInput) (*models.ContentTag, error) {
	if err := s.requireAdvantage(); err != nil {
		return nil, err
	}
	if s.adv.ModuleRepo == nil || s.adv.ModuleItemRepo == nil {
		return nil, errors.New("modules are not configured")
	}
	module, err := s.adv.ModuleRepo.FindByID(ctx, in.ModuleID, in.AccountID)
	if err != nil || module == nil || module.CourseID != in.CourseID {
		return nil, ErrLTILaunchNotFound
	}
	tool, err := s.adv.Store.FindActiveToolForCourse(ctx, in.ToolID, in.CourseID, in.AccountID)
	if err != nil || tool == nil {
		return nil, ErrLTILaunchNotFound
	}
	if in.URL != "" && !validToolURL(in.URL) {
		return nil, ltiInvalid("launch URL must be an https URL")
	}
	title := SanitizePlainText(strings.TrimSpace(in.Title))
	if title == "" {
		title = tool.Name
	}
	pos := in.Position
	if pos <= 0 {
		pos, _ = s.adv.Store.NextModuleItemPosition(ctx, module.ID)
	}
	toolID := tool.ID
	tag := &models.ContentTag{
		ContextModuleID: module.ID, Title: title, ContentType: ContentTypeExternalTool, ContentID: &toolID,
		URL: in.URL, NewTab: in.NewTab, Position: pos, WorkflowState: models.ContentTagActive,
	}
	if err := s.adv.ModuleItemRepo.Create(ctx, tag); err != nil {
		return nil, err
	}
	tagID := tag.ID
	link := &models.LTIResourceLink{
		ContextExternalToolID: tool.ID, ContextType: "Course", ContextID: in.CourseID,
		ResourceLinkID: uuid.NewString(), LookupUUID: uuid.NewString(), Title: title, URL: in.URL,
		ContentTagID: &tagID, Placement: LTIPlacementLinkSelection,
	}
	if err := s.adv.Store.CreateResourceLink(ctx, link); err != nil {
		return nil, err
	}
	return tag, nil
}
