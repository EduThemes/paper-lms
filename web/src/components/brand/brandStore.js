// Tiny store for the signed-in account's theme logo (G118). The
// BrandThemeLoader writes it; BrandLogo reads it without needing a query
// client (it also renders on the sign-in pages).

let logoUrl = null;
const listeners = new Set();

export function setBrandLogoUrl(url) {
  if (url === logoUrl) return;
  logoUrl = url;
  listeners.forEach((l) => l());
}

export function getBrandLogoUrl() {
  return logoUrl;
}

export function subscribeBrandLogo(listener) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}
