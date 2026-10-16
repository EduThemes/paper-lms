// Containment for every surface that renders stored rich content
// (RichContentViewer, SafeHTML, designed pages).
//
// Rich content may use absolute positioning, z-index, transforms and
// backgrounds (docs/page-designer/README.md, "Styling model"), and a page
// stylesheet may even use position: fixed. The content root is therefore a
// containment box:
//
//   contain: layout paint  -> the root is the containing block for absolute
//                             AND fixed descendants, and nothing paints
//                             outside it;
//   isolation: isolate     -> its own stacking context, so a z-index inside
//                             can never rise above Paper's header, menus or
//                             dialogs;
//   position: relative     -> offsets resolve against the root in browsers
//                             that predate layout containment.
//
// Kept inline (not only in CSS) so the guarantee holds even if a stylesheet
// fails to load. Popovers and tooltips Paper renders through portals live
// outside the root and are unaffected.
export const CONTENT_CONTAINMENT = Object.freeze({
  contain: 'layout paint',
  isolation: 'isolate',
  position: 'relative',
});

// A style object as an inline `style` attribute string, for roots React does
// not render itself (the RCE's ProseMirror root, the designer preview shell).
export const styleText = (o) =>
  Object.entries(o)
    .map(([k, v]) => `${k.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}: ${v}`)
    .join('; ');

// Presentation-only companion class (index.css): lets wide tables and display
// equations scroll sideways instead of being clipped by paint containment.
export const CONTAINED_CLASS = 'paper-contained';

// Containment for the frame around a page with its own stylesheet
// (DesignedPageView / PageScope). The page CSS is scoped to the inner root
// (#paper-page-<id>) and its descendants, so it can restyle the root itself:
// `:root { padding-right: 1200px }` grows the root's own box (paint
// containment clips what is inside a box, not the box), and
// `.paper-contained { contain: none !important }` switches off an inner
// viewer's containment. The frame is the root's parent, which no scoped
// selector can match, so its containment always holds:
//
//   contain: layout paint + isolation + position  -> as CONTENT_CONTAINMENT;
//   overflow: clip, overflow-clip-margin: 0       -> whatever the root grows
//                                                    to is cut at the frame's
//                                                    edge, with no scroll
//                                                    container (and no
//                                                    scrollbar to style).
export const PAGE_FRAME_CONTAINMENT = Object.freeze({
  ...CONTENT_CONTAINMENT,
  overflow: 'clip',
  overflowClipMargin: '0px',
});

// Class and data attribute of that frame (for tests and devtools; nothing
// styles it, and page CSS cannot reach it).
export const PAGE_FRAME_CLASS = 'paper-page-frame';
