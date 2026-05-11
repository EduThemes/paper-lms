package service

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// tokenEmitterCtx holds the lookup tables that turn paper-LMS-internal URLs
// back into Canvas's portable placeholder tokens. Built once per export
// from the data the exporter has just written; passed to emitTokens for
// every HTML body about to land in the zip.
type tokenEmitterCtx struct {
	courseID uint
	// fileURLByID["/api/v1/files/<id>/download"] → "web_resources/<path>"
	// keyed by the public download URL the importer rewrote into the body.
	filePathByID map[uint]string
	// pageSlugBySlug echoes the slug back; included for symmetry with the
	// import side and to let the emitter validate that the slug is real
	// before injecting a $WIKI_REFERENCE$ token (otherwise we'd corrupt
	// links to unrelated /courses/N/pages/X URLs the author typed manually).
	pageSlugs map[string]bool
	// migrationIDByEntity[Type:ID] → stable migration_id used in the zip
	// (manifest resource identifier). Defaults to fmt.Sprintf("g%d", id)
	// when not pre-populated.
	migrationIDByEntity map[string]string
}

func (c *tokenEmitterCtx) migrationID(kind string, id uint) string {
	key := fmt.Sprintf("%s:%d", kind, id)
	if v, ok := c.migrationIDByEntity[key]; ok && v != "" {
		return v
	}
	return fmt.Sprintf("g%d", id)
}

// regexes are the inverse of the ones in imscc_token_rewriter.go:
//
//   /api/v1/files/<id>/download             → $IMS-CC-FILEBASE$/...
//   /courses/<courseID>/pages/<slug>        → $WIKI_REFERENCE$/wiki/<slug>
//   /courses/<courseID>/<type>/<id>         → $CANVAS_OBJECT_REFERENCE$/<type>/<migID>
var (
	reFileDownload = regexp.MustCompile(`/api/v1/files/(\d+)/download(?:/?[^"'<\s]*)?`)
	rePageURL      = regexp.MustCompile(`/courses/(\d+)/pages/([^"'<\s#?]+)`)
	reObjectURL    = regexp.MustCompile(`/courses/(\d+)/(assignments|quizzes|discussion_topics|announcements|modules)/(\d+)`)
)

// emitTokens replaces every internal URL it can resolve with the Canvas
// placeholder token that round-trips back through the importer. URLs we
// can't resolve (an unknown file id, a slug that isn't a wiki page in this
// course, a course id that doesn't match the export's course) are left
// untouched — better to ship a working absolute URL than a broken token.
func emitTokens(html string, ctx *tokenEmitterCtx) string {
	if html == "" || ctx == nil {
		return html
	}
	out := html

	// 1. Files. Match the longest URL form first so trailing path segments
	// (Canvas occasionally appends /verify or query strings) get included.
	out = reFileDownload.ReplaceAllStringFunc(out, func(match string) string {
		m := reFileDownload.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		var id uint
		_, _ = fmt.Sscanf(m[1], "%d", &id)
		zipPath, ok := ctx.filePathByID[id]
		if !ok {
			return match
		}
		// $IMS-CC-FILEBASE$ already implies the web_resources/ root, so
		// strip the prefix from the recorded zip path.
		rel := strings.TrimPrefix(zipPath, "web_resources/")
		return "$IMS-CC-FILEBASE$/" + url.PathEscape(rel)
	})

	// 2. Wiki pages.
	out = rePageURL.ReplaceAllStringFunc(out, func(match string) string {
		m := rePageURL.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		var cid uint
		_, _ = fmt.Sscanf(m[1], "%d", &cid)
		if cid != ctx.courseID {
			// Cross-course link — leave it alone; it isn't ours to relativize.
			return match
		}
		slug := m[2]
		if !ctx.pageSlugs[slug] {
			return match
		}
		return "$WIKI_REFERENCE$/wiki/" + slug
	})

	// 3. Other course objects.
	out = reObjectURL.ReplaceAllStringFunc(out, func(match string) string {
		m := reObjectURL.FindStringSubmatch(match)
		if len(m) < 4 {
			return match
		}
		var cid, oid uint
		_, _ = fmt.Sscanf(m[1], "%d", &cid)
		if cid != ctx.courseID {
			return match
		}
		objType := m[2]
		_, _ = fmt.Sscanf(m[3], "%d", &oid)
		canvasType := canvasObjectTypeName(objType)
		migID := ctx.migrationID(internalTypeName(objType), oid)
		return fmt.Sprintf("$CANVAS_OBJECT_REFERENCE$/%s/%s", canvasType, migID)
	})

	return out
}

// canvasObjectTypeName maps the URL segment to the Canvas-style type label
// expected after $CANVAS_OBJECT_REFERENCE$/.
func canvasObjectTypeName(urlSegment string) string {
	switch urlSegment {
	case "assignments":
		return "assignments"
	case "quizzes":
		return "quizzes"
	case "discussion_topics":
		return "discussion_topics"
	case "announcements":
		return "announcements"
	case "modules":
		return "modules"
	}
	return urlSegment
}

// internalTypeName converts the URL plural to the singular EntityRef.Type
// the exporter / importer use internally so migrationID lookups land in
// the same namespace as Wave 1's entityByMigID.
func internalTypeName(urlSegment string) string {
	switch urlSegment {
	case "assignments":
		return "Assignment"
	case "quizzes":
		return "Quiz"
	case "discussion_topics":
		return "DiscussionTopic"
	case "announcements":
		return "Announcement"
	case "modules":
		return "ContextModule"
	}
	return urlSegment
}
