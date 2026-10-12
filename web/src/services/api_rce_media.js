// API calls for rich-content media (G002): uploads and recordings from the
// editor, playback URLs and caption tracks. Backend:
// internal/api/v1/routes_rce_media.go.
import { request, getCSRFToken, ApiError } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

const enc = encodeURIComponent;

/** Playback URL of a media object (what <video src> / <audio src> load). */
export function mediaStreamUrl(mediaId) {
  return `${API_URL}/media_objects/${enc(mediaId)}/stream`;
}

/**
 * Upload (or send a recording of) an audio/video file. Uses XHR so the
 * caller can show progress for large files.
 *
 * @param {Blob|File} file
 * @param {{title?: string, filename?: string, courseId?: string|number, onProgress?: (pct: number) => void}} opts
 * @returns {Promise<object>} the created media object
 */
export function uploadMedia(file, { title, filename, courseId, onProgress } = {}) {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append('file', file, filename || file.name || 'media');
    if (title) form.append('title', title);
    if (courseId != null && courseId !== '') {
      form.append('context_type', 'Course');
      form.append('context_id', String(courseId));
    }
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${API_URL}/media_objects`);
    xhr.withCredentials = true;
    xhr.setRequestHeader('X-CSRF-Token', getCSRFToken());
    if (onProgress) {
      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
      });
    }
    xhr.onload = () => {
      let body = null;
      try { body = xhr.responseText ? JSON.parse(xhr.responseText) : null; } catch { body = null; }
      if (xhr.status >= 200 && xhr.status < 300) resolve(body);
      else reject(new ApiError({ status: xhr.status, body }));
    };
    xhr.onerror = () => reject(new ApiError({ status: 0, body: null, message: 'Upload failed — check your connection and try again.' }));
    xhr.send(form);
  });
}

export const rceMediaApi = {
  getMediaObject: async (mediaId) => (await request(`/media_objects/${enc(mediaId)}`)).data,
  listMyMedia: async (page = 1, perPage = 50) =>
    (await request(`/users/self/media_objects?page=${page}&per_page=${perPage}`)).data || [],
  renameMedia: async (mediaId, title) =>
    (await request(`/media_objects/${enc(mediaId)}`, { method: 'PUT', body: JSON.stringify({ title }) })).data,
  deleteMedia: async (mediaId) =>
    (await request(`/media_objects/${enc(mediaId)}`, { method: 'DELETE' })).data,
  listTracks: async (mediaId) => (await request(`/media_objects/${enc(mediaId)}/media_tracks`)).data || [],
  saveTrack: async (mediaId, { locale, kind, label, content }) =>
    (await request(`/media_objects/${enc(mediaId)}/media_tracks`, {
      method: 'POST',
      body: JSON.stringify({ locale, kind, label, content }),
    })).data,
  deleteTrack: async (mediaId, trackId) =>
    (await request(`/media_objects/${enc(mediaId)}/media_tracks/${enc(trackId)}`, { method: 'DELETE' })).data,
  uploadMedia,
  mediaStreamUrl,
};

export const rceMediaKeys = {
  all: ['rceMedia'],
  mine: () => ['rceMedia', 'mine'],
  detail: (mediaId) => ['rceMedia', 'detail', mediaId],
};
