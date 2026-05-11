import React, { createContext, useContext, useEffect, useMemo, useState, useCallback } from 'react';
import PropTypes from 'prop-types';

/**
 * ReadingPrefsContext
 * Persists per-user reading preferences (typography, spacing, background, TTS)
 * and applies them as CSS custom properties on <html>, so any element using
 * `prose` or `.reading-surface` inherits them globally.
 */

const STORAGE_KEY = 'paper.reading';

const FONT_STACKS = {
  system: 'ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif',
  lexend: '"Lexend", "Lexend Deca", ui-sans-serif, system-ui, sans-serif',
  atkinson: '"Atkinson Hyperlegible", "Atkinson Hyperlegible Next", ui-sans-serif, system-ui, sans-serif',
  opendyslexic: '"OpenDyslexic", "Comic Sans MS", ui-sans-serif, system-ui, sans-serif',
};

const BG_PALETTE = {
  white: { bg: '#FFFFFF', fg: '#111827' },
  cream: { bg: '#FAF6E9', fg: '#1F2937' },
  gray:  { bg: '#F3F4F6', fg: '#111827' },
  dark:  { bg: '#1E293B', fg: '#F8FAFC' },
};

export const DEFAULT_PREFS = Object.freeze({
  fontFamily: 'system',     // system | lexend | atkinson | opendyslexic
  fontScale: 1.0,           // 1.0 | 1.15 | 1.3 | 1.5
  lineHeight: 1.5,          // 1.5 | 1.75 | 2.0
  letterSpacing: 0,         // 0 | 0.02 | 0.05 (em)
  maxWidth: 'none',         // none | 75ch | 65ch | 55ch
  bg: 'white',              // white | cream | gray | dark
  noItalic: false,
  ttsEnabled: false,
});

function loadPrefs() {
  if (typeof window === 'undefined') return DEFAULT_PREFS;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_PREFS;
    return { ...DEFAULT_PREFS, ...JSON.parse(raw) };
  } catch {
    return DEFAULT_PREFS;
  }
}

function applyPrefs(prefs) {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  const palette = BG_PALETTE[prefs.bg] || BG_PALETTE.white;

  root.style.setProperty('--reading-font-family', FONT_STACKS[prefs.fontFamily] || FONT_STACKS.system);
  root.style.setProperty('--reading-font-scale', String(prefs.fontScale));
  root.style.setProperty('--reading-line-height', String(prefs.lineHeight));
  root.style.setProperty('--reading-letter-spacing', `${prefs.letterSpacing}em`);
  root.style.setProperty('--reading-max-width', prefs.maxWidth);
  root.style.setProperty('--reading-bg', palette.bg);
  root.style.setProperty('--reading-fg', palette.fg);
  root.style.setProperty('--reading-no-italic', prefs.noItalic ? '1' : '0');
  root.style.setProperty('--reading-tts-enabled', prefs.ttsEnabled ? '1' : '0');

  root.dataset.readingBg = prefs.bg;
  root.dataset.readingNoItalic = prefs.noItalic ? '1' : '0';
}

export const ReadingPrefsContext = createContext(null);

export function ReadingPrefsProvider({ children }) {
  const [prefs, setPrefsState] = useState(loadPrefs);

  useEffect(() => { applyPrefs(prefs); }, [prefs]);

  const setPrefs = useCallback((patch) => {
    setPrefsState((prev) => {
      const next = typeof patch === 'function' ? patch(prev) : { ...prev, ...patch };
      try { window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next)); } catch { /* ignore quota */ }
      return next;
    });
  }, []);

  const reset = useCallback(() => {
    try { window.localStorage.removeItem(STORAGE_KEY); } catch { /* ignore */ }
    setPrefsState(DEFAULT_PREFS);
  }, []);

  const value = useMemo(() => ({ prefs, setPrefs, reset }), [prefs, setPrefs, reset]);

  return <ReadingPrefsContext.Provider value={value}>{children}</ReadingPrefsContext.Provider>;
}

ReadingPrefsProvider.propTypes = { children: PropTypes.node };

export function useReadingPrefs() {
  const ctx = useContext(ReadingPrefsContext);
  if (!ctx) throw new Error('useReadingPrefs must be used within <ReadingPrefsProvider>');
  return ctx;
}

export { FONT_STACKS, BG_PALETTE };
