/* global describe, test, expect, beforeEach */
import { render, act } from '@testing-library/react';
import { ReadingPrefsProvider, useReadingPrefs, FONT_STACKS } from '../ReadingPrefsContext';

let api;
function Grab() {
  api = useReadingPrefs();
  return null;
}

const rootStyle = () => document.documentElement.style;

// Designed pages write their heading font as
// var(--reading-font-choice, <theme font>): the variable must exist only
// when the reader picked a font of their own.
describe('ReadingPrefsProvider --reading-font-choice', () => {
  beforeEach(() => {
    try { window.localStorage.clear(); } catch { /* no storage */ }
    rootStyle().removeProperty('--reading-font-choice');
    rootStyle().removeProperty('--reading-font-family');
  });

  test('unset with the default system font; --reading-font-family still set', () => {
    render(<ReadingPrefsProvider><Grab /></ReadingPrefsProvider>);
    expect(rootStyle().getPropertyValue('--reading-font-choice')).toBe('');
    expect(rootStyle().getPropertyValue('--reading-font-family')).toBe(FONT_STACKS.system);
  });

  test('set when the reader picks a font, removed again on system', () => {
    render(<ReadingPrefsProvider><Grab /></ReadingPrefsProvider>);
    act(() => api.setPrefs({ fontFamily: 'opendyslexic' }));
    expect(rootStyle().getPropertyValue('--reading-font-choice')).toBe(FONT_STACKS.opendyslexic);
    expect(rootStyle().getPropertyValue('--reading-font-family')).toBe(FONT_STACKS.opendyslexic);

    act(() => api.setPrefs({ fontFamily: 'system' }));
    expect(rootStyle().getPropertyValue('--reading-font-choice')).toBe('');
    expect(rootStyle().getPropertyValue('--reading-font-family')).toBe(FONT_STACKS.system);
  });
});
