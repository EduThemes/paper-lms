package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// LTILaunchRequest asks for a launch of an installed tool in a course
// (G131). Exactly one of ResourceLinkUUID / ModuleItemID / AssignmentID
// names an existing resource link; otherwise it is a placement launch
// (course_navigation, editor_button, ...).
type LTILaunchRequest struct {
	UserID    uint
	AccountID uint
	// IsAdmin is the DB-verified tenant-admin flag.
	IsAdmin bool
	// IsStaff is true for a teacher/TA of the course (or an admin);
	// deep linking (content selection) is staff-only.
	IsStaff bool

	CourseID         uint
	ToolID           uint
	Placement        string
	ResourceLinkUUID string
	ModuleItemID     uint
	AssignmentID     uint
	// DeepLink, when set, makes this an LtiDeepLinkingRequest whose
	// content goes to the target.
	DeepLink *LTIDeepLinkTarget
	Locale   string
}

// LTILaunchResult is how the browser continues the launch: an LTI 1.3
// launch redirects (GET) to the tool's OIDC login initiation URL; an LTI
// 1.1 launch auto-posts FormParams to FormAction.
type LTILaunchResult struct {
	RedirectURL string
	FormAction  string
	FormParams  map[string]string
}

func (s *LTIService) requireAdvantage() error {
	if s.adv.Store == nil {
		return errors.New("LTI Advantage is not configured")
	}
	return nil
}

