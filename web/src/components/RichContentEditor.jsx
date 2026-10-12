import React from 'react';
import RichContentEditorV2 from './rce/RichContentEditorV2';

/**
 * Legacy entry point, kept so older imports keep working (G017).
 *
 * Paper used to ship two rich content editors: this contentEditable one
 * (only on the assignment edit form) and the TipTap editor everywhere else,
 * with different features and different equation markup. Every rich-text
 * field now uses RichContentEditorV2; this module forwards to it. Equations
 * stored by the old editor (<span class="math-tex" data-latex>) are
 * converted on load (see rce/contentTransforms.js).
 *
 * New code should import ./rce/RichContentEditorV2 directly.
 *
 * @param {{value: string, onChange: (html: string) => void, placeholder?: string,
 *   courseId?: (string|number), autoSaveKey?: string, label?: string, className?: string}} props
 */
export default function RichContentEditor({ value, onChange, placeholder, courseId, autoSaveKey, label, className }) {
  return (
    <RichContentEditorV2
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      courseId={courseId}
      autoSaveKey={autoSaveKey}
      label={label}
      className={className}
    />
  );
}
