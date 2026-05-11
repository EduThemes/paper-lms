import React from 'react';
import { Type, AlignLeft, Palette, Volume2, RotateCcw } from 'lucide-react';
import { useReadingPrefs, DEFAULT_PREFS } from '../contexts/ReadingPrefsContext';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../components/ui/card';
import { Button } from '../components/ui/button';
import { Label } from '../components/ui/label';
import { Separator } from '../components/ui/separator';
import Layout from '../components/Layout';

/* ---- Option groups (single source of truth, drives the whole UI) ---- */

const FONT_OPTIONS = [
  { value: 'system', label: 'System', sample: 'Aa' },
  { value: 'lexend', label: 'Lexend', sample: 'Aa' },
  { value: 'atkinson', label: 'Atkinson Hyperlegible', sample: 'Aa' },
  { value: 'opendyslexic', label: 'OpenDyslexic', sample: 'Aa' },
];
const SCALE_OPTIONS    = [{ value: 1.0, label: '100%' }, { value: 1.15, label: '115%' }, { value: 1.3, label: '130%' }, { value: 1.5, label: '150%' }];
const LINE_OPTIONS     = [{ value: 1.5, label: 'Cozy' }, { value: 1.75, label: 'Roomy' }, { value: 2.0, label: 'Airy' }];
const SPACING_OPTIONS  = [{ value: 0, label: 'Default' }, { value: 0.02, label: '+ Wide' }, { value: 0.05, label: '+ Wider' }];
const WIDTH_OPTIONS    = [{ value: 'none', label: 'Full' }, { value: '75ch', label: 'Wide' }, { value: '65ch', label: 'Comfort' }, { value: '55ch', label: 'Narrow' }];
const BG_OPTIONS       = [
  { value: 'white', label: 'White',    swatch: '#FFFFFF', ring: '#E5E7EB' },
  { value: 'cream', label: 'Cream',    swatch: '#FAF6E9', ring: '#E5DFC4' },
  { value: 'gray',  label: 'Soft Gray', swatch: '#F3F4F6', ring: '#D1D5DB' },
  { value: 'dark',  label: 'Night',    swatch: '#1E293B', ring: '#334155' },
];

/* ---- Generic chip-style segmented control ---- */

function OptionGrid({ value, onChange, options, render }) {
  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
      {options.map((opt) => {
        const active = String(opt.value) === String(value);
        return (
          <button
            key={String(opt.value)}
            type="button"
            onClick={() => onChange(opt.value)}
            aria-pressed={active}
            className={[
              'flex flex-col items-center justify-center gap-1 rounded-md border px-3 py-2.5 text-sm transition-colors',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
              active
                ? 'border-primary bg-primary text-primary-foreground shadow-sm'
                : 'border-input bg-background hover:bg-accent hover:text-accent-foreground',
            ].join(' ')}
          >
            {render ? render(opt, active) : <span>{opt.label}</span>}
          </button>
        );
      })}
    </div>
  );
}

/* ---- Page ---- */

const SAMPLE = `<h2>The Curious Otter</h2>
<p>Once upon a riverbank, a small otter named <em>Pip</em> discovered a smooth, shiny stone. It glittered in the sunlight like a tiny star caught in the water.</p>
<p>Pip wondered: <strong>where did it come from?</strong> She tucked the stone safely between her paws and floated downstream, listening to the song of the rushing water.</p>`;