// PrepareLaunch authorizes a launch and returns where the browser goes.
// Every "unknown / other tenant / not installed / not enrolled" case is
// ErrLTILaunchNotFound (existence-leak contract).
func (s *LTIService) PrepareLaunch(ctx context.Context, req LTILaunchRequest) (*LTILaunchResult, error) {
	if err := s.requireAdvantage(); err != nil {
		return nil, err
	}
	s.RefreshPlatformKeys(ctx)

	course, err := s.courseRepo.FindByID(ctx, req.CourseID, req.AccountID)
	if err != nil || course == nil {
		return nil, ErrLTILaunchNotFound
	}
	if !req.IsAdmin {
		enrollment, eErr := s.enrollRepo.FindByUserAndCourse(ctx, req.UserID, req.CourseID, req.AccountID)
		if eErr != nil || enrollment == nil || enrollment.WorkflowState != "active" {
			return nil, ErrLTILaunchNotFound
		}
	}
	tool, err := s.adv.Store.FindActiveToolForCourse(ctx, req.ToolID, req.CourseID, req.AccountID)
	if err != nil || tool == nil {
		return nil, ErrLTILaunchNotFound
	}

	deepLinking := req.DeepLink != nil
	if deepLinking && !req.IsStaff && req.DeepLink.Kind != DeepLinkTargetSubmission {
		return nil, ErrLTILaunchNotFound
	}
	if deepLinking && req.DeepLink.Kind == DeepLinkTargetSubmission && req.DeepLink.AssignmentID == 0 {
		return nil, ltiInvalid("assignment_id is required to submit from a tool")
	}
	// Submitting from a tool is opening the assignment: a locked or
	// unavailable one cannot be reached this way either.
	if deepLinking && req.DeepLink.Kind == DeepLinkTargetSubmission && !s.learnerMayLaunch(ctx, req, 0, req.DeepLink.AssignmentID) {
		return nil, ErrLTILaunchNotFound
	}

	var link *models.LTIResourceLink
	placement := req.Placement
	if !deepLinking {
		link, err = s.resolveResourceLink(ctx, req, tool)
		if err != nil {
			return nil, err
		}
		if link != nil && (req.ResourceLinkUUID != "" || req.ModuleItemID != 0 || req.AssignmentID != 0) {
			placement = LTIPlacementResourceLink
		}
	}

	if tool.IsLTI11() {
		if deepLinking {
			return nil, ltiInvalid("LTI 1.1 tools do not support content selection")
		}
		return s.buildLTI11Launch(ctx, req, course, tool, link)
	}

	devKey, err := s.devKeyRepo.FindByID(ctx, tool.DeveloperKeyID, req.AccountID)
	if err != nil || devKey == nil || devKey.WorkflowState != "active" || !devKey.IsLTIKey {
		return nil, ErrLTILaunchNotFound
	}
	cfg, err := s.ltiConfigRepo.FindByDeveloperKeyID(ctx, devKey.ID)
	if err != nil || cfg == nil {
		return nil, ErrLTILaunchNotFound
	}

	target := cfg.TargetLinkURI
	documentTarget := "iframe"
	messageType := LTIMessageResourceLink
	switch {
	case deepLinking:
		pc, ok := findPlacement(cfg, placement)
		if !ok || pc.EffectiveMessageType() != LTIMessageDeepLinking {
			return nil, ltiInvalid("this tool does not support content selection from " + placement)
		}
		messageType = LTIMessageDeepLinking
		if pc.TargetLinkURI != "" {
			target = pc.TargetLinkURI
		}
	case placement == LTIPlacementResourceLink:
		if link != nil && link.URL != "" {
			target = link.URL
		}
	case placement == "":
		// A plain launch of the tool (e.g. from the course's tool list)
		// goes to its default target link URI.
	default:
		pc, ok := findPlacement(cfg, placement)
		if !ok {
			return nil, ErrLTILaunchNotFound
		}
		if pc.TargetLinkURI != "" {
			target = pc.TargetLinkURI
		}
	}

	hint := ltiLaunchHint{
		UserID: req.UserID, AccountID: req.AccountID, CourseID: req.CourseID, ToolID: tool.ID,
		DevKeyID: devKey.ID, Placement: placement, MessageType: messageType, TargetLinkURI: target,
		DocumentTarget: documentTarget, DeepLink: req.DeepLink, Locale: req.Locale,
	}
	if link != nil {
		hint.ResourceLinkUUID = link.ResourceLinkID
		if link.AssignmentID != nil {
			hint.AssignmentID = *link.AssignmentID
		}
	}
	signedHint, err := s.signHint(hint)
	if err != nil {
		return nil, err
	}

	initURL, err := url.Parse(cfg.OIDCInitiationURL)
	if err != nil || initURL.Scheme == "" {
		return nil, ltiInvalid("the tool's OIDC login URL is invalid")
	}
	q := initURL.Query()
	q.Set("iss", s.PlatformIssuer)
	q.Set("login_hint", strconv.FormatUint(uint64(req.UserID), 10))
	q.Set("target_link_uri", target)
	q.Set("lti_message_hint", signedHint)
	q.Set("client_id", devKey.ClientID)
	q.Set("lti_deployment_id", strconv.FormatUint(uint64(tool.ID), 10))
	initURL.RawQuery = q.Encode()
	return &LTILaunchResult{RedirectURL: initURL.String()}, nil
}

