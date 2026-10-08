package handlers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Canvas request-parameter conventions (G137). Canvas clients send list
// parameters as repeated "name[]=a&name[]=b" pairs; many also send
// "name=a,b" or a bare repeated "name=a&name=b". These helpers accept all
// three so a script written for Canvas works unchanged.

// queryList returns every value of the list parameter name, accepting
// name[]=a&name[]=b, name=a&name=b and name=a,b.
func queryList(c *fiber.Ctx, name string) []string {
	var out []string
	args := c.Context().QueryArgs()
	for _, key := range []string{name + "[]", name} {
		for _, raw := range args.PeekMulti(key) {
			for _, v := range strings.Split(string(raw), ",") {
				if v = strings.TrimSpace(v); v != "" {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// includeSet parses Canvas's include[] parameter into a set.
func includeSet(c *fiber.Ctx) map[string]bool {
	set := map[string]bool{}
	for _, v := range queryList(c, "include") {
		set[v] = true
	}
	return set
}

// queryUintList parses a list parameter of numeric ids, resolving "self"
// to the caller. ok=false means some value was not a valid id; the
// returned string names it.
func queryUintList(c *fiber.Ctx, name string) ([]uint, string, bool) {
	vals := queryList(c, name)
	out := make([]uint, 0, len(vals))
	for _, v := range vals {
		if v == "self" {
			if uid, ok := c.Locals("user_id").(uint); ok && uid > 0 {
				out = append(out, uid)
				continue
			}
			return nil, v, false
		}
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return nil, v, false
		}
		out = append(out, uint(n))
	}
	return out, "", true
}

// queryBool parses a Canvas boolean parameter ("true"/"1"/"false"/"0").
// nil means the parameter was absent or unrecognised.
func queryBool(c *fiber.Ctx, name string) *bool {
	switch strings.ToLower(strings.TrimSpace(c.Query(name))) {
	case "true", "1", "yes", "on":
		v := true
		return &v
	case "false", "0", "no", "off":
		v := false
		return &v
	}
	return nil
}

// queryTime parses an ISO 8601 timestamp parameter. ok=false means the
// value was present but malformed.
func queryTime(c *fiber.Ctx, name string) (*time.Time, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t, true
		}
	}
	return nil, false
}

// canvasEnrollmentTypes maps Canvas's short enrollment_type[] values
// ("student", "teacher", ...) and full type names to model types.
func canvasEnrollmentTypes(vals []string) ([]string, string, bool) {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		switch strings.ToLower(strings.TrimSuffix(v, "enrollment")) {
		case "student", "studentenrollment":
			out = append(out, "StudentEnrollment")
		case "teacher", "teacherenrollment":
			out = append(out, "TeacherEnrollment")
		case "ta", "taenrollment":
			out = append(out, "TaEnrollment")
		case "observer", "observerenrollment":
			out = append(out, "ObserverEnrollment")
		case "designer", "designerenrollment":
			out = append(out, "DesignerEnrollment")
		case "student_view":
			// Paper has no test-student enrollments; matches nothing.
			out = append(out, "StudentViewEnrollment")
		default:
			return nil, v, false
		}
	}
	return out, "", true
}
