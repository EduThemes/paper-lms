import { describe, it, expect } from 'vitest';
import { toEmbedAttrs, isTrustedIframeSrc, IFRAME_ALLOW_RE } from '../iframePolicy';
import { sanitizeHTML } from '../../components/RichContentViewer';

describe('iframe allowlist (G001)', () => {
  it('accepts common education embeds and rejects lookalikes', () => {
    [
      'https://www.youtube.com/embed/abc123XYZ',
      'https://docs.google.com/presentation/d/abc/embed',
      'https://school.hosted.panopto.com/Panopto/Pages/Embed.aspx?id=1',
      'https://www.desmos.com/calculator/abcd',
      'https://phet.colorado.edu/sims/html/forces/latest/forces_en.html',
      'https://padlet.com/embed/xyz',
      '/courses/3/external_tools/retrieve?url=https%3A%2F%2Ftool.example',
    ].forEach((src) => expect(isTrustedIframeSrc(src), src).toBe(true));
    [
      'https://attacker.example/',
      'https://docs.google.com.attacker.example/document/',
      'http://www.youtube.com/embed/abc',
      '//attacker.example/courses/1/external_tools/retrieve?x',
    ].forEach((src) => expect(isTrustedIframeSrc(src), src).toBe(false));
  });

  it('keeps trusted embeds through the viewer sanitizer and drops others', () => {
    const html = sanitizeHTML('<iframe src="https://www.geogebra.org/material/iframe/id/abc"></iframe><iframe src="https://attacker.example/x"></iframe>');
    expect(html).toContain('geogebra.org');
    expect(html).not.toContain('attacker.example');
  });

  it('turns share links into embeddable URLs', () => {
    expect(toEmbedAttrs('https://youtu.be/abc123XYZ').src).toBe('https://www.youtube.com/embed/abc123XYZ');
    expect(toEmbedAttrs('https://www.youtube.com/watch?v=abc123XYZ&t=4').src).toBe('https://www.youtube.com/embed/abc123XYZ');
    expect(toEmbedAttrs('https://vimeo.com/123456').src).toBe('https://player.vimeo.com/video/123456');
    expect(toEmbedAttrs('https://www.loom.com/share/0123abcd').src).toBe('https://www.loom.com/embed/0123abcd');
    expect(toEmbedAttrs('https://docs.google.com/document/d/1AbC/edit?usp=sharing').src).toBe('https://docs.google.com/document/d/1AbC/preview');
    expect(toEmbedAttrs('https://docs.google.com/presentation/d/1AbC/edit#slide=id.p').src).toBe('https://docs.google.com/presentation/d/1AbC/embed');
    expect(toEmbedAttrs('https://drive.google.com/file/d/1AbC/view?usp=sharing').src).toBe('https://drive.google.com/file/d/1AbC/preview');
    expect(toEmbedAttrs('https://docs.google.com/forms/d/e/1FAIpQ/viewform').src).toContain('embedded=true');
    expect(toEmbedAttrs('https://www.desmos.com/calculator/abcd').src).toBe('https://www.desmos.com/calculator/abcd');
  });

  it('reads pasted embed code and keeps only allowed attributes', () => {
    const attrs = toEmbedAttrs('<iframe width="560" height="315" src="https://www.youtube.com/embed/abc123XYZ" title="Cells" allow="accelerometer; autoplay; encrypted-media" onload="alert(1)"></iframe>');
    expect(attrs).toEqual({
      src: 'https://www.youtube.com/embed/abc123XYZ', title: 'Cells', width: '560', height: '315', allow: 'accelerometer; autoplay; encrypted-media',
    });
    expect(toEmbedAttrs('<iframe src="https://attacker.example/phish"></iframe>')).toBeNull();
    expect(toEmbedAttrs('javascript:alert(1)')).toBeNull();
    expect(toEmbedAttrs('https://attacker.example/video')).toBeNull();
  });

  it('validates allow= the way the server does', () => {
    expect(IFRAME_ALLOW_RE.test('autoplay; fullscreen')).toBe(true);
    expect(IFRAME_ALLOW_RE.test('camera https://attacker.example')).toBe(false);
    expect(IFRAME_ALLOW_RE.test('geolocation')).toBe(false);
  });
});
