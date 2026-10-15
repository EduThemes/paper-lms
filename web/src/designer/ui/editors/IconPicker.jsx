/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Icon picker: the theme's own icons and a short list first, then search across every Lucide icon the
 * engine knows. Icons are shown exactly as the page will show them (the icon route, in the page's style and
 * colours).
 */
import { useId, useMemo, useState } from 'react';
import { ChevronRight, Search } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';
import { iconNames, iconUrl, paletteOf, themeOf } from '../lib/engine';
import { ICON_CHOICES } from '../lib/sections';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

const MAX_RESULTS = 60;
let allIcons = null;
const everyIcon = () => (allIcons ??= iconNames());

export function IconPicker({ label, value, onChange }) {
  const { t } = useDesignerT();
  const { doc } = useDesigner();
  const id = useId();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const theme = themeOf(doc);
  const palette = paletteOf(doc);
  const src = (name) => iconUrl(name, { style: theme.iconStyle, fg: palette.onPrimary, bg: palette.primary });

  const choices = useMemo(() => {
    const q = query.trim().toLowerCase().replace(/\s+/g, '-');
    const first = [...new Set([...Object.values(theme.icons ?? {}), ...ICON_CHOICES, value].filter(Boolean))];
    if (!q) return first;
    const hits = [...first, ...everyIcon()].filter((n, i, arr) => n.includes(q) && arr.indexOf(n) === i);
    return hits.slice(0, MAX_RESULTS);
  }, [query, theme, value]);

  const pretty = (n) => n.replace(/-/g, ' ');

  return (
    <div className="space-y-1.5">
      <span id={`${id}-label`} className="text-sm font-medium text-text-primary">
        {label}
      </span>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            aria-labelledby={`${id}-label ${id}-value`}
            className="flex h-10 w-full items-center gap-2 rounded-md border border-input bg-surface-0 px-2 text-left text-sm text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {value && <img src={src(value)} alt="" width={24} height={24} className="h-6 w-6" />}
            <span id={`${id}-value`} className="flex-1 truncate">
              {pretty(value || '')}
            </span>
            <ChevronRight className="h-4 w-4 text-text-tertiary" aria-hidden="true" />
          </button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-80 space-y-2 p-3">
          <div className="relative">
            <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-text-tertiary" aria-hidden="true" />
            <Input
              aria-label={t('field.iconSearch')}
              placeholder={t('field.iconSearch')}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="h-9 bg-surface-0 pl-8 text-text-primary"
            />
          </div>
          {choices.length === 0 ? (
            <p className="text-sm text-text-secondary">{t('field.iconNone')}</p>
          ) : (
            <ul className="grid max-h-64 grid-cols-6 gap-1 overflow-y-auto" aria-label={label}>
              {choices.map((name) => (
                <li key={name}>
                  <button
                    type="button"
                    title={pretty(name)}
                    aria-pressed={name === value}
                    onClick={() => {
                      onChange(name);
                      setOpen(false);
                      setQuery('');
                    }}
                    className={cn(
                      'flex h-11 w-11 items-center justify-center rounded-md hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                      name === value && 'ring-2 ring-brand-600',
                    )}
                  >
                    <img src={src(name)} alt="" width={28} height={28} loading="lazy" className="h-7 w-7" />
                    <span className="sr-only">{pretty(name)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </PopoverContent>
      </Popover>
    </div>
  );
}
