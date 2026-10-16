/**
 * Which country's school calendar "Coming up" follows. Some observances move between countries (Black
 * History Month is February in the US, October in the UK). Guessed from the user's Paper language, then the
 * browser's languages; the teacher can change it, and the choice is remembered in this browser.
 */
import { readJson, writeJson } from './storage';

export const REGIONS = ['US', 'CA', 'GB', 'IE', 'AU', 'NZ'];

const KEY = 'paper-designer:region';
const isRegion = (v) => REGIONS.includes(v);

/** The country part of a locale tag: "en-GB" → "GB", "en_au" → "AU", "en-UK" → "GB". */
function countryOf(tag) {
  const sub = String(tag ?? '').split(/[-_]/)[1]?.toUpperCase();
  const id = sub === 'UK' ? 'GB' : sub;
  return isRegion(id) ? id : null;
}

function browserLanguages() {
  try {
    return navigator.languages?.length ? navigator.languages : [navigator.language];
  } catch {
    return [];
  }
}

export function detectRegion(locale, languages = browserLanguages()) {
  for (const tag of [locale, ...languages]) {
    const r = countryOf(tag);
    if (r) return r;
  }
  return 'US';
}

/** The teacher's saved choice, else a guess. */
export function initialRegion(locale) {
  const saved = readJson(KEY);
  return isRegion(saved) ? saved : detectRegion(locale);
}

export const saveRegion = (r) => writeJson(KEY, r);
