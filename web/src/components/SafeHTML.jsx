import React, { useEffect, useMemo, useRef } from 'react';
import { sanitizeHTML } from './RichContentViewer';
import { enhanceUserContent } from '../utils/enhanceUserContent';
import { looksLikeHTML } from '../utils/html';

/**
 * Render stored rich content (question text, syllabus, stimulus, submission
 * HTML, ...) safely: DOMPurify-sanitized, with LaTeX (\( \), \[ \], $$ $$ and
 * .math-tex spans) rendered by KaTeX.
 *
 * Use this — never a bare dangerouslySetInnerHTML — wherever stored HTML is
 * shown. It is the lightweight counterpart of RichContentViewer (no prose
 * styling, read-aloud button or wrapper), so it fits inline spots such as a
 * question prompt or a table cell.
 */
export default function SafeHTML({ html, as: Tag = 'div', className, ...rest }) {
  const ref = useRef(null);
  const clean = useMemo(() => sanitizeHTML(html), [html]);

  useEffect(() => {
    if (ref.current && clean) enhanceUserContent(ref.current);
  }, [clean]);

  return <Tag ref={ref} className={className} dangerouslySetInnerHTML={{ __html: clean }} {...rest} />;
}

/**
 * Render a text-entry submission body. New bodies are HTML (the submission
 * form converts typed text to escaped paragraphs; API clients send HTML), but
 * older bodies were stored as raw typed text. Rendering those as HTML
 * collapsed line breaks and swallowed text such as "x<y and y>z" (B003), so
 * plain text is shown as text with its line breaks preserved.
 */
export function SubmissionBody({ body, className }) {
  if (!body) return null;
  if (looksLikeHTML(body)) {
    return <SafeHTML html={body} className={className} />;
  }
  return <div className={[className, 'whitespace-pre-wrap'].filter(Boolean).join(' ')}>{body}</div>;
}