// resolveResourceLink finds (or lazily creates) the resource link a
// launch addresses. A placement launch reuses one course-level link per
// tool and placement.
func (s *LTIService) resolveResourceLink(ctx context.Context, req LTILaunchRequest, tool *models.ContextExternalTool) (*models.LTIResourceLink, error) {
	store := s.adv.Store
	switch {
	case req.ResourceLinkUUID != "":
		link, err := store.FindResourceLinkByUUID(ctx, req.ResourceLinkUUID, req.AccountID)
		if err != nil || link == nil || link.ContextID != req.CourseID || link.ContextExternalToolID != tool.ID {
			return nil, ErrLTILaunchNotFound
		}
		// A link attached to a module item or an assignment is visible to
		// a student only when that item is (the UUID must not be a way
		// around the module_item_id / assignment_id checks below).
		if !req.IsStaff {
			var tagID, assignmentID uint
			if link.ContentTagID != nil {
				tag, tErr := store.FindContentTagInCourse(ctx, *link.ContentTagID, req.CourseID, req.AccountID)
				if tErr != nil || tag == nil || !s.studentSeesModuleItem(ctx, tag, req) {
					return nil, ErrLTILaunchNotFound
				}
				tagID = tag.ID
			}
			if link.AssignmentID != nil && s.adv.AssignmentRepo != nil {
				a, aErr := s.adv.AssignmentRepo.FindByID(ctx, *link.AssignmentID, req.AccountID)
				if aErr != nil || a == nil || a.CourseID != req.CourseID || !a.IsPublished() {
					return nil, ErrLTILaunchNotFound
				}
				assignmentID = a.ID
			}
			// Module locks and the assignment's availability apply too.
			if !s.learnerMayLaunch(ctx, req, tagID, assignmentID) {
				return nil, ErrLTILaunchNotFound
			}
		}
		return link, nil
	case req.ModuleItemID != 0:
		tag, err := store.FindContentTagInCourse(ctx, req.ModuleItemID, req.CourseID, req.AccountID)
		if err != nil || tag == nil || tag.ContentType != ContentTypeExternalTool || tag.ContentID == nil || *tag.ContentID != tool.ID {
			return nil, ErrLTILaunchNotFound
		}
		if !req.IsStaff && (!s.studentSeesModuleItem(ctx, tag, req) || !s.learnerMayLaunch(ctx, req, tag.ID, 0)) {
			return nil, ErrLTILaunchNotFound
		}
		if link, err := store.FindResourceLinkByContentTag(ctx, tag.ID, req.AccountID); err == nil && link != nil {
			return link, nil
		}
		tagID := tag.ID
		link := &models.LTIResourceLink{
			ContextExternalToolID: tool.ID, ContextType: "Course", ContextID: req.CourseID,
			ResourceLinkID: uuid.NewString(), LookupUUID: uuid.NewString(), Title: tag.Title, URL: tag.URL,
			ContentTagID: &tagID, Placement: LTIPlacementLinkSelection,
		}
		if err := store.CreateResourceLink(ctx, link); err != nil {
			return nil, err
		}
		return link, nil
	case req.AssignmentID != 0:
		if s.adv.AssignmentRepo == nil {
			return nil, ErrLTILaunchNotFound
		}
		a, err := s.adv.AssignmentRepo.FindByID(ctx, req.AssignmentID, req.AccountID)
		if err != nil || a == nil || a.CourseID != req.CourseID || a.WorkflowState == models.AssignmentDeleted {
			return nil, ErrLTILaunchNotFound
		}
		if !req.IsStaff && (!a.IsPublished() || !s.learnerMayLaunch(ctx, req, 0, a.ID)) {
			return nil, ErrLTILaunchNotFound
		}
		link, err := store.FindResourceLinkByAssignment(ctx, a.ID, req.AccountID)
		if err != nil || link == nil || link.ContextExternalToolID != tool.ID {
			return nil, ErrLTILaunchNotFound
		}
		return link, nil
	}

	placement := req.Placement
	if link, err := store.FindPlacementResourceLink(ctx, tool.ID, req.CourseID, placement); err == nil && link != nil {
		return link, nil
	}
	link := &models.LTIResourceLink{
		ContextExternalToolID: tool.ID, ContextType: "Course", ContextID: req.CourseID,
		ResourceLinkID: uuid.NewString(), LookupUUID: uuid.NewString(), Title: tool.Name, Placement: placement,
	}
	if err := store.CreateResourceLink(ctx, link); err != nil {
		return nil, err
	}
	return link, nil
}

// studentSeesModuleItem reports whether a non-staff caller may open a
// module item: the item and its module must both be published.
func (s *LTIService) studentSeesModuleItem(ctx context.Context, tag *models.ContentTag, req LTILaunchRequest) bool {
	if tag.WorkflowState != models.ContentTagActive {
		return false
	}
	if s.adv.ModuleRepo == nil {
		return true
	}
	module, err := s.adv.ModuleRepo.FindByID(ctx, tag.ContextModuleID, req.AccountID)
	return err == nil && module != nil && module.CourseID == req.CourseID && module.WorkflowState == "active"
}

