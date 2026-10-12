// Pure helpers for the editor's media features (G002): node attributes for
// an uploaded media object, recorder format choice, caption languages.

/** Longest browser recording, in seconds (keeps uploads bounded). */
export const MAX_RECORDING_SECONDS = 10 * 60;

const RECORDER_TYPES = {
  video: ['video/webm;codecs=vp9,opus', 'video/webm;codecs=vp8,opus', 'video/webm', 'video/mp4'],
  audio: ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4', 'audio/ogg;codecs=opus'],
};

/**
 * The first container/codec the browser's MediaRecorder supports for kind
 * ('video' | 'audio'), or '' to let the browser choose.
 */
export function pickRecorderMimeType(kind, isTypeSupported) {
  const check = isTypeSupported
    || ((t) => typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported?.(t));
  return (RECORDER_TYPES[kind] || []).find((t) => {
    try { return check(t); } catch { return false; }
  }) || '';
}

/** A filename whose extension matches a recording's type. */
export function recordingFilename(mimeType, kind) {
  const base = (mimeType || '').split(';')[0].trim().toLowerCase();
  const ext = {
    'video/webm': 'webm', 'audio/webm': 'webm', 'video/mp4': 'mp4', 'audio/mp4': 'm4a',
    'audio/ogg': 'ogg', 'video/ogg': 'ogv', 'video/quicktime': 'mov',
  }[base] || (kind === 'audio' ? 'webm' : 'webm');
  const stamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
  return `${kind === 'audio' ? 'audio' : 'video'}-recording-${stamp}.${ext}`;
}

/** Editor `media` node attributes for a media object from the API. */
export function mediaNodeAttrs(mediaObject) {
  if (!mediaObject?.media_id) return null;
  return {
    src: mediaObject.stream_url || `/api/v1/media_objects/${mediaObject.media_id}/stream`,
    mediaId: mediaObject.media_id,
    mediaType: mediaObject.media_type === 'audio' ? 'audio' : 'video',
    title: mediaObject.title || '',
  };
}

/** Is this File/Blob something the media upload accepts? */
export function isMediaFile(file) {
  const type = (file?.type || '').toLowerCase();
  if (type.startsWith('audio/') || type.startsWith('video/')) return true;
  return /\.(mp4|m4v|m4a|mov|webm|ogg|ogv|oga|mp3|wav|aac)$/i.test(file?.name || '');
}

/** mm:ss for a recording timer. */
export function formatDuration(totalSeconds) {
  const s = Math.max(0, Math.floor(totalSeconds || 0));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

/** Caption languages offered in the captions dialog (any BCP 47 tag works). */
export const CAPTION_LANGUAGES = [
  { code: 'en', label: 'English' },
  { code: 'es', label: 'Español' },
  { code: 'fr', label: 'Français' },
  { code: 'de', label: 'Deutsch' },
  { code: 'pt', label: 'Português' },
  { code: 'it', label: 'Italiano' },
  { code: 'zh', label: '中文' },
  { code: 'ja', label: '日本語' },
  { code: 'ko', label: '한국어' },
  { code: 'ar', label: 'العربية' },
  { code: 'ru', label: 'Русский' },
  { code: 'vi', label: 'Tiếng Việt' },
  { code: 'hi', label: 'हिन्दी' },
  { code: 'ht', label: 'Kreyòl ayisyen' },
];

/** Default label for a caption language code. */
export function captionLabel(code) {
  const hit = CAPTION_LANGUAGES.find((l) => l.code.toLowerCase() === String(code || '').toLowerCase());
  return hit ? hit.label : String(code || '');
}
