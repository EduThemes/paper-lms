const COURSE_URL_PATTERN = /(?:api\/v1\/)?courses\/(\d+)/;

export function detectCrossCourseLinks(html, currentCourseId) {
  if (!html || !currentCourseId) return [];

  // Parse into an inert document. A <div> made with document.createElement
  // belongs to the live document, so assigning it unsaved editor HTML (or a
  // legacy stored body) loads its images and runs <img onerror> handlers on
  // every save, even though the element is never attached.
  const container = new DOMParser()
    .parseFromString(`<!DOCTYPE html><body>${html}</body>`, 'text/html').body;
  const issues = [];
  const currentId = String(currentCourseId);

  const elements = container.querySelectorAll('[href], [src]');
  elements.forEach((el) => {
    const attrs = ['href', 'src'];
    attrs.forEach((attr) => {
      const value = el.getAttribute(attr);
      if (!value) return;

      // Skip fully-qualified external URLs to avoid false positives
      if (/^https?:\/\//i.test(value)) return;
      // Skip data URIs, anchors, mailto, tel
      if (/^(data:|#|mailto:|tel:|javascript:)/i.test(value)) return;

      const match = value.match(COURSE_URL_PATTERN);
      if (match && match[1] !== currentId) {
        issues.push({
          element: el.tagName.toLowerCase(),
          attribute: attr,
          url: value,
          referencedCourseId: match[1],
          text: el.textContent?.trim().substring(0, 80) || el.getAttribute('alt') || value,
        });
      }
    });
  });

  return issues;
}
