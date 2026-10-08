// ePortfolio private sharing link helpers (G155).

/** The page URL of a privately shared portfolio. */
export function sharedPortfolioUrl(token, origin = '') {
  if (!token) return '';
  return `${origin}/portfolios/shared/${encodeURIComponent(token)}`;
}

/** Appends ?share_token= to an API path when viewing through a link. */
export function withShareToken(path, token) {
  if (!token) return path;
  const sep = path.includes('?') ? '&' : '?';
  return `${path}${sep}share_token=${encodeURIComponent(token)}`;
}
