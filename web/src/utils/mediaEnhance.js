// Caption tracks for embedded media (G002).
//
// Rich content stores a media embed as
//   <video controls src="/api/v1/media_objects/m-.../stream" data-media-id="m-...">
// without its <track> elements: captions are added (and fixed) after the
// content is written, so they are attached when the content is displayed.
// enhanceMediaInElement looks every such player up once and appends a
// <track> per caption language.
import { rceMediaApi } from '../services/api_rce_media';

export const MEDIA_STREAM_SRC_RE = /\/api\/v1\/media_objects\/(m-[0-9a-f]{8,64})\/stream/i;

export const TRACK_KINDS = ['subtitles', 'captions', 'descriptions', 'chapters', 'metadata'];

/** The media id of a <video>/<audio> element, or null. */
export function mediaIdOf(el) {
  if (!el?.getAttribute) return null;
  const attr = el.getAttribute('data-media-id');
  if (attr && /^m-[0-9a-f]{8,64}$/i.test(attr)) return attr;
  const src = el.getAttribute('src') || el.querySelector?.('source')?.getAttribute('src') || '';
  return src.match(MEDIA_STREAM_SRC_RE)?.[1] || null;
}

const trackCache = new Map();

/** Tracks of a media object (cached per page load). */
export function fetchMediaTracks(mediaId, loader = rceMediaApi.getMediaObject) {
  if (!trackCache.has(mediaId)) {
    const p = Promise.resolve()
      .then(() => loader(mediaId))
      .then((m) => (Array.isArray(m?.media_tracks) ? m.media_tracks : []))
      .catch(() => {
        trackCache.delete(mediaId); // retry on a later render
        return [];
      });
    trackCache.set(mediaId, p);
  }
  return trackCache.get(mediaId);
}

/** Forget cached tracks (after captions are edited). */
export function invalidateMediaTracks(mediaId) {
  if (mediaId) trackCache.delete(mediaId);
  else trackCache.clear();
}

/** Append <track> elements for tracks the element does not have yet. */
export function applyTracks(el, tracks) {
  if (!el || !Array.isArray(tracks)) return 0;
  let added = 0;
  tracks.forEach((t) => {
    if (!t?.url || !t.locale) return;
    const existing = Array.from(el.querySelectorAll('track'))
      .some((tr) => (tr.getAttribute('srclang') || '').toLowerCase() === String(t.locale).toLowerCase());
    if (existing) return;
    const track = document.createElement('track');
    track.setAttribute('kind', TRACK_KINDS.includes(t.kind) ? t.kind : 'subtitles');
    track.setAttribute('srclang', t.locale);
    track.setAttribute('label', t.label || t.locale);
    track.setAttribute('src', t.url);
    el.appendChild(track);
    added += 1;
  });
  return added;
}

/**
 * Attach caption tracks to every media-object player in container. Safe to
 * call repeatedly; each element is looked up once.
 */
export async function enhanceMediaInElement(container, loader) {
  if (!container?.querySelectorAll) return;
  const players = Array.from(container.querySelectorAll('video, audio'));
  await Promise.all(players.map(async (el) => {
    const mediaId = mediaIdOf(el);
    if (!mediaId || el.getAttribute('data-tracks-loaded') === mediaId) return;
    el.setAttribute('data-tracks-loaded', mediaId);
    if (!el.hasAttribute('controls')) el.setAttribute('controls', '');
    if (!el.hasAttribute('preload')) el.setAttribute('preload', 'metadata');
    const tracks = await fetchMediaTracks(mediaId, loader);
    applyTracks(el, tracks);
  }));
}
