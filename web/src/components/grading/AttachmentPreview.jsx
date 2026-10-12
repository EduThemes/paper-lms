import React from 'react';
import { FileText, Download } from 'lucide-react';

// Inline preview of a submission or comment file: images, PDFs, audio
// and video play in place (SpeedGrader's document preview); anything
// else is a download link. The URL is always the API's authorized
// download route, never a raw storage path.

export const previewKind = (file) => {
  const type = String(file?.content_type || '').toLowerCase();
  const name = String(file?.display_name || file?.filename || '').toLowerCase();
  if (type.startsWith('image/') || /\.(png|jpe?g|gif|webp|bmp)$/.test(name)) return 'image';
  if (type === 'application/pdf' || name.endsWith('.pdf')) return 'pdf';
  if (type.startsWith('audio/') || /\.(mp3|m4a|wav|oga|ogg)$/.test(name)) return 'audio';
  if (type.startsWith('video/') || /\.(mp4|webm|mov|m4v)$/.test(name)) return 'video';
  return 'other';
};

export const formatSize = (size) => {
  if (!size) return '';
  return size >= 1048576 ? `${(size / 1048576).toFixed(1)} MB` : `${Math.max(1, Math.round(size / 1024))} KB`;
};

// The download routes serve inline-safe types (images, PDF, audio,
// video) with Content-Disposition: inline, so the same URL previews.
const inlineUrl = (url) => url;

const AttachmentPreview = ({ file, height = 480, showLink = true }) => {
  if (!file?.url) return null;
  const kind = previewKind(file);
  const name = file.display_name || file.filename || 'File';
  return (
    <div className="space-y-1">
      {kind === 'image' && (
        <img src={inlineUrl(file.url)} alt={name} className="max-w-full rounded border border-border-default" />
      )}
      {kind === 'pdf' && (
        <iframe
          title={name}
          src={inlineUrl(file.url)}
          className="w-full rounded border border-border-default bg-surface-1"
          style={{ height }}
        />
      )}
      {kind === 'audio' && (
        <audio controls src={inlineUrl(file.url)} className="w-full" aria-label={name} />
      )}
      {kind === 'video' && (
        <video controls src={inlineUrl(file.url)} className="w-full max-h-96 rounded border border-border-default" aria-label={name} />
      )}
      {(showLink || kind === 'other') && (
        <a
          href={file.url}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 text-sm text-brand-600 hover:underline"
        >
          {kind === 'other' ? <FileText className="w-4 h-4" /> : <Download className="w-3.5 h-3.5" />}
          <span className="truncate">{name}</span>
          {file.size ? <span className="text-xs text-text-disabled">({formatSize(file.size)})</span> : null}
        </a>
      )}
    </div>
  );
};

export default AttachmentPreview;
