package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
)

// registerRCEMediaRoutes mounts rich-content media (G002): audio/video
// uploaded or recorded from the editor, playback, and caption tracks.
//
// None of these routes carry :course_id, so the handler authorizes: media
// is loaded under the caller's tenant (404 otherwise); playback needs only
// the unguessable media id; title and caption changes need the owner, a
// tenant admin or an instructor of the course the media was uploaded in;
// deleting needs the owner or a tenant admin.
func (r *Router) registerRCEMediaRoutes(protected fiber.Router) {
	if r.MediaObjectHandler == nil {
		return
	}
	h := r.MediaObjectHandler
	protected.Post("/media_objects", middleware.UploadRateLimit(), middleware.EnforceUploadSize(r.SettingsLookup), h.Upload)
	protected.Get("/users/self/media_objects", h.ListMine)
	protected.Get("/media_objects/:media_id", h.Get)
	protected.Put("/media_objects/:media_id", h.Update)
	protected.Delete("/media_objects/:media_id", h.Delete)
	protected.Get("/media_objects/:media_id/stream", h.Stream)
	protected.Get("/media_objects/:media_id/media_tracks", h.ListTracks)
	protected.Post("/media_objects/:media_id/media_tracks", h.SaveTrack)
	protected.Get("/media_objects/:media_id/media_tracks/:track_id/content", h.TrackContent)
	protected.Delete("/media_objects/:media_id/media_tracks/:track_id", h.DeleteTrack)
}
