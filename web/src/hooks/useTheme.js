import { useCallback, useEffect, useState } from 'react';

const STORAGE_KEY = 'paper.theme';
const MEDIA_QUERY = '(prefers-color-scheme: dark)';

const readStored = () => {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    return v === 'light' || v === 'dark' || v === 'system' ? v : 'light';
  } catch {
    return 'light';
  }
};

const systemPrefersDark = () =>
  typeof window !== 'undefined' && window.matchMedia(MEDIA_QUERY).matches;

export default function useTheme() {
  const [theme, setThemeState] = useState(readStored);
  const [systemDark, setSystemDark] = useState(systemPrefersDark);

  useEffect(() => {
    const mql = window.matchMedia(MEDIA_QUERY);
    const onChange = (e) => setSystemDark(e.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, []);

  const resolvedTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme;

  useEffect(() => {
    document.documentElement.classList.toggle('dark', resolvedTheme === 'dark');
  }, [resolvedTheme]);

  const setTheme = useCallback((next) => {
    try { localStorage.setItem(STORAGE_KEY, next); } catch { /* storage disabled */ }
    setThemeState(next);
  }, []);

  return { theme, setTheme, resolvedTheme };
}
