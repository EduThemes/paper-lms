import { describe, it, expect, afterEach, vi } from 'vitest';
import { Editor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Image from '@tiptap/extension-image';
import { MathInline, findTextEquations } from '../extensions/MathInline';
import { Media } from '../extensions/Media';
import { sanitizeHTML } from '../../RichContentViewer';
import {
  pickRecorderMimeType, recordingFilename, mediaNodeAttrs, isMediaFile, formatDuration,
} from '../mediaHelpers';

vi.mock('../../../services/api_rce_media', () => ({
  rceMediaApi: { getMediaObject: vi.fn().mockResolvedValue({ media_tracks: [] }) },
  rceMediaKeys: { detail: (id) => ['rceMedia', id], mine: () => ['rceMedia', 'mine'] },
  uploadMedia: vi.fn(),
}));

let editor;
function makeEditor(content = '') {
  editor = new Editor({
    extensions: [StarterKit.configure({ link: false }), Link.configure({ openOnClick: false }), Image, MathInline, Media],
    content,
  });
  return editor;
}

afterEach(() => {
  editor?.destroy();
  editor = null;
});

const nodesOf = (ed, name) => {
  const out = [];
  ed.state.doc.descendants((n, pos) => { if (n.type.name === name) out.push({ node: n, pos }); });
  return out;
};

describe('equation node (G007)', () => {
  it('turns stored \\( \\) text into an editable equation node', async () => {
    const ed = makeEditor('<p>Area is \\(\\pi r^2\\) units</p>');
    // The conversion runs on the editor's create event (next tick).
    await new Promise((r) => setTimeout(r, 0));
    const eqs = nodesOf(ed, 'mathInline');
    expect(eqs).toHaveLength(1);
    expect(eqs[0].node.attrs.latex).toBe('\\pi r^2');
    const out = ed.getHTML();
    expect(out).toContain('class="math-tex"');
    expect(out).toContain('data-latex="\\pi r^2"');
    // The stored form survives the viewer sanitizer.
    expect(sanitizeHTML(out)).toContain('data-latex="\\pi r^2"');
  });

  it('parses Canvas equation markup and leaves code alone', () => {
    const ed = makeEditor('<p><img class="equation_image" data-equation-content="x+1" src="/equation_images/x"> <span class="math_equation_latex">\\(y\\)</span></p><pre><code>\\(not\\)</code></pre>');
    expect(nodesOf(ed, 'mathInline').map((e) => e.node.attrs.latex)).toEqual(['x+1', 'y']);
    expect(ed.getText()).toContain('\\(not\\)');
  });

  it('inserts and updates equations by command', () => {
    const ed = makeEditor('<p>a</p>');
    ed.chain().focus('end').insertMath('x<y & z').run();
    let [eq] = nodesOf(ed, 'mathInline');
    expect(eq.node.attrs.latex).toBe('x<y & z');
    expect(ed.commands.updateMath(eq.pos, '\\sqrt{2}')).toBe(true);
    [eq] = nodesOf(ed, 'mathInline');
    expect(eq.node.attrs.latex).toBe('\\sqrt{2}');
  });

  it('finds equations in text', () => {
    expect(findTextEquations('a \\(x\\) b \\[y\\]')).toEqual([
      { start: 2, end: 7, latex: 'x', display: false },
      { start: 10, end: 15, latex: 'y', display: true },
    ]);
    expect(findTextEquations('no math')).toEqual([]);
  });
});

describe('media node (G002)', () => {
  const MEDIA = 'm-0123456789abcdef0123456789abcdef';

  it('keeps <video> and <audio> through a load/save round trip', () => {
    const ed = makeEditor(`<p>Watch</p><video controls src="/api/v1/media_objects/${MEDIA}/stream" data-media-id="${MEDIA}" title="Lab"></video><audio src="https://cdn.example/a.mp3"><track kind="captions" srclang="en" label="English" src="https://cdn.example/a.vtt"></audio>`);
    const out = ed.getHTML();
    expect(out).toContain(`data-media-id="${MEDIA}"`);
    expect(out).toContain('<audio');
    expect(out).toContain('srclang="en"');
    const clean = sanitizeHTML(out);
    expect(clean).toContain(`src="/api/v1/media_objects/${MEDIA}/stream"`);
    expect(clean).toContain('<track');
  });

  it('inserts an uploaded media object and refuses unsafe sources', () => {
    const ed = makeEditor('<p>a</p>');
    const attrs = mediaNodeAttrs({ media_id: MEDIA, media_type: 'audio', title: 'Interview', stream_url: `/api/v1/media_objects/${MEDIA}/stream` });
    expect(ed.chain().focus('end').setMedia(attrs).run()).toBe(true);
    expect(ed.getHTML()).toContain('<audio');
    expect(ed.commands.setMedia({ src: 'javascript:alert(1)' })).toBe(false);
    expect(makeEditor('<video src="javascript:alert(1)"></video>').getHTML()).not.toContain('javascript');
  });
});

describe('media helpers', () => {
  it('picks the first recorder format the browser supports', () => {
    expect(pickRecorderMimeType('audio', (t) => t === 'audio/mp4')).toBe('audio/mp4');
    expect(pickRecorderMimeType('video', () => false)).toBe('');
  });

  it('names recordings after their type', () => {
    expect(recordingFilename('audio/webm;codecs=opus', 'audio')).toMatch(/^audio-recording-.*\.webm$/);
    expect(recordingFilename('video/mp4', 'video')).toMatch(/^video-recording-.*\.mp4$/);
  });

  it('recognizes media files and formats durations', () => {
    expect(isMediaFile({ type: 'video/quicktime', name: 'a.mov' })).toBe(true);
    expect(isMediaFile({ type: '', name: 'talk.m4a' })).toBe(true);
    expect(isMediaFile({ type: 'application/pdf', name: 'a.pdf' })).toBe(false);
    expect(formatDuration(605)).toBe('10:05');
  });
});
