import { describe, expect, it } from 'vitest';
import { sanitizeRich, safeHref } from '../index';

describe('sanitizeRich', () => {
  const cases: [string, string, string][] = [
    ['keeps the allowed set', '<p>Hi <strong>there</strong> <em>you</em><br>next</p><ul><li>a</li></ul><ol><li>b</li></ol>', '<p>Hi <strong>there</strong> <em>you</em><br>next</p><ul><li>a</li></ul><ol><li>b</li></ol>'],
    ['b/i become strong/em', '<p><b>B</b> <i>I</i></p>', '<p><strong>B</strong> <em>I</em></p>'],
    ['strips attributes and styles', '<p style="color:red" class="x" onclick="evil()">t</p>', '<p>t</p>'],
    ['drops script/style/svg with contents', '<p>a</p><script>alert(1)</script><style>p{}</style><svg><text>x</text></svg>', '<p>a</p>'],
    ['unwraps unknown inline tags, keeps text', '<p><span style="x">in</span><font>side</font> <u>u</u></p>', '<p>inside u</p>'],
    ['div/h3 become paragraphs', '<div>one</div><h3>two</h3>', '<p>one</p><p>two</p>'],
    ['div holding a list is unwrapped', '<div><p>x</p><ul><li>y</li></ul></div>', '<p>x</p><ul><li>y</li></ul>'],
    ['stray list content becomes li', '<ul>text<li>a</li></ul>', '<ul><li>text</li><li>a</li></ul>'],
    ['https, relative, {course} and mailto links survive', '<a href="https://a.org">A</a> <a href="/courses/{course}/pages/x">B</a> <a href="mailto:t@s.edu">C</a>', '<a href="https://a.org">A</a> <a href="/courses/{course}/pages/x">B</a> <a href="mailto:t@s.edu">C</a>'],
    ['http is upgraded to https', '<a href="http://a.org/x">A</a>', '<a href="https://a.org/x">A</a>'],
    ['javascript: (even obfuscated) drops the link, keeps text', '<a href="jav&#x61;script:alert(1)">A</a> <a href=" JaVaScRiPt:x">B</a> <a href="data:text/html,x">C</a>', 'A B C'],
    ['tel: and ftp: are not allowed', '<a href="tel:123">T</a><a href="ftp://x">F</a>', 'TF'],
    ['nested links are flattened', '<a href="/a">x <a href="/b">y</a></a>', '<a href="/a">x y</a>'],
    ['comments removed', '<p>a<!-- hidden --></p>', '<p>a</p>'],
    ['empty paragraphs removed', '<p></p><p>&nbsp;</p><p><br></p><p>x</p>', '<p>x</p>'],
    ['escapes bare < & but keeps entities', '<p>a < b & c &eacute; &amp;</p>', '<p>a &lt; b &amp; c &eacute; &amp;</p>'],
    ['images and tables are removed, text kept', '<p><img src="x.png" alt="pic">Hi</p><table><tr><td>cell</td></tr></table>', '<p>Hi</p><p>cell</p>'],
  ];
  for (const [name, input, out] of cases) it(name, () => expect(sanitizeRich(input)).toBe(out));

  it('is idempotent', () => {
    for (const [, input] of cases) expect(sanitizeRich(sanitizeRich(input))).toBe(sanitizeRich(input));
  });
  it('empty input', () => expect(sanitizeRich('')).toBe(''));
});

describe('safeHref', () => {
  it('normalises', () => {
    expect(safeHref('//cdn.org/x')).toBe('https://cdn.org/x');
    expect(safeHref('#top')).toBe('#top');
    expect(safeHref('page?x=1')).toBe('page?x=1');
    expect(safeHref('vbscript:x')).toBeNull();
    expect(safeHref('')).toBeNull();
  });
});
