import { describe, it, expect, vi } from 'vitest';
import { renderMathInElement, latexOfMathElement } from '../mathRender';
import { applyTracks, enhanceMediaInElement, mediaIdOf } from '../mediaEnhance';

function html(markup) {
  const div = document.createElement('div');
  div.innerHTML = markup;
  return div;
}

describe('renderMathInElement (G007)', () => {
  it('typesets Paper, Canvas and legacy Canvas equation markup', () => {
    const el = html(
      '<p><span class="math-tex" data-latex="x^2">\\(x^2\\)</span></p>'
      + '<p><span class="math_equation_latex">\\(\\frac{1}{2}\\)</span></p>'
      + '<p><img class="equation_image" src="/equation_images/y" alt="y" data-equation-content="y+1"></p>'
      + '<p>Inline \\(a+b\\) and $$c$$ text</p>'
      + '<pre>\\(not math\\)</pre>',
    );
    renderMathInElement(el);
    expect(el.querySelectorAll('.katex').length).toBe(5);
    // The legacy image (whose src would 404 here) is replaced.
    expect(el.querySelector('img.equation_image')).toBeNull();
    expect(el.querySelector('pre').textContent).toBe('\\(not math\\)');
    // Idempotent.
    renderMathInElement(el);
    expect(el.querySelectorAll('.katex').length).toBe(5);
  });

  it('reads LaTeX from each markup form', () => {
    expect(latexOfMathElement(html('<span class="math-tex">\\[x\\]</span>').firstChild)).toEqual({ latex: 'x', display: true });
    expect(latexOfMathElement(html('<span class="math-tex" data-latex="y" data-display="block"></span>').firstChild)).toEqual({ latex: 'y', display: true });
    expect(latexOfMathElement(html('<img data-equation-content="z">').firstChild)).toEqual({ latex: 'z', display: false });
  });
});

describe('media caption tracks (G002)', () => {
  const MEDIA = 'm-0123456789abcdef0123456789abcdef';

  it('finds the media id from data-media-id or the stream URL', () => {
    expect(mediaIdOf(html(`<video data-media-id="${MEDIA}"></video>`).firstChild)).toBe(MEDIA);
    expect(mediaIdOf(html(`<audio src="/api/v1/media_objects/${MEDIA}/stream"></audio>`).firstChild)).toBe(MEDIA);
    expect(mediaIdOf(html('<video src="https://cdn.example/x.mp4"></video>').firstChild)).toBeNull();
  });

  it('attaches each caption language once', async () => {
    const el = html(`<video src="/api/v1/media_objects/${MEDIA}/stream"></video><video src="/api/v1/media_objects/${MEDIA}/stream"></video>`);
    const loader = vi.fn().mockResolvedValue({
      media_tracks: [
        { locale: 'en', kind: 'captions', label: 'English', url: `/api/v1/media_objects/${MEDIA}/media_tracks/1/content` },
        { locale: 'es', kind: 'karaoke', label: 'Español', url: `/api/v1/media_objects/${MEDIA}/media_tracks/2/content` },
      ],
    });
    await enhanceMediaInElement(el, loader);
    await enhanceMediaInElement(el, loader);
    expect(loader).toHaveBeenCalledTimes(1); // cached per media id
    const tracks = el.querySelector('video').querySelectorAll('track');
    expect(tracks).toHaveLength(2);
    expect(tracks[0].getAttribute('kind')).toBe('captions');
    expect(tracks[1].getAttribute('kind')).toBe('subtitles'); // unknown kind normalized
    expect(el.querySelector('video').hasAttribute('controls')).toBe(true);
  });

  it('does not duplicate a language the element already has', () => {
    const el = html('<video><track srclang="en" src="/a.vtt"></video>').firstChild;
    expect(applyTracks(el, [{ locale: 'en', url: '/b.vtt' }, { locale: 'fr', url: '/c.vtt' }])).toBe(1);
  });
});
