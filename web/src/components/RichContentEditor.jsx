import React, { useState, useRef, useCallback, useEffect } from 'react';
import {
  Bold, Italic, Underline, Strikethrough, Heading2, Heading3,
  List, ListOrdered, Quote, Minus, Link, Image, Table, Calculator,
  Play, Shield, Code, Type, RemoveFormatting, X, Check, ChevronDown,
  Upload, AlertTriangle
} from 'lucide-react';
import katex from 'katex';
import { api } from '../services/api';
import { useClickOutside } from '../hooks/useClickOutside';

/* --- Helpers ------------------------------------------------------------ */

const isMac = typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform);
const modKey = isMac ? '\u2318' : 'Ctrl';

function cx(...args) {
  return args.filter(Boolean).join(' ');
}

/* --- Toolbar Button ----------------------------------------------------- */

function ToolbarButton({ icon: Icon, label, shortcut, active, disabled, onClick, className }) {
  return (
    <button
      type="button"
      className={cx(
        'relative flex items-center justify-center w-8 h-8 rounded transition-colors',
        'hover:bg-border-default focus:outline-none focus:ring-2 focus:ring-blue-400 focus:ring-offset-1',
        active && 'bg-brand-100 text-brand-700',
        disabled && 'opacity-40 pointer-events-none',
        !active && !disabled && 'text-text-secondary',
        className,
      )}
      onMouseDown={(e) => {
        e.preventDefault();
        if (!disabled) onClick?.();
      }}
      aria-label={label}
      aria-pressed={active || undefined}
      title={shortcut ? `${label} (${shortcut})` : label}
      tabIndex={-1}
      disabled={disabled}
    >
      <Icon size={16} strokeWidth={2} />
    </button>
  );
}

function Separator() {
  return <div className="w-px h-6 bg-gray-300 mx-1 self-center shrink-0" />;
}

/* --- Popover shell ------------------------------------------------------ */

function Popover({ open, onClose, children, className }) {
  const ref = useRef(null);

  // Popover is only mounted when `open=true` (see early-return below), so the
  // hook's listener registers AFTER the click that opened the popover has
  // already propagated — no need for the previous setTimeout-defer guard.
  useClickOutside(ref, onClose);

  if (!open) return null;

  return (
    <div
      ref={ref}
      className={cx(
        'absolute top-full left-0 mt-1 z-50 bg-surface-0 border border-border-default rounded-lg shadow-lg p-3',
        'before:absolute before:-top-1.5 before:left-4 before:w-3 before:h-3',
        'before:bg-surface-0 before:border-l before:border-t before:border-border-default before:rotate-45',
        className,
      )}
    >
      {children}
    </div>
  );
}

/* --- Link Popover ------------------------------------------------------- */

function LinkPopover({ open, onClose, onInsert }) {
  const [url, setUrl] = useState('https://');
  const [text, setText] = useState('');
  const urlRef = useRef(null);

  useEffect(() => {
    if (open && urlRef.current) urlRef.current.focus();
  }, [open]);

  const handleInsert = () => {
    if (!url || url === 'https://') return;
    onInsert(url, text);
    setUrl('https://');
    setText('');
    onClose();
  };

  return (
    <Popover open={open} onClose={onClose} className="w-72">
      <label className="block text-xs font-medium text-text-secondary mb-1">URL</label>
      <input
        ref={urlRef}
        type="url"
        className="w-full border border-border-strong rounded px-2 py-1.5 text-sm focus:ring-2 focus:ring-blue-400 focus:outline-none mb-2"
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter') handleInsert(); }}
        placeholder="https://example.com"
      />
      <label className="block text-xs font-medium text-text-secondary mb-1">Display text (optional)</label>
      <input
        type="text"
        className="w-full border border-border-strong rounded px-2 py-1.5 text-sm focus:ring-2 focus:ring-blue-400 focus:outline-none mb-3"
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter') handleInsert(); }}
        placeholder="Link text"
      />
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onClose} className="px-3 py-1 text-sm text-text-secondary hover:bg-surface-2 rounded">Cancel</button>
        <button type="button" onClick={handleInsert} className="px-3 py-1 text-sm bg-brand-600 text-white rounded hover:bg-brand-700">Insert Link</button>
      </div>
    </Popover>
  );
}

/* --- Image Popover (URL + Upload tabs) ---------------------------------- */

