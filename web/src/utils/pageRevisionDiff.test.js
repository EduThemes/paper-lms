import { describe, test, expect } from 'vitest';
import { tokenize, diffTokens, diffRevisionText, diffStats, revisionText } from './pageRevisionDiff';

const join = (parts) => parts.map((p) => p.text).join('');

describe('page revision diff (G065)', () => {
  test('tokenize keeps whitespace so tokens rejoin exactly', () => {
    expect(tokenize('a  b\nc')).toEqual(['a', '  ', 'b', '\n', 'c']);
    expect(tokenize('')).toEqual([]);
  });

  test('identical text is a single unchanged part', () => {
    const parts = diffTokens(tokenize('same words here'), tokenize('same words here'));
    expect(parts).toEqual([{ type: 'same', text: 'same words here' }]);
  });

  test('a changed word shows as a removal and an addition', () => {
    const parts = diffTokens(tokenize('the quick fox'), tokenize('the slow fox'));
    expect(parts).toEqual([
      { type: 'same', text: 'the ' },
      { type: 'del', text: 'quick' },
      { type: 'add', text: 'slow' },
      { type: 'same', text: ' fox' },
    ]);
  });

  test('both sides can be rebuilt from the diff', () => {
    const before = 'Read chapter 1 before Monday and bring notes.';
    const after = 'Read chapters 1 and 2 before Tuesday, and bring your notes.';
    const parts = diffTokens(tokenize(before), tokenize(after));
    expect(join(parts.filter((p) => p.type !== 'add'))).toBe(before);
    expect(join(parts.filter((p) => p.type !== 'del'))).toBe(after);
  });

  test('revisionText keeps one line per block and drops markup', () => {
    expect(revisionText('<h2>Week 1</h2><p>Read <strong>chapter 1</strong>.</p><ul><li>a</li><li>b</li></ul>'))
      .toBe('Week 1\nRead chapter 1.\na\nb');
    expect(revisionText('')).toBe('');
  });

  test('markup-only changes produce no diff', () => {
    const parts = diffRevisionText('<p>Hello class</p>', '<p class="lead">Hello <em>class</em></p>');
    expect(parts.every((p) => p.type === 'same')).toBe(true);
  });

  test('diffStats counts words, not whitespace', () => {
    const parts = diffRevisionText('<p>one two three</p>', '<p>one four five three</p>');
    expect(diffStats(parts)).toEqual({ added: 2, removed: 1 });
  });

  test('revision text is inert', () => {
    // An onerror handler must not run while computing the text.
    window.__pwned = false;
    revisionText('<img src=x onerror="window.__pwned=true"><p>ok</p>');
    expect(window.__pwned).toBe(false);
  });
});