// ContentTypeExternalTool is the module item (content tag) content type
// of an LTI tool link (Canvas: ContextExternalTool).
const ContentTypeExternalTool = "ContextExternalTool"

// authorizeHintLaunch is AuthorizeLaunch for a platform-signed hint: the
// hint carries the whole launch context; the session user, tenant and
// client must match it, and the course/enrollment/install checks are
// re-run (the hint may be minutes old).
func (s *LTIService) authorizeHintLaunch(ctx context.Context, req LTIAuthRequest) (*LTIAuthResponse, error) {
	hint, err := s.parseHint(req.LTIMessageHint)
	if err != nil {
		return nil, ltiInvalid("invalid lti_message_hint")
	}
	if hint.UserID != req.UserID || hint.AccountID != req.AccountID {
		return nil, ErrLTILaunchNotFound
	}
	devKey, toolConfig, err := s.resolveLTIKey(ctx, req.ClientID, req.AccountID)
	if err != nil {
		return nil, err
	}
	if devKey.ID != hint.DevKeyID {
		return nil, ltiInvalid("lti_message_hint does not match client_id")
	}
	// The id_token goes only to a registered redirect URI, or to the
	// launch's own target link URI when it is on the tool's host.
	targetOK := req.RedirectURI == hint.TargetLinkURI && sameHost(hint.TargetLinkURI, toolConfig.TargetLinkURI, toolConfig.Domain)
	if req.RedirectURI != toolConfig.TargetLinkURI && !targetOK && !DeveloperKeyAllowsRedirect(devKey, req.RedirectURI) {
		return nil, ltiInvalid("redirect_uri is not registered for this tool")
	}
	if _, err := s.courseRepo.FindByID(ctx, hint.CourseID, req.AccountID); err != nil {
		return nil, ErrLTILaunchNotFound
	}
	if !req.IsAdmin {
		enrollment, eErr := s.enrollRepo.FindByUserAndCourse(ctx, req.UserID, hint.CourseID, req.AccountID)
		if eErr != nil || enrollment == nil || enrollment.WorkflowState != "active" {
			return nil, ErrLTILaunchNotFound
		}
	}
	tool, err := s.adv.Store.FindActiveToolForCourse(ctx, hint.ToolID, hint.CourseID, req.AccountID)
	if err != nil || tool == nil || tool.DeveloperKeyID != devKey.ID {
		return nil, ErrLTILaunchNotFound
	}
	if err := s.consumeToolNonce(ctx, req.ClientID, req.Nonce); err != nil {
		return nil, err
	}

	idToken, err := s.buildIDToken(ctx, launchContext{
		hint: *hint, nonce: req.Nonce, devKey: devKey, cfg: toolConfig, tool: tool, isAdmin: req.IsAdmin,
	})
	if err != nil {
		return nil, err
	}
	return &LTIAuthResponse{RedirectURI: req.RedirectURI, IDToken: idToken, State: req.State}, nil
}

