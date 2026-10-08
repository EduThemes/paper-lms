import React, { useEffect, useRef } from 'react';
import { X } from 'lucide-react';
import LtiLaunchFrame from './LtiLaunchFrame';
import { parseDeepLinkMessage } from '../../services/api_lti';

/**
 * Content selection dialog (LTI Deep Linking, G130). Launches the tool's
 * deep linking placement in an iframe and waits for Paper's deep linking
 * return page to post the result back (subject LtiDeepLinkingResponse).
 *
 * Props: open, title, src (ltiLaunchUrl with deep_link=…), width/height,
 * onResult({ items, result, error }), onClose.
 */
export default function LtiDeepLinkModal({ open, title = 'Select content', src, width, height, onResult, onClose }) {
  const onResultRef = useRef(onResult);
  onResultRef.current = onResult;

  useEffect(() => {
    if (!open) return undefined;
    const handler = (event) => {
      const parsed = parseDeepLinkMessage(event);
      if (parsed) onResultRef.current?.(parsed);
    };
    window.addEventListener('message', handler);
    return () => window.removeEventListener('message', handler);
  }, [open]);

  useEffect(() => {
    if (!open) return undefined;
    const onKey = (e) => {
      if (e.key === 'Escape') onClose?.();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open || !src) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true" aria-label={title}>
      <div
        className="flex max-h-full w-full flex-col rounded-lg bg-surface-0 shadow-xl"
        style={{ maxWidth: width ? Math.max(width + 48, 480) : 960 }}
      >
        <div className="flex items-center justify-between border-b border-border-default px-4 py-3">
          <h2 className="text-base font-semibold text-text-primary">{title}</h2>
          <button type="button" onClick={onClose} className="rounded p-1 text-text-tertiary hover:text-text-primary" aria-label="Close">
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="p-4">
          <LtiLaunchFrame src={src} title={title} height={height || 560} />
        </div>
      </div>
    </div>
  );
}
