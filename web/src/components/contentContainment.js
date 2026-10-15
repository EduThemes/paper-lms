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

// Presentation-only companion class (index.css): lets wide tables and display
// equations scroll sideways instead of being clipped by paint containment.
export const CONTAINED_CLASS = 'paper-contained';