// consumeToolNonce enforces the single use of a tool's OIDC nonce.
func (s *LTIService) consumeToolNonce(ctx context.Context, clientID, nonce string) error {
	nonceKey := "lti:" + clientID + ":" + nonce
	if used, err := s.nonceRepo.Exists(ctx, nonceKey); err != nil || used {
		return ltiInvalid("nonce has already been used")
	}
	if err := s.nonceRepo.Create(ctx, &models.Nonce{Value: nonceKey, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		return ltiInvalid("nonce has already been used")
	}
	return nil
}

// launchContext is the input of the id_token builder.
type launchContext struct {
	hint    ltiLaunchHint
	nonce   string
	devKey  *models.DeveloperKey
	cfg     *models.LTIToolConfiguration
	tool    *models.ContextExternalTool // nil for a legacy launch
	isAdmin bool
}

// LTI role vocabularies.
const (
	ltiRoleLearner        = "http://purl.imsglobal.org/vocab/lis/v2/membership#Learner"
	ltiRoleInstructor     = "http://purl.imsglobal.org/vocab/lis/v2/membership#Instructor"
	ltiRoleTA             = "http://purl.imsglobal.org/vocab/lis/v2/membership/Instructor#TeachingAssistant"
	ltiRoleMentor         = "http://purl.imsglobal.org/vocab/lis/v2/membership#Mentor"
	ltiRoleContentDev     = "http://purl.imsglobal.org/vocab/lis/v2/membership#ContentDeveloper"
	ltiRoleMemberAdmin    = "http://purl.imsglobal.org/vocab/lis/v2/membership#Administrator"
	ltiRoleInstAdmin      = "http://purl.imsglobal.org/vocab/lis/v2/institution/person#Administrator"
	ltiRoleSysAdmin       = "http://purl.imsglobal.org/vocab/lis/v2/system/person#SysAdmin"
	ltiRoleSysUser        = "http://purl.imsglobal.org/vocab/lis/v2/system/person#User"
	ltiRoleInstStudent    = "http://purl.imsglobal.org/vocab/lis/v2/institution/person#Student"
	ltiRoleInstInstructor = "http://purl.imsglobal.org/vocab/lis/v2/institution/person#Instructor"
)

// launchRoles builds the context roles (from the course enrollment) and
// institution/system roles (from the user's account role) of a launch.
func launchRoles(enrollmentType string, user *models.User, isAdmin bool) []string {
	roles := []string{}
	switch enrollmentType {
	case "StudentEnrollment":
		roles = append(roles, ltiRoleLearner, ltiRoleInstStudent)
	case "TeacherEnrollment":
		roles = append(roles, ltiRoleInstructor, ltiRoleInstInstructor)
	case "TaEnrollment":
		roles = append(roles, ltiRoleTA, ltiRoleInstructor)
	case "ObserverEnrollment":
		roles = append(roles, ltiRoleMentor)
	case "DesignerEnrollment":
		roles = append(roles, ltiRoleContentDev)
	}
	adminRole := isAdmin || (user != nil && (user.Role == models.RoleAdmin || user.Role == models.RoleSuperUser))
	if adminRole {
		if enrollmentType == "" {
			roles = append(roles, ltiRoleMemberAdmin)
		}
		roles = append(roles, ltiRoleInstAdmin)
	}
	if user != nil && user.Role == models.RoleSuperUser {
		roles = append(roles, ltiRoleSysAdmin)
	}
	roles = append(roles, ltiRoleSysUser)
	return roles
}

// buildIDToken assembles and signs the LTI 1.3 id_token (G125): identity
// claims gated by the tool's privacy level, context/institution roles,
// custom parameters with substitution variables, the AGS and NRPS
// endpoint claims (when the tool holds those scopes), launch
// presentation, the per-install deployment_id, and for deep linking the
// deep_linking_settings claim.
func (s *LTIService) buildIDToken(ctx context.Context, lc launchContext) (string, error) {
	key, kid := s.currentKey()
	if key == nil {
		return "", errors.New("platform key has not been initialized")
	}
	h := lc.hint
	course, err := s.courseRepo.FindByID(ctx, h.CourseID, h.AccountID)
	if err != nil || course == nil {
		return "", ErrLTILaunchNotFound
	}

	var user *models.User
	if s.adv.UserRepo != nil {
		user, _ = s.adv.UserRepo.FindByID(ctx, h.UserID, h.AccountID)
	}
	enrollmentType := ""
	if e, eErr := s.enrollRepo.FindByUserAndCourse(ctx, h.UserID, h.CourseID, h.AccountID); eErr == nil && e != nil && e.WorkflowState == "active" {
		enrollmentType = e.Type
	}
	roles := launchRoles(enrollmentType, user, lc.isAdmin)

	privacy := LTIPrivacyAnonymous
	if lc.cfg != nil && ValidLTIPrivacyLevel(lc.cfg.PrivacyLevel) {
		privacy = lc.cfg.PrivacyLevel
	}

	var link *models.LTIResourceLink
	if h.ResourceLinkUUID != "" && s.adv.Store != nil {
		link, _ = s.adv.Store.FindResourceLinkByUUID(ctx, h.ResourceLinkUUID, h.AccountID)
	}
	var assignment *models.Assignment
	if h.AssignmentID != 0 && s.adv.AssignmentRepo != nil {
		if a, aErr := s.adv.AssignmentRepo.FindByID(ctx, h.AssignmentID, h.AccountID); aErr == nil && a != nil && a.CourseID == h.CourseID {
			assignment = a
		}
	}

	deploymentID := strconv.FormatUint(uint64(lc.devKey.ID), 10)
	if lc.tool != nil {
		deploymentID = strconv.FormatUint(uint64(lc.tool.ID), 10)
	}
	messageType := h.MessageType
	if messageType == "" {
		messageType = LTIMessageResourceLink
	}
	target := h.TargetLinkURI
	if target == "" && lc.cfg != nil {
		target = lc.cfg.TargetLinkURI
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   s.PlatformIssuer,
		"sub":   strconv.FormatUint(uint64(h.UserID), 10),
		"aud":   lc.devKey.ClientID,
		"azp":   lc.devKey.ClientID,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
		"nonce": lc.nonce,

		"https://purl.imsglobal.org/spec/lti/claim/message_type":    messageType,
		"https://purl.imsglobal.org/spec/lti/claim/version":         "1.3.0",
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id":   deploymentID,
		"https://purl.imsglobal.org/spec/lti/claim/target_link_uri": target,
		"https://purl.imsglobal.org/spec/lti/claim/roles":           roles,
		"https://purl.imsglobal.org/spec/lti/claim/context": map[string]interface{}{
			"id":    strconv.FormatUint(uint64(h.CourseID), 10),
			"label": course.CourseCode,
			"title": course.Name,
			"type":  []string{"http://purl.imsglobal.org/vocab/lis/v2/course#CourseOffering"},
		},
		"https://purl.imsglobal.org/spec/lti/claim/tool_platform": map[string]interface{}{
			"guid":                s.PlatformIssuer,
			"name":                "Paper LMS",
			"product_family_code": "paper-lms",
			"version":             "1.0.0",
		},
	}

	// Launch presentation (LTI Core §5.4.4).
	pres := map[string]interface{}{
		"document_target": firstNonEmpty(h.DocumentTarget, "iframe"),
		"return_url":      s.courseURL(h.CourseID, ""),
	}
	locale := h.Locale
	if locale == "" && user != nil {
		locale = user.Locale
	}
	if locale != "" {
		pres["locale"] = locale
	}
	claims["https://purl.imsglobal.org/spec/lti/claim/launch_presentation"] = pres
	if h.Placement != "" {
		claims["https://www.instructure.com/placement"] = h.Placement
	}

	// Identity claims by privacy level.
	if user != nil {
		if privacyAllowsName(privacy) {
			claims["name"] = user.Name
			claims["given_name"] = givenName(user.Name)
			claims["family_name"] = familyName(user.Name)
			if user.AvatarURL != "" && privacy == LTIPrivacyPublic {
				claims["picture"] = user.AvatarURL
			}
		}
		if privacyAllowsEmail(privacy) && user.Email != "" {
			claims["email"] = user.Email
		}
		if locale != "" {
			claims["locale"] = locale
		}
	}
	lis := map[string]interface{}{}
	if privacyAllowsSIS(privacy) && user != nil && user.SISUserID != nil && *user.SISUserID != "" {
		lis["person_sourcedid"] = *user.SISUserID
	}
	if course.SISCourseID != nil && *course.SISCourseID != "" {
		lis["course_offering_sourcedid"] = *course.SISCourseID
	}
	if len(lis) > 0 {
		claims["https://purl.imsglobal.org/spec/lti/claim/lis"] = lis
	}

	// Custom parameters: tool configuration < install < resource link,
	// then substitution variables.
	custom := map[string]string{}
	if lc.cfg != nil {
		for k, v := range ParseStringMap(lc.cfg.CustomFields) {
			custom[k] = v
		}
	}
	if lc.tool != nil {
		for k, v := range ParseStringMap(lc.tool.CustomFields) {
			custom[k] = v
		}
	}
	if link != nil {
		for k, v := range ParseStringMap(link.CustomParameters) {
			custom[k] = v
		}
	}
	if len(custom) > 0 {
		expanded := ExpandLTIVariables(custom, &LTIVariableContext{
			Issuer: s.PlatformIssuer, AccountID: h.AccountID, User: user, Course: course, Assignment: assignment,
			ResourceLink: link, EnrollmentType: enrollmentType, Roles: roles, PrivacyLevel: privacy, Locale: locale,
		})
		claims["https://purl.imsglobal.org/spec/lti/claim/custom"] = expanded
	}

	// AGS / NRPS service endpoint claims (only when the tool holds the
	// scopes, so it never learns of services it cannot call).
	var scopes []string
	if lc.cfg != nil {
		scopes = ParseScopeList(lc.cfg.Scopes)
	}
	if scopeListHas(scopes, LTIScopeLineItem, LTIScopeLineItemReadOnly, LTIScopeScore, LTIScopeResultReadOnly) {
		var granted []string
		for _, sc := range scopes {
			if strings.Contains(sc, "/spec/lti-ags/") {
				granted = append(granted, sc)
			}
		}
		lineItems := fmt.Sprintf("%s/lti/courses/%d/line_items", s.APIBaseURL(), h.CourseID)
		ags := map[string]interface{}{"scope": granted, "lineitems": lineItems}
		if assignment != nil && s.adv.LineItemRepo != nil {
			if li, liErr := s.adv.LineItemRepo.FindByAssignmentID(ctx, assignment.ID); liErr == nil && li != nil {
				ags["lineitem"] = fmt.Sprintf("%s/%d", lineItems, li.ID)
			}
		}
		claims["https://purl.imsglobal.org/spec/lti-ags/claim/endpoint"] = ags
	}
	if scopeListHas(scopes, LTIScopeNRPS) {
		claims["https://purl.imsglobal.org/spec/lti-nrps/claim/namesroleservice"] = map[string]interface{}{
			"context_memberships_url": fmt.Sprintf("%s/lti/courses/%d/names_and_roles", s.APIBaseURL(), h.CourseID),
			"service_versions":        []string{"2.0"},
		}
	}

	if messageType == LTIMessageDeepLinking {
		settings, sErr := s.deepLinkingSettings(h, lc.devKey.ID)
		if sErr != nil {
			return "", sErr
		}
		claims["https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings"] = settings
	} else {
		rl := map[string]interface{}{"id": h.ResourceLinkUUID}
		if rl["id"] == "" {
			// Legacy launches without a stored link still need an id.
			rl["id"] = fmt.Sprintf("%d-%d", h.CourseID, lc.devKey.ID)
		}
		title := ""
		if link != nil {
			title = link.Title
		}
		if title == "" && lc.tool != nil {
			title = lc.tool.Name
		}
		if title == "" && lc.cfg != nil {
			title = lc.cfg.Title
		}
		rl["title"] = title
		claims["https://purl.imsglobal.org/spec/lti/claim/resource_link"] = rl
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("failed to sign launch token: %w", err)
	}
	return signed, nil
}

// sameHost reports whether raw's host is reference's host or domain (or
// a subdomain of domain).
func sameHost(raw, reference, domain string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if r, rErr := url.Parse(reference); rErr == nil && r.Host != "" && strings.ToLower(r.Hostname()) == h {
		return true
	}
	d := strings.ToLower(strings.TrimSpace(domain))
	return d != "" && (h == d || strings.HasSuffix(h, "."+d))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
