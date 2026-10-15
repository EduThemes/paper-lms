/**
 * Theme art can be missing while it is still being made. In previews only, an image that fails to load is
 * replaced with a tinted block of the same shape, so the layout still reads. Saved HTML keeps the real URLs.
 */
const DEFAULT_RATIO = [1600, 500];

function placeholderSrc(w, h, tint) {
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">` +
    `<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${tint.fg}" stop-opacity=".28"/>` +
    `<stop offset="1" stop-color="${tint.fg}" stop-opacity=".08"/></linearGradient></defs>` +
    `<rect width="100%" height="100%" fill="${tint.bg}"/><rect width="100%" height="100%" fill="url(#g)"/></svg>`;
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

export function swapForPlaceholder(img, tint) {
  if (img.dataset.pdPlaceholder) return;
  const w = Number(img.getAttribute('width')) || DEFAULT_RATIO[0];
  const h = Number(img.getAttribute('height')) || (w === DEFAULT_RATIO[0] ? DEFAULT_RATIO[1] : w);
  img.dataset.pdPlaceholder = '1';
  img.removeAttribute('srcset');
  img.src = placeholderSrc(w, h, tint);
}

/** Watches every image under `root`; broken ones (now or later) become tinted placeholders. */
export function patchMissingImages(root, tint) {
  root.querySelectorAll('img').forEach((img) => {
    if (img.dataset.pdPlaceholder) return;
    if (img.complete && img.naturalWidth === 0 && img.getAttribute('src')) swapForPlaceholder(img, tint);
    else img.addEventListener('error', () => swapForPlaceholder(img, tint), { once: true });
  });
}
