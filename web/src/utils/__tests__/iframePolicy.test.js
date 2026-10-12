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
    // Device / data features are never delegated (server parity).
    ['camera', "microphone 'src'", 'display-capture *', 'clipboard-read', 'midi', 'autoplay; camera *']
      .forEach((v) => expect(IFRAME_ALLOW_RE.test(v), v).toBe(false));
    expect(IFRAME_ALLOW_RE.test('accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share')).toBe(true);
  });

  it('drops a device-permission allow list in the viewer sanitizer', () => {
    const html = sanitizeHTML('<iframe src="https://codepen.io/x/embed/abc" allow="camera *; microphone *; display-capture *; clipboard-read *"></iframe>');
    expect(html).toContain('codepen.io/x/embed/abc');
    expect(html).not.toContain('allow=');
    expect(sanitizeHTML('<iframe src="https://www.youtube.com/embed/x" allow="autoplay; fullscreen"></iframe>'))
      .toContain('allow="autoplay; fullscreen"');
  });

  it('limits user-hostable hosts to their embed endpoints', () => {
    [
      'https://contoso.sharepoint.com/sites/Math/_layouts/15/embed.aspx?UniqueId=1',
      'https://forms.office.com/Pages/ResponsePage.aspx?id=abc&embed=true',
      'https://cdnapisec.kaltura.com/p/1/sp/100/embedIframeJs/uiconf_id/2/partner_id/1',
    ].forEach((src) => expect(isTrustedIframeSrc(src), src).toBe(true));
    [
      'https://contoso.sharepoint.com/sites/phish/SitePages/Login.aspx',
      'https://forms.office.com/',
      'https://attacker.kaltura.com/login.html',
      'https://codepen.io/x/pen/abc',
    ].forEach((src) => expect(isTrustedIframeSrc(src), src).toBe(false));
  });
});