function ImagePopover({ open, onClose, onInsert, courseId }) {
  const [tab, setTab] = useState(courseId ? 'upload' : 'url');
  const [url, setUrl] = useState('');
  const [alt, setAlt] = useState('');
  const [selectedFile, setSelectedFile] = useState(null);
  const [preview, setPreview] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploadError, setUploadError] = useState('');
  const urlRef = useRef(null);
  const fileRef = useRef(null);

  useEffect(() => {
    if (open && tab === 'url' && urlRef.current) urlRef.current.focus();
  }, [open, tab]);

  useEffect(() => {
    if (!open) {
      setUrl('');
      setAlt('');
      setSelectedFile(null);
      setPreview(null);
      setUploading(false);
      setUploadProgress(0);
      setUploadError('');
      setTab(courseId ? 'upload' : 'url');
    }
  }, [open, courseId]);

  useEffect(() => {
    if (!selectedFile) { setPreview(null); return; }
    const objectUrl = URL.createObjectURL(selectedFile);
    setPreview(objectUrl);
    return () => URL.revokeObjectURL(objectUrl);
  }, [selectedFile]);

  const handleInsertUrl = () => {
    if (!url) return;
    onInsert(url, alt);
    onClose();
  };

  const handleUpload = async () => {
    if (!selectedFile || !courseId) return;
    setUploading(true);
    setUploadError('');
    try {
      const attachment = await api.uploadCourseFileWithProgress(courseId, selectedFile, setUploadProgress);
      const imgUrl = `/api/v1/files/${attachment.id}/download`;
      onInsert(imgUrl, alt);
      onClose();
    } catch (err) {
      setUploadError(err.message || 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const handleFileChange = (e) => {
    const file = e.target.files?.[0];
    if (file) {
      setSelectedFile(file);
      setUploadError('');
      if (!alt) setAlt(file.name.replace(/\.[^.]+$/, ''));
    }
  };

  return (
    <Popover open={open} onClose={onClose} className="w-80">
      {courseId && (
        <div className="flex gap-1 mb-3 border-b border-border-default">
          <button
            type="button"
            className={cx('px-3 py-1.5 text-xs font-medium border-b-2 -mb-px transition-colors', tab === 'upload' ? 'border-brand-600 text-brand-700' : 'border-transparent text-text-tertiary hover:text-text-secondary')}
            onClick={() => setTab('upload')}
          >
            <Upload size={12} className="inline mr-1 -mt-0.5" />Upload
          </button>
          <button
            type="button"
            className={cx('px-3 py-1.5 text-xs font-medium border-b-2 -mb-px transition-colors', tab === 'url' ? 'border-brand-600 text-brand-700' : 'border-transparent text-text-tertiary hover:text-text-secondary')}
            onClick={() => setTab('url')}
          >
            URL
          </button>
        </div>
      )}

      {tab === 'url' ? (
        <>
          <label className="block text-xs font-medium text-text-secondary mb-1">Image URL</label>
          <input
            ref={urlRef}
            type="url"
            className="w-full border border-border-strong rounded px-2 py-1.5 text-sm focus:ring-2 focus:ring-blue-400 focus:outline-none mb-2"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') handleInsertUrl(); }}
            placeholder="https://example.com/image.png"
          />
          <label className="block text-xs font-medium text-text-secondary mb-1">Alt text (for accessibility)</label>
          <input
            type="text"
            className="w-full border border-border-strong rounded px-2 py-1.5 text-sm focus:ring-2 focus:ring-blue-400 focus:outline-none mb-3"
            value={alt}
            onChange={(e) => setAlt(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') handleInsertUrl(); }}
            placeholder="Describe the image"
          />
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="px-3 py-1 text-sm text-text-secondary hover:bg-surface-2 rounded">Cancel</button>
            <button type="button" onClick={handleInsertUrl} className="px-3 py-1 text-sm bg-brand-600 text-white rounded hover:bg-brand-700">Insert Image</button>
          </div>
        </>
      ) : (
        <>
          <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={handleFileChange} />
          {!selectedFile ? (
            <button
              type="button"
              onClick={() => fileRef.current?.click()}
              className="w-full border-2 border-dashed border-border-strong rounded-lg p-6 text-center hover:border-blue-400 hover:bg-brand-50 transition-colors"
            >
              <Upload size={24} className="mx-auto mb-2 text-text-disabled" />
              <span className="text-sm text-text-secondary">Click to choose an image</span>
              <span className="block text-xs text-text-disabled mt-1">or drag & drop onto the editor</span>
            </button>
          ) : (
            <div className="space-y-2">
              {preview && (
                <div className="flex justify-center p-2 bg-surface-1 rounded border border-border-default">
                  <img src={preview} alt="Preview" className="max-h-32 max-w-full rounded" />
                </div>
              )}
              <div className="flex items-center justify-between text-xs text-text-tertiary">
                <span className="truncate max-w-[180px]">{selectedFile.name}</span>
                <button type="button" onClick={() => { setSelectedFile(null); setPreview(null); }} className="text-text-disabled hover:text-text-secondary ml-2"><X size={14} /></button>
              </div>
            </div>
          )}
          <label className="block text-xs font-medium text-text-secondary mb-1 mt-2">Alt text (for accessibility)</label>
          <input
            type="text"
            className="w-full border border-border-strong rounded px-2 py-1.5 text-sm focus:ring-2 focus:ring-blue-400 focus:outline-none mb-2"
            value={alt}
            onChange={(e) => setAlt(e.target.value)}
            placeholder="Describe the image"
          />
          {uploading && (
            <div className="w-full bg-border-default rounded-full h-1.5 mb-2">
              <div className="bg-brand-600 h-1.5 rounded-full transition-all duration-200" style={{ width: `${Math.round(uploadProgress * 100)}%` }} />
            </div>
          )}
          {uploadError && <p className="text-xs text-accent-danger mb-2">{uploadError}</p>}
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="px-3 py-1 text-sm text-text-secondary hover:bg-surface-2 rounded">Cancel</button>
            <button
              type="button"
              onClick={handleUpload}
              disabled={!selectedFile || uploading}
              className="px-3 py-1 text-sm bg-brand-600 text-white rounded hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {uploading ? 'Uploading...' : 'Upload & Insert'}
            </button>
          </div>
        </>
      )}
    </Popover>
  );
}

/* --- Table Grid Selector ------------------------------------------------ */

function TablePopover({ open, onClose, onInsert }) {
  const [hover, setHover] = useState({ r: 0, c: 0 });
  const maxRows = 6;
  const maxCols = 6;

  const handleSelect = (r, c) => {
    onInsert(r + 1, c + 1);
    onClose();
  };

  return (
    <Popover open={open} onClose={onClose} className="w-auto">
      <p className="text-xs text-text-tertiary mb-2 text-center">
        {hover.r + 1} &times; {hover.c + 1} table
      </p>
      <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${maxCols}, 1fr)` }}>
        {Array.from({ length: maxRows }).map((_, r) =>
          Array.from({ length: maxCols }).map((_, c) => (
            <div
              key={`${r}-${c}`}
              className={cx(
                'w-5 h-5 border rounded-sm cursor-pointer transition-colors',
                r <= hover.r && c <= hover.c
                  ? 'bg-blue-200 border-blue-400'
                  : 'bg-surface-1 border-border-strong',
              )}
              onMouseEnter={() => setHover({ r, c })}
              onMouseDown={(e) => {
                e.preventDefault();
                handleSelect(r, c);
              }}
            />
          ))
        )}
      </div>
    </Popover>
  );
}

/* --- Equation Editor Popover -------------------------------------------- */

const EQUATION_SHORTCUTS = [
  { label: '\\frac{a}{b}', display: 'a/b', title: 'Fraction' },
  { label: '\\sqrt{x}', display: '\u221Ax', title: 'Square root' },
  { label: 'x^{n}', display: 'x\u207F', title: 'Exponent' },
  { label: 'x_{n}', display: 'x\u2099', title: 'Subscript' },
  { label: '\\sum_{i=0}^{n}', display: '\u03A3', title: 'Sum' },
  { label: '\\int_{a}^{b}', display: '\u222B', title: 'Integral' },
];

function EquationPopover({ open, onClose, onInsert }) {
  const [latex, setLatex] = useState('');
  const inputRef = useRef(null);

  useEffect(() => {
    if (open && inputRef.current) inputRef.current.focus();
  }, [open]);

  const handleInsert = () => {
    if (!latex.trim()) return;
    onInsert(latex.trim());
    setLatex('');
    onClose();
  };

  return (
    <Popover open={open} onClose={onClose} className="w-80">
      <label className="block text-xs font-medium text-text-secondary mb-1">LaTeX expression</label>
      <input
        ref={inputRef}
        type="text"
        className="w-full border border-border-strong rounded px-2 py-1.5 text-sm font-mono focus:ring-2 focus:ring-blue-400 focus:outline-none mb-2"
        value={latex}
        onChange={(e) => setLatex(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Enter') handleInsert(); }}
        placeholder="e.g. \\frac{1}{2}"
      />
      <div className="flex flex-wrap gap-1 mb-3">
        {EQUATION_SHORTCUTS.map((s) => (
          <button
            key={s.label}
            type="button"
            className="px-2 py-1 text-xs border border-border-strong rounded hover:bg-surface-2 font-mono"
            title={s.title}
            onMouseDown={(e) => {
              e.preventDefault();
              setLatex((prev) => prev + s.label);
              inputRef.current?.focus();
            }}
          >
            {s.display}
          </button>
        ))}
      </div>
      {latex.trim() && (
        <div className="mb-3 p-2 bg-surface-1 rounded border border-border-default text-center">
          <span
            dangerouslySetInnerHTML={{
              __html: (() => {
                try {
                  return katex.renderToString(latex, { throwOnError: false, displayMode: false });
                } catch {
                  return `<span class="text-sm italic text-text-primary">${latex.replace(/</g, '&lt;')}</span>`;
                }
              })()
            }}
          />
        </div>
      )}
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onClose} className="px-3 py-1 text-sm text-text-secondary hover:bg-surface-2 rounded">Cancel</button>
        <button type="button" onClick={handleInsert} className="px-3 py-1 text-sm bg-brand-600 text-white rounded hover:bg-brand-700">Insert Equation</button>
      </div>
    </Popover>
  );
}

/* --- Media Embed Popover ------------------------------------------------ */

function parseMediaUrl(url) {
  let match = url.match(/(?:youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]{11})/);
  if (match) return { type: 'youtube', id: match[1] };
  match = url.match(/vimeo\.com\/(\d+)/);
  if (match) return { type: 'vimeo', id: match[1] };
  if (/\.(mp4|webm|ogg)(\?|$)/i.test(url)) return { type: 'video', url };
  if (/\.(mp3|wav|ogg|m4a)(\?|$)/i.test(url)) return { type: 'audio', url };
  return null;
}

function MediaPopover({ open, onClose, onInsert }) {
  const [url, setUrl] = useState('');
  const [error, setError] = useState('');
  const urlRef = useRef(null);

  useEffect(() => {
    if (open && urlRef.current) urlRef.current.focus();
  }, [open]);

  const handleInsert = () => {
    const parsed = parseMediaUrl(url);
    if (!parsed) {
      setError('Unsupported URL. Use YouTube, Vimeo, or direct video/audio URL.');
      return;
    }
    onInsert(parsed);
    setUrl('');
    setError('');
    onClose();
  };

  return (
    <Popover open={open} onClose={onClose} className="w-80">
      <label className="block text-xs font-medium text-text-secondary mb-1">Media URL</label>
      <input
        ref={urlRef}
        type="url"
        className="w-full border border-border-strong rounded px-2 py-1.5 text-sm focus:ring-2 focus:ring-blue-400 focus:outline-none mb-1"
        value={url}
        onChange={(e) => { setUrl(e.target.value); setError(''); }}
        onKeyDown={(e) => { if (e.key === 'Enter') handleInsert(); }}
        placeholder="https://youtube.com/watch?v=..."
      />
      {error && <p className="text-xs text-accent-danger mb-2">{error}</p>}
      <p className="text-xs text-text-disabled mb-3">Supports YouTube, Vimeo, .mp4, .webm, .mp3, .wav</p>
      <div className="flex justify-end gap-2">
        <button type="button" onClick={onClose} className="px-3 py-1 text-sm text-text-secondary hover:bg-surface-2 rounded">Cancel</button>
        <button type="button" onClick={handleInsert} className="px-3 py-1 text-sm bg-brand-600 text-white rounded hover:bg-brand-700">Embed</button>
      </div>
    </Popover>
  );
}

/* --- Accessibility Checker ---------------------------------------------- */

function runAccessibilityCheck(html) {
  const issues = [];
  const container = document.createElement('div');
  container.innerHTML = html;

  container.querySelectorAll('img').forEach((img, i) => {
    if (!img.getAttribute('alt')?.trim()) {
      issues.push({ type: 'error', message: `Image ${i + 1} is missing alt text`, suggestion: 'Add descriptive alt text to the image.' });
    }
  });

  container.querySelectorAll('a').forEach((a, i) => {
    if (!a.textContent?.trim() && !a.querySelector('img')) {
      issues.push({ type: 'error', message: `Link ${i + 1} has no text`, suggestion: 'Add visible text or an aria-label to the link.' });
    }
  });

  const headings = container.querySelectorAll('h1, h2, h3, h4, h5, h6');
  let lastLevel = 0;
  headings.forEach((h) => {
    const level = parseInt(h.tagName[1], 10);
    if (lastLevel && level > lastLevel + 1) {
      issues.push({
        type: 'warning',
        message: `Heading hierarchy skips from H${lastLevel} to H${level}`,
        suggestion: `Use an H${lastLevel + 1} instead, or restructure your headings.`,
      });
    }
    lastLevel = level;
  });

  container.querySelectorAll('[style]').forEach((el) => {
    const style = el.getAttribute('style') || '';
    if (/color\s*:/i.test(style) && !/background/i.test(style)) {
      issues.push({
        type: 'warning',
        message: 'Inline text color detected without background color',
        suggestion: 'Ensure sufficient contrast between text and background colors (WCAG 4.5:1 ratio).',
      });
    }
  });

  return issues;
}

function AccessibilityPanel({ open, onClose, issues }) {
  if (!open) return null;

  return (
    <Popover open={open} onClose={onClose} className="w-80 max-h-64 overflow-y-auto right-0 left-auto">
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold text-text-primary">Accessibility Check</h3>
        <button type="button" onClick={onClose} className="text-text-disabled hover:text-text-secondary">
          <X size={14} />
        </button>
      </div>
      {issues.length === 0 ? (
        <div className="flex items-center gap-2 text-accent-success text-sm py-2">
          <Check size={16} /> No issues found
        </div>
      ) : (
        <ul className="space-y-2">
          {issues.map((issue, i) => (
            <li key={i} className="text-xs">
              <div className={cx(
                'font-medium',
                issue.type === 'error' ? 'text-accent-danger' : 'text-accent-warning',
              )}>
                {issue.type === 'error' ? 'Error' : 'Warning'}: {issue.message}
              </div>
              <div className="text-text-tertiary mt-0.5">{issue.suggestion}</div>
            </li>
          ))}
        </ul>
      )}
    </Popover>
  );
}

/* --- Main Rich Content Editor ------------------------------------------- */

export default function RichContentEditor({
  value = '',
  onChange,
  placeholder = 'Start typing...',
  minHeight = '200px',
  disabled = false,
  id,
  ariaLabel = 'Rich content editor',
  courseId,
}) {
  const editorRef = useRef(null);
  const [sourceView, setSourceView] = useState(false);
  const [sourceHtml, setSourceHtml] = useState(value);
  const [charCount, setCharCount] = useState(0);
  const [activeFormats, setActiveFormats] = useState({});

  const [linkOpen, setLinkOpen] = useState(false);
  const [imageOpen, setImageOpen] = useState(false);
  const [tableOpen, setTableOpen] = useState(false);
  const [equationOpen, setEquationOpen] = useState(false);
  const [mediaOpen, setMediaOpen] = useState(false);
  const [a11yOpen, setA11yOpen] = useState(false);
  const [a11yIssues, setA11yIssues] = useState([]);

  const [dragOver, setDragOver] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(null);
  const [uploadError, setUploadError] = useState(null);

  const savedSelectionRef = useRef(null);

  const saveSelection = useCallback(() => {
    const sel = window.getSelection();
    if (sel && sel.rangeCount > 0) {
      savedSelectionRef.current = sel.getRangeAt(0).cloneRange();
    }
  }, []);

  const restoreSelection = useCallback(() => {
    if (savedSelectionRef.current) {
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(savedSelectionRef.current);
    }
  }, []);

  useEffect(() => {
    if (editorRef.current && !sourceView) {
      if (editorRef.current.innerHTML !== value) {
        editorRef.current.innerHTML = value || '';
      }
      updateCharCount();
    }
  }, [value, sourceView]);

  const updateCharCount = useCallback(() => {
    if (editorRef.current) {
      setCharCount(editorRef.current.textContent?.length || 0);
    }
  }, []);

  const updateActiveFormats = useCallback(() => {
    const formats = {};
    try {
      formats.bold = document.queryCommandState('bold');
      formats.italic = document.queryCommandState('italic');
      formats.underline = document.queryCommandState('underline');
      formats.strikeThrough = document.queryCommandState('strikeThrough');
      formats.insertUnorderedList = document.queryCommandState('insertUnorderedList');
      formats.insertOrderedList = document.queryCommandState('insertOrderedList');
      const val = document.queryCommandValue('formatBlock');
      formats.h2 = val === 'h2';
      formats.h3 = val === 'h3';
      formats.blockquote = val === 'blockquote';
    } catch (_e) {
      // queryCommandState may throw in some browsers
    }
    setActiveFormats(formats);
  }, []);

  const handleInput = useCallback(() => {
    if (editorRef.current) {
      const html = editorRef.current.innerHTML;
      onChange?.(html);
      updateCharCount();
      updateActiveFormats();
    }
  }, [onChange, updateCharCount, updateActiveFormats]);

  const handleSelectionChange = useCallback(() => {
    updateActiveFormats();
  }, [updateActiveFormats]);

  useEffect(() => {
    document.addEventListener('selectionchange', handleSelectionChange);
    return () => document.removeEventListener('selectionchange', handleSelectionChange);
  }, [handleSelectionChange]);

  const exec = useCallback((command, val) => {
    const el = editorRef.current;
    if (!el) return;
    el.focus();
    // Ensure editor has at least one block element for formatting commands to work
    if (!el.innerHTML || el.innerHTML === '<br>' || el.innerHTML.trim() === '') {
      el.innerHTML = '<p><br></p>';
      const range = document.createRange();
      range.selectNodeContents(el.querySelector('p'));
      range.collapse(true);
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
    }
    document.execCommand(command, false, val === undefined ? null : val);
    handleInput();
  }, [handleInput]);

  const toggleBold = useCallback(() => exec('bold'), [exec]);
  const toggleItalic = useCallback(() => exec('italic'), [exec]);
  const toggleUnderline = useCallback(() => exec('underline'), [exec]);
  const toggleStrikethrough = useCallback(() => exec('strikeThrough'), [exec]);

  const toggleH2 = useCallback(() => {
    const current = document.queryCommandValue('formatBlock')?.toLowerCase();
    exec('formatBlock', current === 'h2' ? '<p>' : '<h2>');
  }, [exec]);

  const toggleH3 = useCallback(() => {
    const current = document.queryCommandValue('formatBlock')?.toLowerCase();
    exec('formatBlock', current === 'h3' ? '<p>' : '<h3>');
  }, [exec]);

  const toggleBulletList = useCallback(() => exec('insertUnorderedList'), [exec]);
  const toggleNumberedList = useCallback(() => exec('insertOrderedList'), [exec]);

  const toggleBlockquote = useCallback(() => {
    const current = document.queryCommandValue('formatBlock')?.toLowerCase();
    exec('formatBlock', current === 'blockquote' ? '<p>' : '<blockquote>');
  }, [exec]);

  const insertHR = useCallback(() => exec('insertHorizontalRule'), [exec]);
  const clearFormatting = useCallback(() => exec('removeFormat'), [exec]);

  const handleInsertLink = useCallback((url, text) => {
    const el = editorRef.current;
    if (!el) return;
    restoreSelection();
    el.focus();
    // If editor is empty, add a paragraph first
    if (!el.innerHTML || el.innerHTML === '<br>' || el.innerHTML.trim() === '') {
      el.innerHTML = '<p><br></p>';
      const range = document.createRange();
      range.selectNodeContents(el.querySelector('p'));
      range.collapse(true);
      window.getSelection().removeAllRanges();
      window.getSelection().addRange(range);
    }
    const sel = window.getSelection();
    if (sel && sel.rangeCount > 0) {
      const range = sel.getRangeAt(0);
      const selectedText = range.toString();
      if (selectedText) {
        document.execCommand('createLink', false, url);
      } else {
        const displayText = text || url;
        const safeUrl = url.replace(/"/g, '&quot;');
        const safeText = displayText.replace(/</g, '&lt;').replace(/>/g, '&gt;');
        document.execCommand('insertHTML', false, '<a href="' + safeUrl + '">' + safeText + '</a>');
      }
    }
    handleInput();
  }, [restoreSelection, handleInput]);

  const handleInsertImage = useCallback((url, alt) => {
    const el = editorRef.current;
    if (!el) return;
    restoreSelection();
    el.focus();
    // If editor is empty, add a paragraph first
    if (!el.innerHTML || el.innerHTML === '<br>' || el.innerHTML.trim() === '') {
      el.innerHTML = '<p><br></p>';
      const range = document.createRange();
      range.selectNodeContents(el.querySelector('p'));
      range.collapse(true);
      window.getSelection().removeAllRanges();
      window.getSelection().addRange(range);
    }
    const safeUrl = url.replace(/"/g, '&quot;');
    const safeAlt = (alt || '').replace(/"/g, '&quot;');
    const imgHtml = '<img src="' + safeUrl + '" alt="' + safeAlt + '" style="max-width:100%;height:auto;" />';
    document.execCommand('insertHTML', false, imgHtml);
    handleInput();
  }, [restoreSelection, handleInput]);

  const handleInsertTable = useCallback((rows, cols) => {
    restoreSelection();
    editorRef.current?.focus();
    let html = '<table style="border-collapse:collapse;width:100%;">';
    html += '<thead><tr>';
    for (let c = 0; c < cols; c++) {
      html += '<th style="border:1px solid #ccc;padding:8px;background:#f3f4f6;text-align:left;">Header</th>';
    }
    html += '</tr></thead><tbody>';
    for (let r = 1; r < rows; r++) {
      html += '<tr>';
      for (let c = 0; c < cols; c++) {
        html += '<td style="border:1px solid #ccc;padding:8px;">&nbsp;</td>';
      }
      html += '</tr>';
    }
    html += '</tbody></table><p><br></p>';
    document.execCommand('insertHTML', false, html);
    handleInput();
  }, [restoreSelection, handleInput]);

  const handleInsertEquation = useCallback((latex) => {
    restoreSelection();
    editorRef.current?.focus();
    const safeTex = latex.replace(/</g, '&lt;').replace(/>/g, '&gt;');
    let renderedHtml;
    try {
      renderedHtml = katex.renderToString(latex, { throwOnError: false, displayMode: false });
    } catch {
      renderedHtml = safeTex;
    }
    const eqHtml = '<span class="math-tex" contenteditable="false" data-latex="' + safeTex + '" style="display:inline-block;padding:2px 4px;">' + renderedHtml + '</span>&nbsp;';
    document.execCommand('insertHTML', false, eqHtml);
    handleInput();
  }, [restoreSelection, handleInput]);

  const handleInsertMedia = useCallback((parsed) => {
    restoreSelection();
    editorRef.current?.focus();
    let html = '';
    if (parsed.type === 'youtube') {
      html = '<div style="position:relative;padding-bottom:56.25%;height:0;overflow:hidden;max-width:100%;margin:16px 0;"><iframe src="https://www.youtube.com/embed/' + parsed.id + '" style="position:absolute;top:0;left:0;width:100%;height:100%;border:0;" allowfullscreen title="YouTube video"></iframe></div>';
    } else if (parsed.type === 'vimeo') {
      html = '<div style="position:relative;padding-bottom:56.25%;height:0;overflow:hidden;max-width:100%;margin:16px 0;"><iframe src="https://player.vimeo.com/video/' + parsed.id + '" style="position:absolute;top:0;left:0;width:100%;height:100%;border:0;" allowfullscreen title="Vimeo video"></iframe></div>';
    } else if (parsed.type === 'video') {
      const safeUrl = parsed.url.replace(/"/g, '&quot;');
      html = '<video controls style="max-width:100%;margin:16px 0;" src="' + safeUrl + '">Your browser does not support the video tag.</video>';
    } else if (parsed.type === 'audio') {
      const safeUrl = parsed.url.replace(/"/g, '&quot;');
      html = '<audio controls style="width:100%;margin:16px 0;" src="' + safeUrl + '">Your browser does not support the audio tag.</audio>';
    }
    if (html) {
      document.execCommand('insertHTML', false, html + '<p><br></p>');
    }
    handleInput();
  }, [restoreSelection, handleInput]);

  const handleA11yCheck = useCallback(() => {
    const html = editorRef.current?.innerHTML || '';
    const issues = runAccessibilityCheck(html);
    setA11yIssues(issues);
    setA11yOpen(true);
  }, []);

  const insertUploadedImage = useCallback((downloadUrl, altText) => {
    const el = editorRef.current;
    if (!el) return;
    el.focus();
    if (!el.innerHTML || el.innerHTML === '<br>' || el.innerHTML.trim() === '') {
      el.innerHTML = '<p><br></p>';
      const range = document.createRange();
      range.selectNodeContents(el.querySelector('p'));
      range.collapse(true);
      window.getSelection().removeAllRanges();
      window.getSelection().addRange(range);
    }
    const safeUrl = downloadUrl.replace(/"/g, '&quot;');
    const safeAlt = (altText || '').replace(/"/g, '&quot;');
    document.execCommand('insertHTML', false, '<img src="' + safeUrl + '" alt="' + safeAlt + '" style="max-width:100%;height:auto;" />');
    handleInput();
  }, [handleInput]);

  const uploadAndInsertImage = useCallback(async (file) => {
    if (!courseId) return;
    setUploadProgress(0);
    setUploadError(null);
    try {
      const attachment = await api.uploadCourseFileWithProgress(courseId, file, (p) => setUploadProgress(p));
      insertUploadedImage(`/api/v1/files/${attachment.id}/download`, file.name.replace(/\.[^.]+$/, ''));
    } catch (err) {
      setUploadError(err.message || 'Upload failed');
    } finally {
      setUploadProgress(null);
    }
  }, [courseId, insertUploadedImage]);

  const handleDragOver = useCallback((e) => {
    if (!courseId) return;
    const hasImages = Array.from(e.dataTransfer?.types || []).includes('Files');
    if (hasImages) {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'copy';
      setDragOver(true);
    }
  }, [courseId]);

  const handleDragLeave = useCallback((e) => {
    if (e.currentTarget.contains(e.relatedTarget)) return;
    setDragOver(false);
  }, []);

  const handleDrop = useCallback((e) => {
    setDragOver(false);
    if (!courseId) return;
    const files = Array.from(e.dataTransfer?.files || []).filter(f => f.type.startsWith('image/'));
    if (files.length === 0) return;
    e.preventDefault();
    // Place cursor at drop point
    const el = editorRef.current;
    if (el) {
      const caretRange = document.caretRangeFromPoint?.(e.clientX, e.clientY);
      if (caretRange) {
        const sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(caretRange);
      }
    }
    files.forEach(file => uploadAndInsertImage(file));
  }, [courseId, uploadAndInsertImage]);

  const handlePaste = useCallback((e) => {
    if (!courseId) return;
    const items = Array.from(e.clipboardData?.items || []);
    const imageItem = items.find(item => item.type.startsWith('image/'));
    if (!imageItem) return;
    e.preventDefault();
    const file = imageItem.getAsFile();
    if (!file) return;
    const ext = file.type.split('/')[1] || 'png';
    const namedFile = new File([file], `pasted-image-${Date.now()}.${ext}`, { type: file.type });
    uploadAndInsertImage(namedFile);
  }, [courseId, uploadAndInsertImage]);

  const handleToggleSource = useCallback(() => {
    if (sourceView) {
      if (editorRef.current) {
        editorRef.current.innerHTML = sourceHtml;
      }
      onChange?.(sourceHtml);
    } else {
      setSourceHtml(editorRef.current?.innerHTML || '');
    }
    setSourceView((prev) => !prev);
  }, [sourceView, sourceHtml, onChange]);

  const handleSourceChange = useCallback((e) => {
    const html = e.target.value;
    setSourceHtml(html);
    onChange?.(html);
    setCharCount(html.replace(/<[^>]*>/g, '').length);
  }, [onChange]);

  const handleKeyDown = useCallback((e) => {
    const mod = isMac ? e.metaKey : e.ctrlKey;

    if (mod && e.key === 'b') {
      e.preventDefault();
      toggleBold();
    } else if (mod && e.key === 'i') {
      e.preventDefault();
      toggleItalic();
    } else if (mod && e.key === 'u') {
      e.preventDefault();
      toggleUnderline();
    } else if (mod && e.key === 'k') {
      e.preventDefault();
      saveSelection();
      setLinkOpen(true);
    } else if (mod && e.shiftKey && (e.key === 'x' || e.key === 'X')) {
      e.preventDefault();
      toggleStrikethrough();
    } else if (e.key === 'Tab') {
      const inList = document.queryCommandState('insertUnorderedList') || document.queryCommandState('insertOrderedList');
      if (inList) {
        e.preventDefault();
        if (e.shiftKey) {
          exec('outdent');
        } else {
          exec('indent');
        }
      }
    }
  }, [exec, saveSelection, toggleBold, toggleItalic, toggleUnderline, toggleStrikethrough]);

  const closeAllPopovers = useCallback(() => {
    setLinkOpen(false);
    setImageOpen(false);
    setTableOpen(false);
    setEquationOpen(false);
    setMediaOpen(false);
    setA11yOpen(false);
  }, []);

  const openPopover = useCallback((setter) => {
    closeAllPopovers();
    saveSelection();
    setter(true);
  }, [closeAllPopovers, saveSelection]);

  return (
    <div
      className={cx(
        'border border-border-strong rounded-lg overflow-hidden bg-surface-0',
        disabled && 'opacity-60 pointer-events-none',
      )}
    >
      {/* Toolbar */}
      <div
        role="toolbar"
        aria-label="Formatting toolbar"
        className="flex flex-wrap items-center gap-0.5 px-2 py-1.5 bg-surface-1 border-b border-border-default"
      >
        <ToolbarButton icon={Bold} label="Bold" shortcut={modKey + '+B'} active={activeFormats.bold} onClick={toggleBold} disabled={disabled} />
        <ToolbarButton icon={Italic} label="Italic" shortcut={modKey + '+I'} active={activeFormats.italic} onClick={toggleItalic} disabled={disabled} />
        <ToolbarButton icon={Underline} label="Underline" shortcut={modKey + '+U'} active={activeFormats.underline} onClick={toggleUnderline} disabled={disabled} />
        <ToolbarButton icon={Strikethrough} label="Strikethrough" shortcut={modKey + '+Shift+X'} active={activeFormats.strikeThrough} onClick={toggleStrikethrough} disabled={disabled} />

        <Separator />

        <ToolbarButton icon={Heading2} label="Heading 2" active={activeFormats.h2} onClick={toggleH2} disabled={disabled} />
        <ToolbarButton icon={Heading3} label="Heading 3" active={activeFormats.h3} onClick={toggleH3} disabled={disabled} />

        <Separator />

        <ToolbarButton icon={List} label="Bulleted list" active={activeFormats.insertUnorderedList} onClick={toggleBulletList} disabled={disabled} />
        <ToolbarButton icon={ListOrdered} label="Numbered list" active={activeFormats.insertOrderedList} onClick={toggleNumberedList} disabled={disabled} />
        <ToolbarButton icon={Quote} label="Blockquote" active={activeFormats.blockquote} onClick={toggleBlockquote} disabled={disabled} />

        <Separator />

        <ToolbarButton icon={Minus} label="Horizontal rule" onClick={insertHR} disabled={disabled} />
        <ToolbarButton icon={RemoveFormatting} label="Clear formatting" onClick={clearFormatting} disabled={disabled} />

        <Separator />

        <div className="relative">
          <ToolbarButton icon={Link} label="Insert link" shortcut={modKey + '+K'} onClick={() => openPopover(setLinkOpen)} disabled={disabled} />
          <LinkPopover open={linkOpen} onClose={() => setLinkOpen(false)} onInsert={handleInsertLink} />
        </div>
        <div className="relative">
          <ToolbarButton icon={Image} label="Insert image" onClick={() => openPopover(setImageOpen)} disabled={disabled} />
          <ImagePopover open={imageOpen} onClose={() => setImageOpen(false)} onInsert={handleInsertImage} courseId={courseId} />
        </div>
        <div className="relative">
          <ToolbarButton icon={Table} label="Insert table" onClick={() => openPopover(setTableOpen)} disabled={disabled} />
          <TablePopover open={tableOpen} onClose={() => setTableOpen(false)} onInsert={handleInsertTable} />
        </div>
        <div className="relative">
          <ToolbarButton icon={Calculator} label="Insert equation" onClick={() => openPopover(setEquationOpen)} disabled={disabled} />
          <EquationPopover open={equationOpen} onClose={() => setEquationOpen(false)} onInsert={handleInsertEquation} />
        </div>
        <div className="relative">
          <ToolbarButton icon={Play} label="Embed media" onClick={() => openPopover(setMediaOpen)} disabled={disabled} />
          <MediaPopover open={mediaOpen} onClose={() => setMediaOpen(false)} onInsert={handleInsertMedia} />
        </div>

        <Separator />

        <div className="relative">
          <ToolbarButton icon={Shield} label="Accessibility checker" onClick={handleA11yCheck} disabled={disabled} />
          <AccessibilityPanel open={a11yOpen} onClose={() => setA11yOpen(false)} issues={a11yIssues} />
        </div>
        <ToolbarButton
          icon={Code}
          label={sourceView ? 'Visual editor' : 'HTML source'}
          active={sourceView}
          onClick={handleToggleSource}
          disabled={disabled}
        />
      </div>

      {/* Upload progress bar */}
      {uploadProgress !== null && (
        <div className="px-3 py-1.5 bg-brand-50 border-b border-blue-200">
          <div className="flex items-center gap-2">
            <Upload size={14} className="text-brand-600 shrink-0" />
            <div className="flex-1 bg-blue-200 rounded-full h-1.5">
              <div className="bg-brand-600 h-1.5 rounded-full transition-all duration-200" style={{ width: `${Math.round(uploadProgress * 100)}%` }} />
            </div>
            <span className="text-xs text-brand-700">{Math.round(uploadProgress * 100)}%</span>
          </div>
        </div>
      )}

      {/* Upload error banner */}
      {uploadError && (
        <div className="flex items-center gap-2 px-3 py-1.5 bg-accent-danger/10 border-b border-accent-danger/30 text-xs text-accent-danger">
          <AlertTriangle size={14} className="shrink-0" />
          <span className="flex-1">{uploadError}</span>
          <button type="button" onClick={() => setUploadError(null)} className="text-red-400 hover:text-accent-danger"><X size={14} /></button>
        </div>
      )}

      {/* Editor Surface */}
      {sourceView ? (
        <textarea
          className="w-full p-4 font-mono text-sm text-text-primary bg-surface-1 focus:outline-none resize-y"
          style={{ minHeight }}
          value={sourceHtml}
          onChange={handleSourceChange}
          spellCheck={false}
          aria-label="HTML source editor"
        />
      ) : (
        <div
          className="relative"
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
        >
          <div
            ref={editorRef}
            id={id}
            contentEditable={!disabled}
            suppressContentEditableWarning
            role="textbox"
            aria-multiline="true"
            aria-label={ariaLabel}
            className={cx(
              'w-full p-4 text-text-primary focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-400',
              'prose prose-sm max-w-none',
              'empty:before:content-[attr(data-placeholder)] empty:before:text-text-disabled empty:before:pointer-events-none',
            )}
            style={{ minHeight }}
            data-placeholder={placeholder}
            onInput={handleInput}
            onKeyDown={handleKeyDown}
            onPaste={handlePaste}
            onFocus={updateActiveFormats}
            onMouseUp={updateActiveFormats}
          />
          {dragOver && (
            <div className="absolute inset-0 bg-brand-50/70 border-2 border-dashed border-blue-400 rounded flex items-center justify-center pointer-events-none z-10">
              <div className="flex items-center gap-2 text-brand-700 font-medium text-sm">
                <Image size={20} />
                Drop image here to upload
              </div>
            </div>
          )}
        </div>
      )}

      {/* Bottom Bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-surface-1 border-t border-border-default text-xs text-text-tertiary">
        <span>{charCount} character{charCount !== 1 ? 's' : ''}</span>
        <span className="text-text-disabled">
          {sourceView ? 'HTML source mode' : 'Visual editor'}
        </span>
      </div>
    </div>
  );
}