export default function ReadingPreferencesPage() {
  const { prefs, setPrefs, reset } = useReadingPrefs();
  const set = (k) => (v) => setPrefs({ [k]: v });

  return (
    <Layout>
      <div className="mx-auto max-w-5xl space-y-6 p-4 sm:p-6">
        <header className="space-y-1">
          <h1 className="text-3xl font-semibold tracking-tight">Reading Preferences</h1>
          <p className="text-muted-foreground">
            Tune typography, spacing, and color so reading feels effortless.
            Your choices apply to every assignment, page, and announcement.
          </p>
        </header>

        {/* Live preview */}
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Live preview</CardTitle>
            <CardDescription>Updates instantly as you change settings below.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="reading-surface prose max-w-none" dangerouslySetInnerHTML={{ __html: SAMPLE }} />
          </CardContent>
        </Card>

        <div className="grid gap-6 md:grid-cols-2">
          {/* Typography */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg"><Type className="h-5 w-5" /> Typography</CardTitle>
              <CardDescription>Font face and size.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Font family</Label>
                <OptionGrid
                  value={prefs.fontFamily}
                  onChange={set('fontFamily')}
                  options={FONT_OPTIONS}
                  render={(o, active) => (
                    <>
                      <span className={['text-2xl leading-none', active ? '' : 'text-foreground'].join(' ')}>{o.sample}</span>
                      <span className="text-xs">{o.label}</span>
                    </>
                  )}
                />
              </div>
              <div className="space-y-2">
                <Label>Text size</Label>
                <OptionGrid value={prefs.fontScale} onChange={set('fontScale')} options={SCALE_OPTIONS} />
              </div>
              <div className="flex items-center justify-between rounded-md border border-input p-3">
                <div>
                  <Label className="text-sm font-medium">Disable italics</Label>
                  <p className="text-xs text-muted-foreground">Recommended for dyslexic readers.</p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={prefs.noItalic}
                  onClick={() => setPrefs({ noItalic: !prefs.noItalic })}
                  className={[
                    'relative h-6 w-11 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
                    prefs.noItalic ? 'bg-primary' : 'bg-muted',
                  ].join(' ')}
                >
                  <span className={['absolute top-0.5 h-5 w-5 rounded-full bg-surface-0 shadow transition-transform', prefs.noItalic ? 'translate-x-5' : 'translate-x-0.5'].join(' ')} />
                </button>
              </div>
            </CardContent>
          </Card>

          {/* Spacing */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg"><AlignLeft className="h-5 w-5" /> Spacing</CardTitle>
              <CardDescription>How loose the text feels.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Line height</Label>
                <OptionGrid value={prefs.lineHeight} onChange={set('lineHeight')} options={LINE_OPTIONS} />
              </div>
              <div className="space-y-2">
                <Label>Letter spacing</Label>
                <OptionGrid value={prefs.letterSpacing} onChange={set('letterSpacing')} options={SPACING_OPTIONS} />
              </div>
              <div className="space-y-2">
                <Label>Reading width</Label>
                <OptionGrid value={prefs.maxWidth} onChange={set('maxWidth')} options={WIDTH_OPTIONS} />
              </div>
            </CardContent>
          </Card>

          {/* Background */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg"><Palette className="h-5 w-5" /> Background</CardTitle>
              <CardDescription>Reduces glare and improves contrast.</CardDescription>
            </CardHeader>
            <CardContent>
              <OptionGrid
                value={prefs.bg}
                onChange={set('bg')}
                options={BG_OPTIONS}
                render={(o, active) => (
                  <>
                    <span
                      className="h-7 w-7 rounded-full border"
                      style={{ background: o.swatch, borderColor: active ? 'currentColor' : o.ring }}
                    />
                    <span className="text-xs">{o.label}</span>
                  </>
                )}
              />
            </CardContent>
          </Card>

          {/* Read aloud */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-lg"><Volume2 className="h-5 w-5" /> Read-aloud</CardTitle>
              <CardDescription>Hear pages spoken aloud.</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex items-center justify-between rounded-md border border-input p-3">
                <div>
                  <Label className="text-sm font-medium">Show read-aloud button</Label>
                  <p className="text-xs text-muted-foreground">Adds a speaker icon to readable content.</p>
                </div>
                <button
                  type="button"
                  role="switch"
                  aria-checked={prefs.ttsEnabled}
                  onClick={() => setPrefs({ ttsEnabled: !prefs.ttsEnabled })}
                  className={[
                    'relative h-6 w-11 rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
                    prefs.ttsEnabled ? 'bg-primary' : 'bg-muted',
                  ].join(' ')}
                >
                  <span className={['absolute top-0.5 h-5 w-5 rounded-full bg-surface-0 shadow transition-transform', prefs.ttsEnabled ? 'translate-x-5' : 'translate-x-0.5'].join(' ')} />
                </button>
              </div>
            </CardContent>
          </Card>
        </div>

        <Separator />

        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm text-muted-foreground">
            Changes save automatically to this device.
          </p>
          <Button
            variant="outline"
            onClick={reset}
            disabled={JSON.stringify(prefs) === JSON.stringify(DEFAULT_PREFS)}
          >
            <RotateCcw className="h-4 w-4" />
            Reset to defaults
          </Button>
        </div>
      </div>
    </Layout>
  );
}
