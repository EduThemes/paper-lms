/* eslint-disable react/prop-types -- Paper JSX components don't declare propTypes */
import { cn } from '@/lib/utils';

/** Colourway picker: a radio group of round two-tone swatches with the colourway name as the label. */
export function Swatches({ name, legend, palettes, value, onChange, large = false, showLegend = false }) {
  if (palettes.length < 2 && !showLegend) return null;
  return (
    <fieldset className="flex min-w-0 flex-wrap items-center gap-1.5">
      <legend className={showLegend ? 'mb-1.5 text-sm font-medium text-text-primary' : 'sr-only'}>{legend}</legend>
      {palettes.map((p) => (
        <label key={p.id} title={p.name} className="relative inline-flex cursor-pointer">
          <input
            type="radio"
            name={name}
            value={p.id}
            checked={value === p.id}
            onChange={() => onChange(p.id)}
            aria-label={p.name}
            className="peer sr-only"
          />
          <span
            aria-hidden="true"
            className={cn(
              'block rounded-full border border-border-strong ring-offset-2 ring-offset-surface-0 peer-checked:ring-2 peer-checked:ring-brand-600 peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-4 peer-focus-visible:outline-brand-600',
              large ? 'h-8 w-8' : 'h-5 w-5',
            )}
            style={{ background: `linear-gradient(135deg, ${p.primary} 0 50%, ${p.secondary} 50% 100%)` }}
          />
        </label>
      ))}
    </fieldset>
  );
}
