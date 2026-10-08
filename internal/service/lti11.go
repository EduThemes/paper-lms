package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // OAuth 1.0a HMAC-SHA1 and oauth_body_hash are mandated by LTI 1.1
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// --- OAuth 1.0a signing (RFC 5849) ---------------------------------------------

// oauthPercentEncode is RFC 5849 §3.6 percent-encoding.
func oauthPercentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// oauthBaseURL normalizes the request URL (§3.4.1.2): lowercase scheme
// and host, default ports dropped, no query or fragment.
func oauthBaseURL(raw string) (string, url.Values, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", nil, errors.New("invalid launch URL")
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" && !((scheme == "http" && port == "80") || (scheme == "https" && port == "443")) {
		host += ":" + port
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	return scheme + "://" + host + path, u.Query(), nil
}

// OAuth1Signature computes the HMAC-SHA1 signature of a request whose
// parameters (body + oauth_*, excluding oauth_signature) are params.
// Query parameters of rawURL are included as the spec requires.
func OAuth1Signature(method, rawURL string, params map[string]string, consumerSecret string) (string, error) {
	base, query, err := oauthBaseURL(rawURL)
	if err != nil {
		return "", err
	}
	type kv struct{ k, v string }
	var pairs []kv
	for k, v := range params {
		if k == "oauth_signature" {
			continue
		}
		pairs = append(pairs, kv{oauthPercentEncode(k), oauthPercentEncode(v)})
	}
	for k, vs := range query {
		for _, v := range vs {
			pairs = append(pairs, kv{oauthPercentEncode(k), oauthPercentEncode(v)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k == pairs[j].k {
			return pairs[i].v < pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.k + "=" + p.v
	}
	baseString := strings.ToUpper(method) + "&" + oauthPercentEncode(base) + "&" + oauthPercentEncode(strings.Join(parts, "&"))
	mac := hmac.New(sha1.New, []byte(oauthPercentEncode(consumerSecret)+"&"))
	mac.Write([]byte(baseString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// --- LTI 1.1 launch (G133) -------------------------------------------------------

var customKeyRe = regexp.MustCompile(`[^a-z0-9]`)

func lti11Roles(enrollmentType string, isAdmin bool) string {
	var roles []string
	switch enrollmentType {
	case "StudentEnrollment":
		roles = append(roles, "Learner")
	case "TeacherEnrollment":
		roles = append(roles, "Instructor")
	case "TaEnrollment":
		roles = append(roles, "urn:lti:role:ims/lis/TeachingAssistant")
	case "ObserverEnrollment":
		roles = append(roles, "urn:lti:role:ims/lis/Mentor")
	case "DesignerEnrollment":
		roles = append(roles, "urn:lti:role:ims/lis/ContentDeveloper")
	}
	if isAdmin {
		roles = append(roles, "urn:lti:instrole:ims/lis/Administrator")
	}
	if len(roles) == 0 {
		roles = append(roles, "urn:lti:sysrole:ims/lis/User")
	}
	return strings.Join(roles, ",")
}

// outcomeKeys derives the HMAC keys of the LTI 1.1 sourcedid and opaque
// user_id. The first key signs: the stored, never-rotated "lti11" key
// when there is one (so sourcedids and user ids survive any number of
// platform key rotations), else the operator-managed env key, which
// never rotates either. Every replica derives the same keys.
func (s *LTIService) outcomeKeys() [][]byte {
	s.keyMu.RLock()
	defer s.keyMu.RUnlock()
	var out [][]byte
	add := func(k interface{}) {
		der, err := x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			return
		}
		sum := sha256.Sum256(append([]byte("paper-lms:lti11-outcomes:"), der...))
		out = append(out, sum[:])
	}
	if s.stableKey != nil {
		add(s.stableKey)
	} else if s.platformKey != nil {
		add(s.platformKey)
	}
	return out
}

func sourcedIDMAC(key []byte, base string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(base))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// signSourcedID builds the lis_result_sourcedid "tool:assignment:user:mac".
func (s *LTIService) signSourcedID(toolID, assignmentID, userID uint) string {
	base := fmt.Sprintf("%d:%d:%d", toolID, assignmentID, userID)
	keys := s.outcomeKeys()
	if len(keys) == 0 {
		return ""
	}
	return base + ":" + sourcedIDMAC(keys[0], base)
}

// parseSourcedID verifies a sourcedid and returns its parts.
func (s *LTIService) parseSourcedID(sid string) (toolID, assignmentID, userID uint, err error) {
	parts := strings.Split(strings.TrimSpace(sid), ":")
	if len(parts) != 4 {
		return 0, 0, 0, errors.New("invalid sourcedId")
	}
	base := strings.Join(parts[:3], ":")
	ok := false
	for _, k := range s.outcomeKeys() {
		if subtle.ConstantTimeCompare([]byte(sourcedIDMAC(k, base)), []byte(parts[3])) == 1 {
			ok = true
			break
		}
	}
	if !ok {
		return 0, 0, 0, errors.New("invalid sourcedId")
	}
	nums := make([]uint, 3)
	for i := 0; i < 3; i++ {
		n, perr := strconv.ParseUint(parts[i], 10, 64)
		if perr != nil || n == 0 {
			return 0, 0, 0, errors.New("invalid sourcedId")
		}
		nums[i] = uint(n)
	}
	return nums[0], nums[1], nums[2], nil
}

// opaqueUserID is the stable, non-PII LTI 1.1 user_id.
func (s *LTIService) opaqueUserID(userID uint) string {
	keys := s.outcomeKeys()
	if len(keys) == 0 {
		return strconv.FormatUint(uint64(userID), 10)
	}
	mac := hmac.New(sha256.New, keys[0])
	fmt.Fprintf(mac, "user:%d", userID)
	return hex.EncodeToString(mac.Sum(nil))[:40]
}

// OutcomesURL is the LTI 1.1 Basic Outcomes endpoint of an install.
func (s *LTIService) OutcomesURL(toolID uint) string {
	return fmt.Sprintf("%s/lti/v1p1/tools/%d/outcomes", s.APIBaseURL(), toolID)
}

// buildLTI11Launch signs an LTI 1.1 basic launch for an LTI 1.1 install.
func (s *LTIService) buildLTI11Launch(ctx context.Context, req LTILaunchRequest, course *models.Course, tool *models.ContextExternalTool, link *models.LTIResourceLink) (*LTILaunchResult, error) {
	launchURL := tool.URL
	if link != nil && link.URL != "" {
		launchURL = link.URL
	}
	if !validToolURL(launchURL) {
		return nil, ltiInvalid("the tool's launch URL is invalid")
	}
	if s.adv.OpenSecret == nil {
		return nil, errors.New("LTI 1.1 secrets cannot be opened")
	}
	secret, err := s.adv.OpenSecret(tool.SharedSecret)
	if err != nil || secret == "" {
		return nil, ltiInvalid("the tool has no shared secret")
	}

	var user *models.User
	if s.adv.UserRepo != nil {
		user, _ = s.adv.UserRepo.FindByID(ctx, req.UserID, req.AccountID)
	}
	enrollmentType := ""
	if e, eErr := s.enrollRepo.FindByUserAndCourse(ctx, req.UserID, req.CourseID, req.AccountID); eErr == nil && e != nil && e.WorkflowState == "active" {
		enrollmentType = e.Type
	}
	privacy := tool.PrivacyLevel
	if !ValidLTIPrivacyLevel(privacy) {
		privacy = LTIPrivacyAnonymous
	}

	resourceLinkID := fmt.Sprintf("paper-%d-%d", req.CourseID, tool.ID)
	title := tool.Name
	if link != nil {
		resourceLinkID = link.ResourceLinkID
		title = firstNonEmpty(link.Title, tool.Name)
	}
	p := map[string]string{
		"lti_message_type":                       "basic-lti-launch-request",
		"lti_version":                            "LTI-1p0",
		"resource_link_id":                       resourceLinkID,
		"resource_link_title":                    title,
		"user_id":                                s.opaqueUserID(req.UserID),
		"roles":                                  lti11Roles(enrollmentType, req.IsAdmin),
		"context_id":                             strconv.FormatUint(uint64(course.ID), 10),
		"context_title":                          course.Name,
		"context_label":                          course.CourseCode,
		"context_type":                           "CourseSection",
		"launch_presentation_document_target":    "iframe",
		"launch_presentation_return_url":         s.courseURL(course.ID, ""),
		"tool_consumer_info_product_family_code": "paper-lms",
		"tool_consumer_info_version":             "1.0.0",
		"tool_consumer_instance_guid":            s.PlatformIssuer,
		"tool_consumer_instance_name":            "Paper LMS",
		"oauth_callback":                         "about:blank",
	}
	locale := req.Locale
	if locale == "" && user != nil {
		locale = user.Locale
	}
	if locale != "" {
		p["launch_presentation_locale"] = locale
	}
	if user != nil {
		if privacyAllowsName(privacy) {
			p["lis_person_name_full"] = user.Name
			p["lis_person_name_given"] = givenName(user.Name)
			p["lis_person_name_family"] = familyName(user.Name)
		}
		if privacyAllowsEmail(privacy) && user.Email != "" {
			p["lis_person_contact_email_primary"] = user.Email
		}
		if privacyAllowsSIS(privacy) {
			if user.SISUserID != nil && *user.SISUserID != "" {
				p["lis_person_sourcedid"] = *user.SISUserID
			}
			if user.AvatarURL != "" {
				p["user_image"] = user.AvatarURL
			}
		}
	}

	// Basic Outcomes: graded launches of a student carry the service URL
	// and a signed sourcedid.
	var assignment *models.Assignment
	if link != nil && link.AssignmentID != nil && s.adv.AssignmentRepo != nil {
		if a, aErr := s.adv.AssignmentRepo.FindByID(ctx, *link.AssignmentID, req.AccountID); aErr == nil && a != nil && a.CourseID == course.ID {
			assignment = a
		}
	}
	if assignment != nil && enrollmentType == "StudentEnrollment" {
		p["lis_outcome_service_url"] = s.OutcomesURL(tool.ID)
		p["lis_result_sourcedid"] = s.signSourcedID(tool.ID, assignment.ID, req.UserID)
	}

	custom := ParseStringMap(tool.CustomFields)
	if link != nil {
		for k, v := range ParseStringMap(link.CustomParameters) {
			custom[k] = v
		}
	}
	expanded := ExpandLTIVariables(custom, &LTIVariableContext{
		Issuer: s.PlatformIssuer, AccountID: req.AccountID, User: user, Course: course, Assignment: assignment,
		ResourceLink: link, EnrollmentType: enrollmentType, Roles: strings.Split(p["roles"], ","), PrivacyLevel: privacy, Locale: locale,
	})
	for k, v := range expanded {
		p["custom_"+customKeyRe.ReplaceAllString(strings.ToLower(k), "_")] = v
	}

	p["oauth_consumer_key"] = tool.ConsumerKey
	p["oauth_signature_method"] = "HMAC-SHA1"
	p["oauth_timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
	p["oauth_nonce"] = randomHex(16)
	p["oauth_version"] = "1.0"
	sig, err := OAuth1Signature("POST", launchURL, p, secret)
	if err != nil {
		return nil, ltiInvalid(err.Error())
	}
	p["oauth_signature"] = sig
	return &LTILaunchResult{FormAction: launchURL, FormParams: p}, nil
}

// --- LTI 1.1 Basic Outcomes (G133) ------------------------------------------------

// parseOAuthHeader parses an `Authorization: OAuth k="v", ...` header.
func parseOAuthHeader(h string) map[string]string {
	out := map[string]string{}
	h = strings.TrimSpace(h)
	if !strings.HasPrefix(strings.ToLower(h), "oauth ") {
		return out
	}
	for _, part := range strings.Split(h[6:], ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		v := strings.Trim(kv[1], `"`)
		if dec, err := url.QueryUnescape(strings.ReplaceAll(v, "+", "%2B")); err == nil {
			v = dec
		}
		out[strings.TrimSpace(kv[0])] = v
	}
	return out
}

type poxRequest struct {
	XMLName xml.Name `xml:"imsx_POXEnvelopeRequest"`
	Header  struct {
		MessageID string `xml:"imsx_POXRequestHeaderInfo>imsx_messageIdentifier"`
	} `xml:"imsx_POXHeader"`
	Body struct {
		Replace *poxResultRequest `xml:"replaceResultRequest"`
		Read    *poxResultRequest `xml:"readResultRequest"`
		Delete  *poxResultRequest `xml:"deleteResultRequest"`
	} `xml:"imsx_POXBody"`
}

type poxResultRequest struct {
	SourcedID string `xml:"resultRecord>sourcedGUID>sourcedId"`
	Score     string `xml:"resultRecord>result>resultScore>textString"`
}

// LTI11OutcomeResponse is the POX envelope to send back.
type LTI11OutcomeResponse struct {
	Status int
	XML    string
}

func poxResponse(msgRef, operation, codeMajor, description, body string) string {
	if body == "" {
		body = "<" + operation + "Response/>"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<imsx_POXEnvelopeResponse xmlns="http://www.imsglobal.org/services/ltiv1p1/xsd/imsoms_v1p0">
  <imsx_POXHeader>
    <imsx_POXResponseHeaderInfo>
      <imsx_version>V1.0</imsx_version>
      <imsx_messageIdentifier>` + randomHex(8) + `</imsx_messageIdentifier>
      <imsx_statusInfo>
        <imsx_codeMajor>` + codeMajor + `</imsx_codeMajor>
        <imsx_severity>status</imsx_severity>
        <imsx_description>` + html.EscapeString(description) + `</imsx_description>
        <imsx_messageRefIdentifier>` + html.EscapeString(msgRef) + `</imsx_messageRefIdentifier>
        <imsx_operationRefIdentifier>` + operation + `</imsx_operationRefIdentifier>
      </imsx_statusInfo>
    </imsx_POXResponseHeaderInfo>
  </imsx_POXHeader>
  <imsx_POXBody>` + body + `</imsx_POXBody>
</imsx_POXEnvelopeResponse>`
}

// HandleLTI11Outcome processes a Basic Outcomes POX request posted by an
// LTI 1.1 install (replaceResult / readResult / deleteResult). The
// request must be OAuth1 signed with the install's consumer key and
// shared secret and carry a matching oauth_body_hash; the sourcedid is
// Paper-signed, so a tool can only grade launches it actually received.
func (s *LTIService) HandleLTI11Outcome(ctx context.Context, toolID uint, requestURL, authHeader string, body []byte) LTI11OutcomeResponse {
	fail := func(status int, msgRef, op, desc string) LTI11OutcomeResponse {
		if op == "" {
			op = "unknown"
		}
		return LTI11OutcomeResponse{Status: status, XML: poxResponse(msgRef, op, "failure", desc, "")}
	}
	if s.toolRepo == nil || s.adv.Grader == nil || s.adv.OpenSecret == nil {
		return fail(500, "", "", "outcomes service is not configured")
	}
	// AUTH-INTERNAL: the tool id comes from the URL and the caller is
	// authenticated below by the install's own OAuth1 secret; the tenant
	// is derived from the install.
	tool, err := s.toolRepo.FindByID(ctx, toolID, 0)
	if err != nil || tool == nil || !tool.IsLTI11() || tool.WorkflowState != "active" {
		return fail(401, "", "", "unknown tool")
	}
	oauth := parseOAuthHeader(authHeader)
	if oauth["oauth_consumer_key"] != tool.ConsumerKey || oauth["oauth_signature_method"] != "HMAC-SHA1" {
		return fail(401, "", "", "invalid OAuth consumer")
	}
	ts, _ := strconv.ParseInt(oauth["oauth_timestamp"], 10, 64)
	if math.Abs(float64(time.Now().Unix()-ts)) > 300 {
		return fail(401, "", "", "OAuth timestamp is out of range")
	}
	sum := sha1.Sum(body) //nolint:gosec // oauth_body_hash is SHA-1 by spec
	if oauth["oauth_body_hash"] != base64.StdEncoding.EncodeToString(sum[:]) {
		return fail(401, "", "", "oauth_body_hash does not match")
	}
	secret, err := s.adv.OpenSecret(tool.SharedSecret)
	if err != nil || secret == "" {
		return fail(401, "", "", "invalid OAuth consumer")
	}
	params := map[string]string{}
	for k, v := range oauth {
		if strings.HasPrefix(k, "oauth_") && k != "oauth_signature" && k != "realm" {
			params[k] = v
		}
	}
	want, err := OAuth1Signature("POST", requestURL, params, secret)
	if err != nil || subtle.ConstantTimeCompare([]byte(want), []byte(oauth["oauth_signature"])) != 1 {
		return fail(401, "", "", "invalid OAuth signature")
	}
	if err := s.consumeJTI(ctx, "lti11", tool.ConsumerKey, oauth["oauth_nonce"], 10*time.Minute); err != nil {
		return fail(401, "", "", "OAuth nonce has already been used")
	}

	var req poxRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		return fail(400, "", "", "malformed POX request")
	}
	msgRef := req.Header.MessageID
	var op string
	var rr *poxResultRequest
	switch {
	case req.Body.Replace != nil:
		op, rr = "replaceResult", req.Body.Replace
	case req.Body.Read != nil:
		op, rr = "readResult", req.Body.Read
	case req.Body.Delete != nil:
		op, rr = "deleteResult", req.Body.Delete
	default:
		return LTI11OutcomeResponse{Status: 200, XML: poxResponse(msgRef, "unknown", "unsupported", "operation is not supported", "")}
	}

	sidTool, assignmentID, userID, err := s.parseSourcedID(rr.SourcedID)
	if err != nil || sidTool != tool.ID {
		return fail(200, msgRef, op, "invalid sourcedId")
	}
	accountID, courseOK := s.toolAccountID(ctx, tool)
	if !courseOK || s.adv.AssignmentRepo == nil {
		return fail(200, msgRef, op, "invalid sourcedId")
	}
	assignment, err := s.adv.AssignmentRepo.FindByID(ctx, assignmentID, accountID)
	if err != nil || assignment == nil || (tool.ContextType == "Course" && assignment.CourseID != tool.ContextID) {
		return fail(200, msgRef, op, "invalid sourcedId")
	}
	if link, lErr := s.adv.Store.FindResourceLinkByAssignment(ctx, assignment.ID, accountID); lErr != nil || link == nil || link.ContextExternalToolID != tool.ID {
		return fail(200, msgRef, op, "assignment is not linked to this tool")
	}
	enrollment, err := s.enrollRepo.FindByUserAndCourse(ctx, userID, assignment.CourseID, accountID)
	if err != nil || enrollment == nil || enrollment.WorkflowState != "active" || enrollment.Type != "StudentEnrollment" {
		return fail(200, msgRef, op, "user is not a student in the course")
	}
	pts := 0.0
	if assignment.PointsPossible != nil {
		pts = *assignment.PointsPossible
	}

	switch op {
	case "replaceResult":
		score, perr := strconv.ParseFloat(strings.TrimSpace(rr.Score), 64)
		if perr != nil || math.IsNaN(score) || score < 0 || score > 1 {
			return fail(200, msgRef, op, "resultScore must be a decimal between 0.0 and 1.0")
		}
		grade := strconv.FormatFloat(score*pts, 'f', -1, 64)
		if _, gErr := s.adv.Grader.Grade(ctx, assignment.ID, userID, 0, accountID, grade); gErr != nil {
			return fail(200, msgRef, op, gErr.Error())
		}
		s.recordLTI11Result(ctx, assignment.ID, userID, &score)
		return LTI11OutcomeResponse{Status: 200, XML: poxResponse(msgRef, op, "success", fmt.Sprintf("Score for %s is now %s", rr.SourcedID, strconv.FormatFloat(score, 'f', -1, 64)), "")}
	case "deleteResult":
		if _, gErr := s.adv.Grader.Grade(ctx, assignment.ID, userID, 0, accountID, ""); gErr != nil {
			return fail(200, msgRef, op, gErr.Error())
		}
		s.recordLTI11Result(ctx, assignment.ID, userID, nil)
		return LTI11OutcomeResponse{Status: 200, XML: poxResponse(msgRef, op, "success", "Score deleted", "")}
	default: // readResult
		text := ""
		if score, ok := s.readLTI11Result(ctx, assignment.ID, userID); ok {
			text = strconv.FormatFloat(score, 'f', -1, 64)
		}
		body := `<readResultResponse><result><resultScore><language>en</language><textString>` + text + `</textString></resultScore></result></readResultResponse>`
		return LTI11OutcomeResponse{Status: 200, XML: poxResponse(msgRef, op, "success", "Result read", body)}
	}
}

// toolAccountID resolves the tenant of an install.
func (s *LTIService) toolAccountID(ctx context.Context, tool *models.ContextExternalTool) (uint, bool) {
	if tool.ContextType == "Account" {
		return tool.ContextID, tool.ContextID != 0
	}
	// AUTH-INTERNAL: resolving the install's own course to find its tenant.
	course, err := s.courseRepo.FindByID(ctx, tool.ContextID, 0)
	if err != nil || course == nil {
		return 0, false
	}
	return course.AccountID, true
}

// recordLTI11Result mirrors an outcome onto the assignment's default
// line item result, so readResult returns what was written.
func (s *LTIService) recordLTI11Result(ctx context.Context, assignmentID, userID uint, normalized *float64) {
	if s.adv.LineItemRepo == nil || s.adv.ResultRepo == nil {
		return
	}
	li, err := s.adv.LineItemRepo.FindByAssignmentID(ctx, assignmentID)
	if err != nil || li == nil {
		return
	}
	now := time.Now()
	r := &models.LTIResult{LineItemID: li.ID, UserID: userID, ActivityProgress: "Completed", GradingProgress: "FullyGraded", Timestamp: &now}
	if normalized != nil {
		score := *normalized * li.ScoreMaximum
		max := li.ScoreMaximum
		r.ResultScore, r.ResultMaximum = &score, &max
	} else {
		r.GradingProgress = "NotReady"
	}
	_ = s.adv.ResultRepo.Upsert(ctx, r)
}

func (s *LTIService) readLTI11Result(ctx context.Context, assignmentID, userID uint) (float64, bool) {
	if s.adv.LineItemRepo == nil || s.adv.ResultRepo == nil {
		return 0, false
	}
	li, err := s.adv.LineItemRepo.FindByAssignmentID(ctx, assignmentID)
	if err != nil || li == nil {
		return 0, false
	}
	res, err := s.adv.ResultRepo.ListByLineItem(ctx, li.ID, userID, repository.PaginationParams{Page: 1, PerPage: 1})
	if err != nil || res == nil || len(res.Items) == 0 {
		return 0, false
	}
	r := res.Items[0]
	if r.ResultScore == nil || r.ResultMaximum == nil || *r.ResultMaximum == 0 {
		return 0, false
	}
	return *r.ResultScore / *r.ResultMaximum, true
}
