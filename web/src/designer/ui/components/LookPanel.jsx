/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
/** The Look tab: theme, colourway, custom colours, layout, heading style, spacing and the banner. */
import { LAYOUTS, THEMES, getTheme, themeOf } from '../lib/engine';
import { BannerFields } from '../editors/BannerFields';
import { CustomColours } from './CustomColours';
import { Segmented, SelectField } from './controls';
import { Swatches } from './Swatches';
import { useDesigner } from '../state/context';
import { useDesignerT } from '../i18n';

export function LookPanel() {
  const { t } = useDesignerT();
  const { doc, dispatch } = useDesigner();
  if (!doc) return null;
  const theme = themeOf(doc);
  const banner = doc.sections.find((s) => s.type === 'banner');
  const set = (patch) => dispatch({ type: 'setDoc', patch });

  /** Custom colours (doc.palette) are kept: they work with every theme. */
  function changeTheme(id) {
    const next = getTheme(id);
    if (!next) return;
    const paletteId = next.palettes.some((p) => p.id === doc.paletteId) ? doc.paletteId : next.palettes[0].id;
    set({ themeId: id, paletteId });
  }

  return (
    <div className="space-y-5">
      <h2 className="sr-only">{t('customize.look')}</h2>
      <SelectField label={t('look.theme')} value={doc.themeId} onChange={changeTheme} options={THEMES.map((th) => ({ value: th.id, label: th.name }))} />

      <Swatches
        name="pd-look-palette"
        legend={t('look.colourway')}
        palettes={theme.palettes}
        value={doc.palette ? '' : doc.paletteId}
        onChange={(paletteId) => set({ paletteId, palette: undefined })}
        large
        showLegend
      />

      <CustomColours />

      <div className="space-y-1.5">
        <SelectField label={t('look.layout')} value={doc.layout} onChange={(v) => set({ layout: v })} options={LAYOUTS.map((l) => ({ value: l.id, label: l.name }))} />
        <p className="text-xs text-text-secondary">{LAYOUTS.find((l) => l.id === doc.layout)?.description}</p>
      </div>

      <Segmented
        legend={t('look.headingStyle')}
        name="pd-heading-style"
        size="sm"
        value={doc.options.headingStyle}
        onChange={(v) => set({ options: { headingStyle: v } })}
        options={[
          { value: 'band', label: t('look.headingBand') },
          { value: 'underline', label: t('look.headingUnderline') },
          { value: 'plain', label: t('look.headingPlain') },
        ]}
      />

      <Segmented
        legend={t('look.density')}
        name="pd-density"
        size="sm"
        value={doc.options.density}
        onChange={(v) => set({ options: { density: v } })}
        options={[
          { value: 'comfortable', label: t('look.densityComfortable') },
          { value: 'compact', label: t('look.densityCompact') },
        ]}
      />

      <section aria-labelledby="pd-look-banner" className="space-y-2">
        <h3 id="pd-look-banner" className="text-sm font-semibold text-text-primary">
          {t('look.banner')}
        </h3>
        {banner ? <BannerFields section={banner} /> : <p className="text-sm text-text-secondary">{t('look.noBanner')}</p>}
      </section>
    </div>
  );
}
