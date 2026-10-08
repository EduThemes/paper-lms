package htmlsanitize

import (
	"regexp"
	"strings"
)

// TrustedIframeHosts is the iframe embed allowlist for rich content (G001).
//
// Canvas lets an <iframe> point at any http(s) URL. Paper keeps an allowlist
// (an iframe from an arbitrary host is a convincing phishing surface inside a
// course page) but covers the hosts teachers actually embed from: video
// platforms, Google and Microsoft documents, lecture capture (Panopto,
// Kaltura, YuJa), math and science tools (Desmos, GeoGebra, PhET), and the
// common classroom tools (Padlet, Quizlet, Nearpod, Genially, Canva, ...).
//
// Each entry is a regular-expression fragment matched immediately after
// "https://", case-insensitively. It must be valid in both Go RE2 and
// JavaScript, and must end with a "/" right after the host (or a path under
// it) so "docs.google.com.attacker.example" can never match.
//
// The web client mirrors this list in web/src/utils/trustedIframeHosts.json;
// TestTrustedIframeHosts_MatchesWebClient fails when the two drift.
var TrustedIframeHosts = []string{
	// Video
	`(www\.)?youtube(-nocookie)?\.com/embed/`,
	`youtu\.be/`,
	`player\.vimeo\.com/video/`,
	`(www\.)?vimeo\.com/`,
	`(www\.)?loom\.com/embed/`,
	`fast\.wistia\.(net|com)/embed/`,
	`embed\.ted\.com/talks/`,
	`archive\.org/embed/`,
	`(www\.)?dailymotion\.com/embed/`,
	`go\.screenpal\.com/player/`,
	`screencast-o-matic\.com/(embed|player)`,
	// Lecture capture
	`([a-z0-9-]+\.)+panopto\.(com|eu)/panopto/pages/(embed|viewer)\.aspx`,
	`([a-z0-9-]+\.)*kaltura\.com/`,
	`([a-z0-9-]+\.)+yuja\.com/`,
	`web\.microsoftstream\.com/embed/`,
	// Interactive video / lessons
	`(www\.)?khanacademy\.org/embed`,
	`(www\.)?edpuzzle\.com/(embed|media)/`,
	`h5p\.org/h5p/embed/`,
	`([a-z0-9-]+\.)+h5p\.com/content/`,
	`(share|app)\.nearpod\.com/`,
	`(www\.)?pbslearningmedia\.org/`,
	// Documents
	`docs\.google\.com/(document|presentation|spreadsheets|forms|drawings)/`,
	`drive\.google\.com/file/d/`,
	`(www\.)?google\.com/maps/embed`,
	`calendar\.google\.com/calendar/embed`,
	`onedrive\.live\.com/embed`,
	`([a-z0-9-]+\.)+sharepoint\.com/`,
	`view\.officeapps\.live\.com/op/embed\.aspx`,
	`forms\.office\.com/`,
	// Math and science
	`(www|teacher|student)\.desmos\.com/`,
	`(www\.)?geogebra\.org/`,
	`phet\.colorado\.edu/sims/`,
	`(www\.)?ck12\.org/`,
	// Classroom tools
	`([a-z0-9-]+\.)?padlet\.(com|org)/`,
	`(www\.)?quizlet\.com/`,
	`view\.genial\.ly/`,
	`view\.genially\.com/`,
	`(www\.)?canva\.com/design/`,
	`(www\.)?prezi\.com/(p|v|embed)/`,
	`embed\.wakelet\.com/`,
	`(www\.)?thinglink\.com/(card|mediacard|view|video)/`,
	`read\.bookcreator\.com/`,
	`(www\.)?mentimeter\.com/`,
	`(www\.)?sketchfab\.com/models/`,
	`w\.soundcloud\.com/player/`,
	`open\.spotify\.com/embed`,
	`codepen\.io/[a-z0-9_-]+/embed/`,
}

// sameOriginIframePath is the one relative iframe src kept: a Canvas-style
// LTI launch (/courses/:id/external_tools/retrieve?url=...). Imported Canvas
// content embeds tools this way, and the path is rewritten to the importing
// course, so it stays same-origin.
const sameOriginIframePath = `/courses/[0-9]+/external_tools/retrieve\?`

// TrustedIframeSrc matches the src of every iframe the sanitizer keeps.
// Anything else loses its src, which leaves an inert, empty frame.
var TrustedIframeSrc = regexp.MustCompile(
	`(?i)^(https://(` + strings.Join(TrustedIframeHosts, "|") + `)|` + sameOriginIframePath + `)`,
)

// iframeAllowFeatures are the Permissions-Policy features an embed may ask
// for in its allow="..." attribute (YouTube's default embed code asks for
// most of them). Each may be followed by a 'self' / 'src' / 'none' / *
// allowlist; origins are not accepted.
const iframeAllowFeatures = `(accelerometer|autoplay|camera|clipboard-read|clipboard-write|display-capture|encrypted-media|fullscreen|gyroscope|microphone|midi|picture-in-picture|screen-wake-lock|web-share|xr-spatial-tracking)`

const iframeAllowEntry = iframeAllowFeatures + `( +('self'|'src'|'none'|\*))?`

// IframeAllowAttr validates an iframe's allow="..." value.
var IframeAllowAttr = regexp.MustCompile(
	`^ *` + iframeAllowEntry + `( *; *` + iframeAllowEntry + `)* *;? *$`,
)

// iframeReferrerPolicy validates referrerpolicy="..." (embeds such as
// YouTube send strict-origin-when-cross-origin).
var iframeReferrerPolicy = regexp.MustCompile(
	`^(no-referrer|no-referrer-when-downgrade|origin|origin-when-cross-origin|same-origin|strict-origin|strict-origin-when-cross-origin)$`,
)
