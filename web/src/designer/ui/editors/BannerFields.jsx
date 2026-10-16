/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/**
 * Banner choice: the theme's banner, its alternate, a holiday / special-day banner (the page keeps its
 * theme and colours), or the teacher's own image, uploaded into the course's Files or linked by address.
 */
import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { COLLECTIONS, THEMES, themeImageUrl, themeOf } from '../lib/engine';
import { SEASONAL_COLLECTIONS, collectionOf, seasonalPatch, seasonalThemeOf, seasonalThemes, switchBackPatch } from '../lib/catalog';
import { Checkbox, SelectField, TextField } from '../components/controls';
import { ImageUpload } from './ImageUpload';
import { isOwnImageSrc } from '../lib/starter';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';


function SeasonalPicker({ section }) {
  const { t } = useDesignerT();
  const { doc, state, applySeasonal, switchBack } = useDesigner();
  const list = seasonalThemes(THEMES);
  const active = seasonalThemeOf(section, THEMES);
  const prev = state.seasonalPrev[section.id];

  if (!list.length) return <p className="text-sm text-text-secondary">{t('banner.seasonalNone')}</p>;
  const groups = SEASONAL_COLLECTIONS.map((cid) => ({
    group: COLLECTIONS.find((c) => c.id === cid)?.name ?? cid,
    options: list.filter((x) => collectionOf(x) === cid).map((x) => ({ value: x.id, label: x.observance ? `${x.observance.name} — ${x.name}` : x.name })),
  })).filter((g) => g.options.length);

  return (
    <div className="space-y-2.5">
      <SelectField
        label={t('banner.seasonalPick')}
        value={active?.id ?? ''}
        placeholder={t('banner.seasonalPlaceholder')}
        onChange={(v) => {
          const th = list.find((x) => x.id === v);
          if (th) applySeasonal(section.id, seasonalPatch(th, (tt, img) => themeImageUrl(tt.id, img)));
        }}
        options={groups}
      />
      {active && (
        <>
          <p className="text-sm text-text-primary">{t('banner.seasonalActive', { name: active.observance?.name ?? active.name })}</p>
          <Button type="button" size="sm" variant="outline" onClick={() => switchBack(section.id, switchBackPatch(themeOf(doc), prev))}>
            {t('banner.switchBack')}
          </Button>
        </>
      )}
    </div>
  );
}

export function BannerFields({ section }) {
  const { t } = useDesignerT();
  const { doc, state, dispatch, switchBack } = useDesigner();
  const theme = themeOf(doc);
  const banners = theme.images.filter((i) => i.kind === 'banner');
  const main = banners.find((b) => b.id === 'banner') ?? banners[0];
  const alt = banners.find((b) => b.id !== main?.id);
  const isSeasonal = !!seasonalThemeOf(section, THEMES);
  const prev = state.seasonalPrev[section.id];
  const [pickingSeasonal, setPickingSeasonal] = useState(false);
  const source =
    isSeasonal || pickingSeasonal ? 'seasonal' : section.customSrc !== undefined ? 'own' : section.imageId && section.imageId === alt?.id ? 'alt' : 'theme';
  const [decorative, setDecorative] = useState(source === 'own' && section.alt === '' && !!section.customSrc);

  const up = (patch, field) => dispatch({ type: 'update', id: section.id, patch, coalesce: field });

  function choose(v) {
    setPickingSeasonal(v === 'seasonal');
    if (v === 'seasonal') return; // wait for a pick
    let patch;
    if (v === 'own') {
      setDecorative(false);
      patch = { customSrc: '', imageId: undefined, alt: '' };
    } else {
      const img = v === 'alt' ? alt : main;
      patch = { customSrc: undefined, imageId: img?.id, alt: img?.alt ?? '' };
    }
    // Leaving a seasonal banner also forgets what it replaced.
    if (isSeasonal || prev) switchBack(section.id, patch);
    else up(patch);
  }

  const url = section.customSrc ?? '';
  const badUrl = url.trim() !== '' && !isOwnImageSrc(url);
  const needsAlt = source === 'own' && !decorative && !section.alt.trim();
  const sources = [
    { value: 'theme', label: t('banner.theme'), disabled: !main },
    ...(alt ? [{ value: 'alt', label: t('banner.alt') }] : []),
    { value: 'seasonal', label: t('banner.seasonal') },
    { value: 'own', label: t('banner.own') },
  ];

  return (
    <div className="space-y-3">
      <fieldset className="space-y-1.5">
        <legend className="mb-1 text-sm font-medium text-text-primary">{t('banner.source')}</legend>
        {sources.map((o) => (
          <label key={o.value} className="flex items-center gap-2 text-sm text-text-primary">
            <input
              type="radio"
              name={`banner-src-${section.id}`}
              value={o.value}
              checked={source === o.value}
              disabled={o.disabled}
              onChange={() => choose(o.value)}
              className="h-4 w-4 border-border-strong text-brand-600 focus-visible:ring-2 focus-visible:ring-ring"
            />
            {o.label}
          </label>
        ))}
      </fieldset>

      {(source === 'theme' || source === 'alt') && (main || alt) && (
        <p className="text-xs text-text-secondary">{(source === 'alt' ? alt : main)?.alt || t('banner.decorativeTheme')}</p>
      )}

      {source === 'seasonal' && <SeasonalPicker section={section} />}

      {source === 'own' && (
        <div className="space-y-3">
          <ImageUpload label={t('banner.upload')} onUploaded={(src) => up({ customSrc: src })} />
          <p className="text-xs text-text-secondary">{t('banner.uploadHint')}</p>
          {isOwnImageSrc(url) && (
            <img src={url} alt={t('banner.previewAlt')} className="max-h-32 w-full rounded-md border border-border object-contain" />
          )}
          <TextField
            label={t('banner.url')}
            type="url"
            value={url}
            placeholder="https://"
            onChange={(v) => up({ customSrc: v }, 'customSrc')}
            error={badUrl ? t('banner.badUrl') : undefined}
            hint={badUrl ? undefined : t('banner.urlHint')}
          />
          <TextField
            label={t('banner.altText')}
            value={section.alt}
            disabled={decorative}
            required={!decorative}
            onChange={(v) => up({ alt: v }, 'alt')}
            error={needsAlt ? t('banner.altRequired') : undefined}
            hint={needsAlt ? undefined : t('banner.altHint')}
          />
          <Checkbox
            label={t('banner.decorative')}
            checked={decorative}
            onChange={(c) => {
              setDecorative(c);
              if (c) up({ alt: '' });
            }}
          />
        </div>
      )}
    </div>
  );
}
