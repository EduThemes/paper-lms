/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * The look gallery: collections, audience filter, search, layout and colours, "Coming up" observances for
 * the teacher's region, and a live thumbnail per theme. Used by the Choose step and the course makeover.
 * Every theme is free in Paper (no premium gating).
 */
import { useDeferredValue, useMemo, useState } from 'react';
import { Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import {
  COLLECTIONS,
  LAYOUTS,
  THEMES,
  renderDesign,
  renderOptions,
  tintOf,
} from '../lib/engine';
import { SEASONAL_COLLECTIONS, collectionOf, comingUp, filterThemes } from '../lib/catalog';
import { REGIONS, initialRegion, saveRegion } from '../lib/region';
import { KINDS, starterDoc } from '../lib/starter';
import { SelectField } from './controls';
import { Swatches } from './Swatches';
import { ThumbFrame } from './PreviewFrames';
import { useDesignerT } from '../i18n';

const AUDIENCES = ['all', 'early-elementary', 'upper-elementary', 'secondary', 'higher-ed', 'professional', 'specialty'];

const monthName = (m, lang) => {
  try {
    return new Date(2000, m - 1, 1).toLocaleString(lang, { month: 'long' });
  } catch {
    return String(m);
  }
};

function ThemeCard({ theme, kind, layout, palette, onUse, ctx }) {
  const { t, i18n } = useDesignerT();
  const [paletteId, setPaletteId] = useState(theme.palettes[0]?.id ?? '');
  const headingId = `pd-theme-${theme.id}`;
  const paletteKey = palette ? Object.values(palette).join(',') : '';
  const key = `${theme.id}|${paletteId}|${paletteKey}|${kind}|${layout}|${ctx.locale}|${ctx.courseId ?? ''}|${ctx.course?.name ?? ''}|${ctx.course?.code ?? ''}|${ctx.teacher?.name ?? ''}`;
  const render = () =>
    renderDesign(starterDoc({ theme, kind, locale: ctx.locale, layout, paletteId, palette, course: ctx.course, teacher: ctx.teacher }), renderOptions(ctx.courseId));
  const o = theme.observance;
  const tint = tintOf({ themeId: theme.id, paletteId, palette });

  return (
    <li aria-labelledby={headingId} className="flex flex-col overflow-hidden rounded-card border border-border bg-surface-0 shadow-xs">
      <div className="relative aspect-[4/3] w-full overflow-hidden border-b border-border">
        <ThumbFrame cacheKey={key} render={render} tint={tint} title={t('preview.thumbTitle', { name: theme.name })} lang={ctx.locale} />
        {/* The whole picture is a big click target; the iframe stays outside the button. */}
        <button
          type="button"
          tabIndex={-1}
          onClick={() => onUse(theme, paletteId)}
          className="absolute inset-0 h-full w-full cursor-pointer"
          aria-hidden="true"
        />
      </div>
      <div className="flex flex-1 flex-col gap-2 p-3">
        <h3 id={headingId} className="text-sm font-semibold text-text-primary">
          {theme.name}
        </h3>
        <p className="line-clamp-2 text-xs text-text-secondary">{theme.description}</p>
        {o && (
          <p className="text-xs text-text-primary">
            <strong>{o.name}</strong> · {o.months.map((m) => monthName(m, i18n.language)).join(', ')}
            {o.traditions.length > 0 && <> · {o.traditions.join(', ')}</>}
          </p>
        )}
        <p className="text-xs text-text-tertiary">{theme.audiences.map((a) => t(`audience.${a}`)).join(' · ')}</p>
        <div className="mt-auto flex items-center justify-between gap-2 pt-1">
          <Swatches
            name={`pal-${theme.id}`}
            legend={t('choose.colourways', { theme: theme.name })}
            palettes={palette ? [] : theme.palettes}
            value={paletteId}
            onChange={setPaletteId}
          />
          <Button size="sm" onClick={() => onUse(theme, paletteId)}>
            <span aria-hidden="true">{t('choose.use')}</span>
            <span className="sr-only">{t('choose.useNamed', { theme: theme.name })}</span>
          </Button>
        </div>
      </div>
    </li>
  );
}

function ComingUpRow({ items, onShow, region, onRegion }) {
  const { t } = useDesignerT();
  return (
    <section aria-labelledby="pd-comingup-h" className="rounded-card border border-border bg-surface-1 p-4">
      <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
        <h2 id="pd-comingup-h" className="text-base font-semibold text-text-primary">
          {t('choose.comingUp')}
        </h2>
        <SelectField
          className="w-48"
          label={t('choose.datesFor')}
          value={region}
          onChange={onRegion}
          options={REGIONS.map((r) => ({ value: r, label: t(`region.${r}`) }))}
        />
      </div>
      {!items.length && <p className="text-sm text-text-secondary">{t('choose.nothingComingUp')}</p>}
      <ul className="flex gap-3 overflow-x-auto pb-1" key={region}>
        {items.map((c) => (
          <li key={c.name} className="flex w-56 shrink-0 flex-col gap-1 rounded-md border border-border bg-surface-0 p-3">
            <span className="text-xs font-semibold uppercase tracking-wide text-text-tertiary">
              {c.thisMonth ? t('choose.thisMonth') : t('choose.nextMonth')}
            </span>
            <span className="font-semibold text-text-primary">{c.name}</span>
            {c.traditions.length > 0 && <span className="text-xs text-text-secondary">{c.traditions.join(', ')}</span>}
            {c.note && <p className="text-xs text-text-secondary">{c.note}</p>}
            <Button size="sm" variant="outline" className="mt-auto" onClick={() => onShow(c.name)}>
              {t('choose.showLooks', { n: c.themes.length })}
              <span className="sr-only"> {t('choose.forObservance', { name: c.name })}</span>
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}

function ComingUpStrip({ items, onShow }) {
  const { t } = useDesignerT();
  if (!items.length) return null;
  return (
    <nav aria-label={t('choose.comingUp')} className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
      <strong className="text-text-primary">{t('choose.comingUp')}:</strong>
      <ul className="flex flex-wrap gap-x-3 gap-y-1">
        {items.slice(0, 6).map((c) => (
          <li key={c.name}>
            <button type="button" className="text-brand-600 underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:text-brand-300" onClick={() => onShow(c)}>
              {c.name}
            </button>
          </li>
        ))}
      </ul>
    </nav>
  );
}

/**
 * props:
 *   ctx: { courseId, course, teacher, locale }
 *   kind, onKind (omit onKind to hide the page-type picker)
 *   defaultAudience: 'all' | Audience (from the course's grade band)
 *   colourOptions: [{ id, name, palette? }]  ('' = each theme's own)
 *   initialColours: id from colourOptions
 *   onUse(theme, paletteId, { layout, palette })
 *   heading, lede, top (node shown above the filters)
 */
export function ThemeGallery({ ctx, kind, onKind, defaultAudience = 'all', colourOptions = [], initialColours = '', onUse, heading, lede, top }) {
  const { t } = useDesignerT();
  const [shelf, setShelf] = useState('all');
  const [audience, setAudience] = useState(defaultAudience);
  const [layout, setLayout] = useState('');
  const [query, setQuery] = useState('');
  const [colours, setColours] = useState(initialColours);
  const deferredQuery = useDeferredValue(query);
  const [region, setRegion] = useState(() => initialRegion(ctx.locale));
  const month = new Date().getMonth() + 1;

  const shelfName = (s) => (s === 'all' ? t('choose.allLooks') : COLLECTIONS.find((c) => c.id === s)?.name ?? s);
  const shelfDesc = (s) => (s === 'all' ? t('choose.allLooksDesc') : COLLECTIONS.find((c) => c.id === s)?.description ?? '');
  const counts = useMemo(() => {
    const c = { all: THEMES.length };
    for (const th of THEMES) c[collectionOf(th)] = (c[collectionOf(th)] ?? 0) + 1;
    return c;
  }, []);
  const upcoming = useMemo(() => comingUp(THEMES, month, SEASONAL_COLLECTIONS, region), [month, region]);
  const upcomingHere = shelf === 'all' ? upcoming : upcoming.filter((c) => c.themes.some((th) => collectionOf(th) === shelf));
  const themes = useMemo(() => filterThemes(THEMES, { shelf, audience, query: deferredQuery }), [shelf, audience, deferredQuery]);
  const palette = colourOptions.find((o) => o.id === colours)?.palette;
  const shelves = ['all', ...COLLECTIONS.map((c) => c.id)];

  const pickShelf = (s) => {
    setShelf(s);
    setQuery('');
  };
  const pickRegion = (r) => {
    setRegion(r);
    saveRegion(r);
  };

  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <nav aria-label={t('choose.collections')} className="hidden w-52 shrink-0 lg:block">
        {onKind && (
          <>
            <h2 id="pd-kind-nav" className="mb-2 text-xs font-semibold uppercase tracking-wide text-text-tertiary">
              {t('choose.pageType')}
            </h2>
            <ul aria-labelledby="pd-kind-nav" className="mb-6 space-y-0.5">
              {KINDS.map((k) => (
                <li key={k}>
                  <button
                    type="button"
                    aria-current={k === kind ? 'page' : undefined}
                    onClick={() => onKind(k)}
                    className={cn(
                      'w-full rounded-md px-2.5 py-1.5 text-left text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                      k === kind ? 'bg-brand-50 font-semibold text-brand-700 dark:bg-brand-900/40 dark:text-brand-200' : 'text-text-secondary hover:bg-surface-1 hover:text-text-primary',
                    )}
                  >
                    {t(`kind.${k}`)}
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
        <h2 id="pd-shelf-nav" className="mb-2 text-xs font-semibold uppercase tracking-wide text-text-tertiary">
          {t('choose.collections')}
        </h2>
        <ul aria-labelledby="pd-shelf-nav" className="space-y-0.5">
          {shelves.map((s) => (
            <li key={s}>
              <button
                type="button"
                aria-current={s === shelf ? 'page' : undefined}
                onClick={() => pickShelf(s)}
                className={cn(
                  'flex w-full items-center justify-between rounded-md px-2.5 py-1.5 text-left text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                  s === shelf ? 'bg-brand-50 font-semibold text-brand-700 dark:bg-brand-900/40 dark:text-brand-200' : 'text-text-secondary hover:bg-surface-1 hover:text-text-primary',
                )}
              >
                <span>{shelfName(s)}</span>
                <span className="text-xs text-text-tertiary">{counts[s] ?? 0}</span>
              </button>
            </li>
          ))}
        </ul>
      </nav>

      <div className="min-w-0 flex-1 space-y-5">
        <div className="space-y-1">
          <h2 className="text-xl font-semibold text-text-primary">{heading ?? (shelf === 'all' ? t('choose.looks') : shelfName(shelf))}</h2>
          <p className="text-sm text-text-secondary">
            {lede} {shelfDesc(shelf)}
          </p>
        </div>
        {top}

        {shelf === 'all' && (
          <ComingUpStrip
            items={upcoming}
            onShow={(c) => {
              setShelf(collectionOf(c.themes[0]));
              setQuery(c.name);
            }}
          />
        )}
        {SEASONAL_COLLECTIONS.includes(shelf) && <ComingUpRow items={upcomingHere} onShow={setQuery} region={region} onRegion={pickRegion} />}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {onKind && (
            <SelectField
              className="lg:hidden"
              label={t('choose.pageType')}
              value={kind}
              onChange={onKind}
              options={KINDS.map((k) => ({ value: k, label: t(`kind.${k}`) }))}
            />
          )}
          <SelectField
            className="lg:hidden"
            label={t('choose.collections')}
            value={shelf}
            onChange={pickShelf}
            options={shelves.map((s) => ({ value: s, label: `${shelfName(s)} (${counts[s] ?? 0})` }))}
          />
          <div className="space-y-1.5 sm:col-span-2 xl:col-span-2">
            <label htmlFor="pd-search" className="text-sm font-medium text-text-primary">
              {t('choose.search')}
            </label>
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-text-tertiary" aria-hidden="true" />
              <Input
                id="pd-search"
                type="search"
                value={query}
                placeholder={t('choose.searchPlaceholder')}
                onChange={(e) => setQuery(e.target.value)}
                className="bg-surface-0 pl-9 text-text-primary"
              />
            </div>
          </div>
          <SelectField
            label={t('choose.layout')}
            value={layout}
            onChange={setLayout}
            options={[{ value: '', label: t('choose.layoutDefault') }, ...LAYOUTS.map((l) => ({ value: l.id, label: l.name }))]}
          />
          {colourOptions.length > 1 && (
            <SelectField
              label={t('choose.colours')}
              value={colourOptions.some((o) => o.id === colours) ? colours : ''}
              onChange={setColours}
              options={colourOptions.map((o) => ({ value: o.id, label: o.name }))}
            />
          )}
        </div>

        <fieldset>
          <legend className="mb-1.5 text-sm font-medium text-text-primary">{t('choose.filter')}</legend>
          <div className="flex flex-wrap gap-2">
            {AUDIENCES.map((a) => (
              <label key={a} className="cursor-pointer">
                <input type="radio" name="pd-audience" value={a} checked={audience === a} onChange={() => setAudience(a)} className="peer sr-only" />
                <span className="inline-flex rounded-full border border-border px-3 py-1 text-sm text-text-secondary transition-colors peer-checked:border-brand-600 peer-checked:bg-brand-600 peer-checked:text-white peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-2 hover:border-border-strong">
                  {t(`audience.${a}`)}
                </span>
              </label>
            ))}
          </div>
          {defaultAudience !== 'all' && <p className="mt-1 text-xs text-text-tertiary">{t('choose.filterFromCourse')}</p>}
        </fieldset>

        <p className="text-sm text-text-secondary" role="status" aria-live="polite">
          {t('choose.count', { count: themes.length })}
          {query && (
            <>
              {' '}
              <button type="button" className="text-brand-600 underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:text-brand-300" onClick={() => setQuery('')}>
                {t('choose.clearSearch')}
              </button>
            </>
          )}
        </p>

        {themes.length === 0 ? (
          <p className="text-sm text-text-primary">{(counts[shelf] ?? 0) === 0 ? t('choose.shelfEmpty') : t('choose.none')}</p>
        ) : (
          <ul aria-label={shelfName(shelf)} className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            {themes.map((th) => (
              <ThemeCard
                key={th.id}
                theme={th}
                kind={kind}
                layout={layout}
                palette={palette}
                ctx={ctx}
                onUse={(theme, paletteId) => onUse(theme, paletteId, { layout, palette })}
              />
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
