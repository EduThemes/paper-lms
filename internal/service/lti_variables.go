package service

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// LTIVariableContext is everything a launch knows when it expands the
// substitution variables of custom parameters (G125). Nil members simply
// leave their variables unexpanded.
type LTIVariableContext struct {
	Issuer       string
	AccountID    uint
	User         *models.User
	Course       *models.Course
	Assignment   *models.Assignment
	ResourceLink *models.LTIResourceLink
	// EnrollmentType is the caller's course enrollment type ("" if none).
	EnrollmentType string
	Roles          []string
	PrivacyLevel   string
	Locale         string
}

// ltiVariable resolves one substitution variable. ok=false leaves the
// "$Name" literal in place (Canvas behavior for unknown or unavailable
// variables).
type ltiVariable func(v *LTIVariableContext) (string, bool)

func uintVar(n uint) (string, bool) {
	if n == 0 {
		return "", false
	}
	return strconv.FormatUint(uint64(n), 10), true
}

func timeVar(t *time.Time) (string, bool) {
	if t == nil {
		return "", false
	}
	return t.UTC().Format(time.RFC3339), true
}

func userVar(f func(u *models.User) (string, bool)) ltiVariable {
	return func(v *LTIVariableContext) (string, bool) {
		if v.User == nil {
			return "", false
		}
		return f(v.User)
	}
}

// nameVar / emailVar / sisVar gate PII variables on the tool's privacy
// level, so a custom parameter cannot leak what the claims withhold.
func nameVar(f func(u *models.User) string) ltiVariable {
	return func(v *LTIVariableContext) (string, bool) {
		if v.User == nil || !privacyAllowsName(v.PrivacyLevel) {
			return "", false
		}
		return f(v.User), true
	}
}

func emailVar(v *LTIVariableContext) (string, bool) {
	if v.User == nil || !privacyAllowsEmail(v.PrivacyLevel) || v.User.Email == "" {
		return "", false
	}
	return v.User.Email, true
}

func courseVar(f func(c *models.Course) (string, bool)) ltiVariable {
	return func(v *LTIVariableContext) (string, bool) {
		if v.Course == nil {
			return "", false
		}
		return f(v.Course)
	}
}

func assignmentVar(f func(a *models.Assignment) (string, bool)) ltiVariable {
	return func(v *LTIVariableContext) (string, bool) {
		if v.Assignment == nil {
			return "", false
		}
		return f(v.Assignment)
	}
}

func strPtrVar(p *string) (string, bool) {
	if p == nil || *p == "" {
		return "", false
	}
	return *p, true
}

func givenName(full string) string  { return extractGivenName(strings.TrimSpace(full)) }
func familyName(full string) string { return extractFamilyName(strings.TrimSpace(full)) }

