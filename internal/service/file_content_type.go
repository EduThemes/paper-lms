package service

import "strings"

// OctetStream is the Content-Type a file is served with when the type it
// was uploaded with could run as script, or be applied as a stylesheet, in
// Paper's origin.
const OctetStream = "application/octet-stream"

// mimeBase returns the lower-cased media type without parameters.
func mimeBase(contentType string) string {
	base := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = strings.TrimSpace(base[:i])
	}
	return base
}

// IsScriptMIME reports whether a browser would execute a response of this
// type as a script (text/javascript, application/x-javascript,
// text/ecmascript, text/jscript, application/javascript;charset=..., and
// any other type naming javascript, ecmascript, jscript or livescript).
// Under "script-src 'self'" such a file, served from Paper's own origin,
// would run from a <script src="/api/v1/files/N/download">.
func IsScriptMIME(contentType string) bool {
	base := mimeBase(contentType)
	for _, w := range []string{"javascript", "ecmascript", "jscript", "livescript"} {
		if strings.Contains(base, w) {
			return true
		}
	}
	return false
}

// RecordedContentType is the type stored for a new upload: a script type
// becomes OctetStream (the client's Content-Type is attacker-controlled,
// and an unknown extension skips the MIME check), anything else is kept.
func RecordedContentType(contentType string) string {
	if IsScriptMIME(contentType) {
		return OctetStream
	}
	return contentType
}

// ServedContentType is the Content-Type a stored file is downloaded with:
// OctetStream for script types (rows stored before RecordedContentType
// existed included), for text/css (a same-origin stylesheet under
// "style-src 'self'") and for a missing type; the stored type otherwise.
//
// HTML, XHTML and SVG keep their type: the download handlers already send
// them as attachments (isInlineSafeMIME), with X-Content-Type-Options:
// nosniff, so they never render as documents, and an SVG must keep
// image/svg+xml to work in an <img>, where it cannot run script.
func ServedContentType(contentType string) string {
	base := mimeBase(contentType)
	if base == "" || base == "text/css" || IsScriptMIME(base) {
		return OctetStream
	}
	return contentType
}
