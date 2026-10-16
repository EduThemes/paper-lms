import { useContext, useEffect, useMemo, useRef } from 'react';
import PropTypes from 'prop-types';
import { sanitizeHTML, ReadAloudButton, htmlToPlainText } from '../RichContentViewer';
import { enhanceUserContent } from '../../utils/enhanceUserContent';
import { ReadingPrefsContext } from '../../contexts/ReadingPrefsContext';
import { CONTENT_CONTAINMENT, PAGE_FRAME_CLASS, PAGE_FRAME_CONTAINMENT } from '../contentContainment';
import { isDesignedPage, pageScopeId } from '../../utils/pageDesign';

// Rendering for wiki pages with a page stylesheet or a design
// (docs/page-designer/README.md, "Styling model").
//
// The server returns scoped_css already sanitized and scoped to
// #paper-page-<id> (csssanitize.Scope), with `</style` impossible in it. It
// goes into a <style> element as a React text child, which React writes as
// a text node (textContent), never parsed as HTML.
//
// Two boxes: an outer frame (PageFrame) and, inside it, the page root
// #paper-page-<id> the stylesheet is scoped to. Both are containment boxes,
// so nothing in the page, not even position: fixed from its stylesheet, can
// draw over Paper's interface. The root alone is not enough: the stylesheet
// can restyle the root itself (padding, min-height) and so grow its box past
// the page area, and it can switch off containment on elements inside it.
// The frame is the root's parent, out of reach of every scoped selector, and
// clips whatever the root grows to.

const pageShape = PropTypes.shape({
  page_id: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  id: PropTypes.oneOfType([PropTypes.number, PropTypes.string]),
  title: PropTypes.string,
  body: PropTypes.string,
  scoped_css: PropTypes.string,
  design: PropTypes.object,
});

// PageStylesheet emits the page's scoped stylesheet, or nothing.
export function PageStylesheet({ css }) {
  if (!css || !String(css).trim()) return null;
  return <style data-paper-page-css="">{String(css)}</style>;
}
PageStylesheet.propTypes = { css: PropTypes.string };

// PageFrame is the containment box around a page root. It carries no id or
// class a page stylesheet could name, and it is an ancestor of the root, so
// no scoped selector (all of which match the root or its descendants) can
// reach it.
export function PageFrame({ children }) {
  return (
    <div className={PAGE_FRAME_CLASS} data-paper-page-frame="" style={PAGE_FRAME_CONTAINMENT}>
      {children}
    </div>
  );
}
PageFrame.propTypes = { children: PropTypes.node };

// PageScope wraps an ordinary page's usual rendering (prose and all) in the
// scope root its stylesheet targets.
export function PageScope({ page, className, children }) {
  return (
    <>
      <PageStylesheet css={page?.scoped_css} />
      <PageFrame>
        <div
          id={pageScopeId(page)}
          className={['paper-page', className].filter(Boolean).join(' ')}
          style={CONTENT_CONTAINMENT}
        >
          {children}
        </div>
      </PageFrame>
    </>
  );
}
PageScope.propTypes = { page: pageShape, className: PropTypes.string, children: PropTypes.node };

// DesignedPageView renders a designed page: its stylesheet and its sanitized
// body, with none of the prose / reading-surface wrappers that restyle
// ordinary content (the design brings its own typography and colours).
export default function DesignedPageView({ page, className }) {
  const ref = useRef(null);
  const body = page?.body || '';
  const sanitized = useMemo(() => sanitizeHTML(body), [body]);
  const readingCtx = useContext(ReadingPrefsContext);
  const showTTS = readingCtx?.prefs?.ttsEnabled === true;
  const plainText = useMemo(() => (showTTS ? htmlToPlainText(sanitized) : ''), [showTTS, sanitized]);

  useEffect(() => {
    if (ref.current && sanitized) enhanceUserContent(ref.current);
  }, [sanitized]);

  return (
    <>
      {showTTS && plainText && (
        <div className="mb-2 flex justify-end">
          <ReadAloudButton text={plainText} />
        </div>
      )}
      <PageStylesheet css={page?.scoped_css} />
      <PageFrame>
        <div
          ref={ref}
          id={pageScopeId(page)}
          className={['paper-page', className].filter(Boolean).join(' ')}
          data-designed-page=""
          style={CONTENT_CONTAINMENT}
          dangerouslySetInnerHTML={{ __html: sanitized }}
        />
      </PageFrame>
    </>
  );
}
DesignedPageView.propTypes = { page: pageShape, className: PropTypes.string };

// PageContent picks the renderer for a page: designed pages render raw
// (DesignedPageView); pages with their own CSS keep `fallback` (the caller's
// usual prose rendering) inside the scope root; anything else is `fallback`.
export function PageContent({ page, fallback, className }) {
  if (isDesignedPage(page)) return <DesignedPageView page={page} className={className} />;
  if (page?.scoped_css && String(page.scoped_css).trim()) {
    return <PageScope page={page} className={className}>{fallback}</PageScope>;
  }
  return fallback ?? null;
}
PageContent.propTypes = { page: pageShape, fallback: PropTypes.node, className: PropTypes.string };
