import { useMemo } from 'react';
import { ASSET_BASE, checkAccessibility, renderDesign, renderOptions } from '../lib/engine';
import { pageCss } from '../lib/savePage';
import { useDesigner } from './context';

/** The open doc rendered for this course: { html, css } (custom CSS appended when the course allows it). */
export function useRendered() {
  const { doc, courseId, course } = useDesigner();
  return useMemo(() => {
    if (!doc) return { html: '', css: '' };
    const { html, css } = renderDesign(doc, renderOptions(courseId));
    return { html, css: pageCss(css, doc, !!course.customCssEnabled) };
  }, [doc, courseId, course.customCssEnabled]);
}

export function useIssues() {
  const { doc, courseId } = useDesigner();
  return useMemo(() => (doc ? checkAccessibility(doc, { courseId: courseId ? String(courseId) : undefined, assetBase: ASSET_BASE }) : []), [doc, courseId]);
}
