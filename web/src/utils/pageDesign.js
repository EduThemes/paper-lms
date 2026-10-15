// Helpers for wiki pages made with the Page Designer, or carrying a page
// stylesheet (docs/page-designer/README.md, "Styling model" and "API").
//
// A page's show payload carries:
//   css         the stored (unscoped) page stylesheet, teachers edit this
//   scoped_css  the same CSS scoped to #paper-page-<id> for display
//   design      the PageDoc a designed page was made from, or null
// The list payload carries has_design instead.

// isDesignedPage: the page was made with the designer (its design is kept).
export const isDesignedPage = (page) => Boolean(page && page.design);

// hasPageStyling: the page needs the scoped renderer (designed, or has CSS).
export const hasPageStyling = (page) =>
  Boolean(page && (page.design || (typeof page.scoped_css === 'string' && page.scoped_css.trim())));

// pageScopeId is the id of the element the server scoped the page CSS to.
// Paper's page payload names the id page_id (Canvas shape); id is accepted
// for payloads that use it.
export const pageScopeId = (page) => `paper-page-${page?.page_id ?? page?.id ?? 'new'}`;

// Designer links (route owned by PageDesignerPage).
export const designerPath = (courseId, params) => {
  const qs = params ? new URLSearchParams(params).toString() : '';
  return `/courses/${courseId}/designer${qs ? `?${qs}` : ''}`;
};
export const makeoverPath = (courseId, params) => {
  const qs = params ? new URLSearchParams(params).toString() : '';
  return `/courses/${courseId}/designer/makeover${qs ? `?${qs}` : ''}`;
};
