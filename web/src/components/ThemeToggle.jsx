import { Moon, Sun } from 'lucide-react';
import { useThemeContext } from '../contexts/ThemeContext';

/**
 * ThemeToggle — single-click sun/moon switch.
 * Sun visible in dark mode (click to go light), moon in light mode (click to go dark).
 * Holding Alt/Option resets to system preference.
 */
export default function ThemeToggle() {
  const { resolvedTheme, setTheme } = useThemeContext();
  const isDark = resolvedTheme === 'dark';
  const Icon = isDark ? Sun : Moon;

  const handleClick = (e) => {
    if (e.altKey) {
      setTheme('system');
      return;
    }
    setTheme(isDark ? 'light' : 'dark');
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      title={isDark ? 'Switch to light mode (Alt-click for system)' : 'Switch to dark mode (Alt-click for system)'}
      aria-label={isDark ? 'Switch to light mode' : 'Switch to dark mode'}
      className="relative group flex items-center justify-center w-10 h-10 rounded-md text-chrome-sidebar-fg/70 hover:bg-white/10 hover:text-chrome-sidebar-fg transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-400"
    >
      <Icon className="w-5 h-5" />
      <span className="absolute left-full ml-2 px-2 py-1 rounded bg-chrome-tooltip text-chrome-tooltip-fg text-xs font-medium whitespace-nowrap opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity z-50 hidden md:block">
        {isDark ? 'Light mode' : 'Dark mode'}
      </span>
    </button>
  );
}
