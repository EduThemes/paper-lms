import React from 'react';

/**
 * Renders an LTI tool launch in an iframe. `src` is the Paper launch
 * endpoint (see ltiLaunchUrl); the browser follows it to the tool's OIDC
 * login, back through Paper's authorization endpoint and on to the tool.
 */
export default function LtiLaunchFrame({ src, title = 'External tool', height = 600, className = '' }) {
  if (!src) return null;
  return (
    <iframe
      src={src}
      title={title}
      className={`w-full rounded-md border border-border-default bg-surface-0 ${className}`}
      style={{ height }}
      allow="fullscreen; clipboard-write"
      allowFullScreen
    />
  );
}
