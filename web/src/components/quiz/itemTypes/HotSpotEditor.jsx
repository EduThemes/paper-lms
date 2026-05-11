import React, { useRef, useState } from 'react';
import { Image as ImageIcon, Trash2, Plus, Upload } from 'lucide-react';
import { api } from '../../../services/api';
import { makeId } from './types';

/**
 * Hot-spot editor. Author uploads an image, then click-and-drags to define
 * one or more accepted rectangular regions. Region coordinates are stored
 * as fractions of the image's natural dimensions so they survive resizing.
 *
 * Stored in single answer entry:
 *   { image_url, regions: [{id, x, y, w, h}] }   // all values 0..1
 */
const HotSpotEditor = ({ answers, onChange, courseId }) => {
  const cfg = (Array.isArray(answers) && answers[0]) || {
    id: makeId('a'), text: '', weight: 100, image_url: '', regions: [],
  };
  const imgRef = useRef(null);
  const [draft, setDraft] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [uploadErr, setUploadErr] = useState(null);

  const patch = (p) => onChange([{ ...cfg, ...p }]);

  const handleUpload = async (file) => {
    if (!file) return;
    setUploading(true);
    setUploadErr(null);
    try {
      // Try the typical course-files upload endpoint; this matches the existing
      // ContentPicker pattern.
      const formData = new FormData();
      formData.append('file', file);
      formData.append('name', file.name);
      // Falls back to a base64 data URL when the API isn't reachable in dev.
      let url = '';
      if (api.uploadCourseFile) {
        try {
          const result = await api.uploadCourseFile(courseId, formData);
          url = result?.url || result?.public_url || '';
        } catch {
          // ignore; fall through to base64
        }
      }
      if (!url) {
        url = await new Promise((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(reader.result);
          reader.onerror = reject;
          reader.readAsDataURL(file);
        });
      }
      patch({ image_url: url });
    } catch (err) {
      setUploadErr(err.message || 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const getRelative = (clientX, clientY) => {
    const img = imgRef.current;
    if (!img) return null;
    const rect = img.getBoundingClientRect();
    return {
      x: Math.max(0, Math.min(1, (clientX - rect.left) / rect.width)),
      y: Math.max(0, Math.min(1, (clientY - rect.top) / rect.height)),
    };
  };

  const handleMouseDown = (e) => {
    if (!cfg.image_url) return;
    const pos = getRelative(e.clientX, e.clientY);
    if (!pos) return;
    setDraft({ startX: pos.x, startY: pos.y, x: pos.x, y: pos.y, w: 0, h: 0 });
  };
  const handleMouseMove = (e) => {
    if (!draft) return;
    const pos = getRelative(e.clientX, e.clientY);
    if (!pos) return;
    const x = Math.min(draft.startX, pos.x);
    const y = Math.min(draft.startY, pos.y);
    const w = Math.abs(pos.x - draft.startX);
    const h = Math.abs(pos.y - draft.startY);
    setDraft(d => ({ ...d, x, y, w, h }));
  };
  const handleMouseUp = () => {
    if (!draft) return;
    if (draft.w > 0.01 && draft.h > 0.01) {
      patch({
        regions: [...(cfg.regions || []), {
          id: makeId('r'), x: draft.x, y: draft.y, w: draft.w, h: draft.h,
        }],
      });
    }
    setDraft(null);
  };

  const removeRegion = (id) => {
    patch({ regions: (cfg.regions || []).filter(r => r.id !== id) });
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <label className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium bg-brand-600 text-white rounded hover:bg-brand-700 cursor-pointer">
          <Upload className="w-3.5 h-3.5" />
          {cfg.image_url ? 'Replace image' : 'Upload image'}
          <input
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => handleUpload(e.target.files?.[0])}
            disabled={uploading}
          />
        </label>
        {uploading && <span className="text-xs text-text-tertiary">Uploading…</span>}
        {uploadErr && <span className="text-xs text-accent-danger">{uploadErr}</span>}
      </div>

      {cfg.image_url ? (
        <div
          className="relative inline-block border border-border-strong rounded overflow-hidden select-none bg-surface-1"
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          onMouseLeave={handleMouseUp}
        >
          <img
            ref={imgRef}
            src={cfg.image_url}
            alt="Hot-spot canvas"
            className="block max-w-full max-h-[400px] pointer-events-none"
            draggable={false}
          />
          {(cfg.regions || []).map((r, i) => (
            <div
              key={r.id}
              className="absolute border-2 border-accent-success bg-accent-success/20 group"
              style={{
                left: `${r.x * 100}%`,
                top: `${r.y * 100}%`,
                width: `${r.w * 100}%`,
                height: `${r.h * 100}%`,
              }}
            >
              <span className="absolute -top-5 -left-0.5 text-[10px] font-mono bg-accent-success text-white px-1 rounded">
                {i + 1}
              </span>
              <button
                onMouseDown={(e) => e.stopPropagation()}
                onClick={() => removeRegion(r.id)}
                type="button"
                className="absolute -top-2 -right-2 bg-accent-danger text-white rounded-full w-5 h-5 flex items-center justify-center opacity-0 group-hover:opacity-100"
                aria-label={`Remove region ${i + 1}`}
              >
                <Trash2 className="w-3 h-3" />
              </button>
            </div>
          ))}
          {draft && (
            <div
              className="absolute border-2 border-dashed border-brand-500 bg-brand-500/10 pointer-events-none"
              style={{
                left: `${draft.x * 100}%`,
                top: `${draft.y * 100}%`,
                width: `${draft.w * 100}%`,
                height: `${draft.h * 100}%`,
              }}
            />
          )}
        </div>
      ) : (
        <div className="border border-dashed border-border-strong rounded p-6 text-center bg-surface-1">
          <ImageIcon className="w-8 h-8 mx-auto text-text-disabled mb-2" />
          <p className="text-xs text-text-tertiary">Upload an image to begin defining hot-spot regions.</p>
        </div>
      )}

      {(cfg.regions || []).length > 0 && (
        <p className="text-xs text-text-tertiary">
          {(cfg.regions || []).length} region{(cfg.regions || []).length !== 1 ? 's' : ''} defined.
          Students must click inside any one region to score full credit.
        </p>
      )}
      {cfg.image_url && (cfg.regions || []).length === 0 && (
        <p className="text-xs text-accent-warning bg-accent-warning/10 border border-accent-warning/30 rounded p-2 inline-flex items-center gap-1">
          <Plus className="w-3.5 h-3.5" /> Click and drag on the image to define your first region.
        </p>
      )}
    </div>
  );
};

export default HotSpotEditor;