// ltiVariables is the supported subset of Canvas's substitution
// variables (lib/lti/variable_expander.rb semantics, original code).
var ltiVariables = map[string]ltiVariable{
	"$Canvas.api.domain": func(v *LTIVariableContext) (string, bool) {
		u, err := url.Parse(v.Issuer)
		if err != nil || u.Host == "" {
			return "", false
		}
		return u.Host, true
	},
	"$Canvas.api.baseUrl":     func(v *LTIVariableContext) (string, bool) { return v.Issuer, v.Issuer != "" },
	"$Canvas.root_account.id": func(v *LTIVariableContext) (string, bool) { return uintVar(v.AccountID) },
	"$Canvas.account.id":      func(v *LTIVariableContext) (string, bool) { return uintVar(v.AccountID) },

	"$User.id":        userVar(func(u *models.User) (string, bool) { return uintVar(u.ID) }),
	"$Canvas.user.id": userVar(func(u *models.User) (string, bool) { return uintVar(u.ID) }),
	"$Canvas.user.loginId": func(v *LTIVariableContext) (string, bool) {
		if v.User == nil || !privacyAllowsSIS(v.PrivacyLevel) || v.User.LoginID == "" {
			return "", false
		}
		return v.User.LoginID, true
	},
	"$Canvas.user.sisSourceId": func(v *LTIVariableContext) (string, bool) {
		if v.User == nil || !privacyAllowsSIS(v.PrivacyLevel) {
			return "", false
		}
		return strPtrVar(v.User.SISUserID)
	},
	"$Person.sourcedId": func(v *LTIVariableContext) (string, bool) {
		if v.User == nil || !privacyAllowsSIS(v.PrivacyLevel) {
			return "", false
		}
		return strPtrVar(v.User.SISUserID)
	},
	"$Person.name.full":     nameVar(func(u *models.User) string { return u.Name }),
	"$Person.name.display":  nameVar(func(u *models.User) string { return u.Name }),
	"$Person.name.given":    nameVar(func(u *models.User) string { return givenName(u.Name) }),
	"$Person.name.family":   nameVar(func(u *models.User) string { return familyName(u.Name) }),
	"$Person.email.primary": emailVar,
	"$Canvas.user.locale": func(v *LTIVariableContext) (string, bool) {
		return v.Locale, v.Locale != ""
	},
	"$Message.locale": func(v *LTIVariableContext) (string, bool) {
		return v.Locale, v.Locale != ""
	},

	"$Canvas.course.id":             courseVar(func(c *models.Course) (string, bool) { return uintVar(c.ID) }),
	"$Context.id":                   courseVar(func(c *models.Course) (string, bool) { return uintVar(c.ID) }),
	"$Canvas.course.name":           courseVar(func(c *models.Course) (string, bool) { return c.Name, c.Name != "" }),
	"$Context.title":                courseVar(func(c *models.Course) (string, bool) { return c.Name, c.Name != "" }),
	"$Context.label":                courseVar(func(c *models.Course) (string, bool) { return c.CourseCode, c.CourseCode != "" }),
	"$com.instructure.contextLabel": courseVar(func(c *models.Course) (string, bool) { return c.CourseCode, c.CourseCode != "" }),
	"$Canvas.course.sisSourceId":    courseVar(func(c *models.Course) (string, bool) { return strPtrVar(c.SISCourseID) }),
	"$CourseOffering.sourcedId":     courseVar(func(c *models.Course) (string, bool) { return strPtrVar(c.SISCourseID) }),
	"$Canvas.course.workflowState":  courseVar(func(c *models.Course) (string, bool) { return string(c.WorkflowState), c.WorkflowState != "" }),

	"$Canvas.assignment.id":               assignmentVar(func(a *models.Assignment) (string, bool) { return uintVar(a.ID) }),
	"$Canvas.assignment.title":            assignmentVar(func(a *models.Assignment) (string, bool) { return a.Name, true }),
	"$Canvas.assignment.dueAt.iso8601":    assignmentVar(func(a *models.Assignment) (string, bool) { return timeVar(a.DueAt) }),
	"$Canvas.assignment.unlockAt.iso8601": assignmentVar(func(a *models.Assignment) (string, bool) { return timeVar(a.UnlockAt) }),
	"$Canvas.assignment.lockAt.iso8601":   assignmentVar(func(a *models.Assignment) (string, bool) { return timeVar(a.LockAt) }),
	"$Canvas.assignment.published":        assignmentVar(func(a *models.Assignment) (string, bool) { return strconv.FormatBool(a.IsPublished()), true }),
	"$Canvas.assignment.pointsPossible": assignmentVar(func(a *models.Assignment) (string, bool) {
		if a.PointsPossible == nil {
			return "", false
		}
		return strconv.FormatFloat(*a.PointsPossible, 'f', -1, 64), true
	}),

	"$ResourceLink.id": func(v *LTIVariableContext) (string, bool) {
		if v.ResourceLink == nil {
			return "", false
		}
		return v.ResourceLink.ResourceLinkID, true
	},
	"$ResourceLink.title": func(v *LTIVariableContext) (string, bool) {
		if v.ResourceLink == nil || v.ResourceLink.Title == "" {
			return "", false
		}
		return v.ResourceLink.Title, true
	},
	"$Canvas.membership.roles": func(v *LTIVariableContext) (string, bool) {
		return v.EnrollmentType, v.EnrollmentType != ""
	},
	"$Membership.role": func(v *LTIVariableContext) (string, bool) {
		return strings.Join(v.Roles, ","), len(v.Roles) > 0
	},
}

// SupportedLTIVariables lists the variable names Paper expands (sorted),
// advertised in the dynamic registration platform configuration.
func SupportedLTIVariables() []string {
	out := make([]string, 0, len(ltiVariables))
	for k := range ltiVariables {
		out = append(out, strings.TrimPrefix(k, "$"))
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ExpandLTIVariables replaces every value that is exactly a supported
// "$Variable" name with its value. Other values pass through unchanged.
func ExpandLTIVariables(custom map[string]string, v *LTIVariableContext) map[string]string {
	out := make(map[string]string, len(custom))
	for k, val := range custom {
		out[k] = val
		trimmed := strings.TrimSpace(val)
		if !strings.HasPrefix(trimmed, "$") {
			continue
		}
		if fn, ok := ltiVariables[trimmed]; ok && v != nil {
			if expanded, ok := fn(v); ok {
				out[k] = expanded
			}
		}
	}
	return out
}
