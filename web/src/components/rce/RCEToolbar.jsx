import React, { useRef, useState, useCallback } from 'react';
import {
  Bold, Italic, Underline as UnderlineIcon, Strikethrough, Code,
  Heading1, Heading2, Heading3, ChevronDown,
  List, ListOrdered, ListChecks,
  Quote, Code2, Minus,
  Link as LinkIcon, Image as ImageIcon, Table as TableIcon,
  AlignLeft, AlignCenter, AlignRight, AlignJustify,
  Undo, Redo, Sigma, Film, FolderSearch,
  Baseline, Highlighter, Subscript as SubIcon, Superscript as SupIcon,
  Indent as IndentIcon, Outdent as OutdentIcon, RemoveFormatting,
  Search, TableProperties, ListCollapse, Pencil,
} from 'lucide-react';
import { Button } from '../ui/button';
import { Popover, PopoverTrigger, PopoverContent } from '../ui/popover';
import { Input } from '../ui/input';
import ContentPicker from './ContentPicker';
import MathInputDialog from './MathInputDialog';
import EmbedMediaDialog from './EmbedMediaDialog';
import { FONT_SIZES } from './extensions/InlineFormatting';

/**
 * @typedef {Object} RCEToolbarProps
 * @property {import('@tiptap/react').Editor} editor
 * @property {(string|number)=} courseId
 * @property {boolean=} disabled - HTML source view is active
 * @property {() => void=} onInsertImage
 * @property {() => void=} onEditImage
 * @property {() => void=} onLink
 * @property {() => void=} onFindReplace
 */

/** Text and background colours offered by the colour pickers (Canvas-like palette). */
export const RCE_COLORS = [
  '#000000', '#434343', '#666666', '#999999', '#d9d9d9', '#ffffff',
  '#c0392b', '#e03e2d', '#e67e23', '#f1c40f', '#2dc26b', '#169179',
  '#3598db', '#236fa1', '#843fa1', '#b96ad9', '#ecf0f1', '#fbeeb8',
  '#bfedd2', '#c2e0f4', '#eccafa', '#f8cac6',
];

const ToolbarBtn = React.forwardRef(function ToolbarBtn(
  { active, label, onClick, children, disabled, ...rest },
  ref
) {
  return (
    <Button
      ref={ref}
      type="button"
      variant="ghost"
      size="icon"
      aria-label={label}
      aria-pressed={active === undefined ? undefined : (active ? 'true' : 'false')}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={`h-8 w-8 ${active ? 'bg-accent text-accent-foreground' : ''}`}
      {...rest}
    >
      {children}
    </Button>
  );
});

function Sep() {
  return <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />;
}

function ColorButton({ label, icon, current, onPick, onClear, disabled }) {
  const [open, setOpen] = useState(false);
  const [custom, setCustom] = useState('#000000');
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="icon" aria-label={label} title={label} disabled={disabled}
          aria-haspopup="dialog" className="relative h-8 w-8">
          {icon}
          <span aria-hidden="true" className="absolute bottom-1 left-2 right-2 h-0.5 rounded"
            style={{ backgroundColor: current || 'transparent' }} />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-60 p-2" align="start">
        <p className="mb-1 text-xs font-medium">{label}</p>
        <div className="grid grid-cols-6 gap-1" role="group" aria-label={`${label} swatches`}>
          {RCE_COLORS.map((c) => (
            <button key={c} type="button" aria-label={c} title={c}
              onClick={() => { onPick(c); setOpen(false); }}
              className={`h-7 w-7 rounded border ${current === c ? 'ring-2 ring-ring' : ''}`}
              style={{ backgroundColor: c }} />
          ))}
        </div>
        <div className="mt-2 flex items-center gap-2">
          <input type="color" aria-label={`Custom ${label.toLowerCase()}`} value={custom}
            onChange={(e) => setCustom(e.target.value)} className="h-7 w-10 cursor-pointer rounded border" />
          <Button type="button" size="sm" variant="outline" className="h-7 px-2 text-xs"
            onClick={() => { onPick(custom); setOpen(false); }}>Apply</Button>
          <Button type="button" size="sm" variant="ghost" className="h-7 px-2 text-xs"
            onClick={() => { onClear(); setOpen(false); }}>Remove</Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}

function TableOptions({ editor, disabled }) {
  const [open, setOpen] = useState(false);
  const [caption, setCaption] = useState('');
  const inTable = editor.isActive('table');
  const run = (fn) => () => { fn(editor.chain().focus()).run(); };
  return (
    <Popover open={open} onOpenChange={(o) => {
      setOpen(o);
      if (o) setCaption(editor.getAttributes('table').caption || '');
    }}>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="icon" className="h-8 w-8" aria-label="Table options"
          title="Table options" disabled={disabled || !inTable} aria-haspopup="dialog">
          <TableProperties />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-72 p-3" align="start">
        <form className="space-y-1" onSubmit={(e) => { e.preventDefault(); editor.chain().focus().setTableCaption(caption).run(); }}>
          <label htmlFor="rce-table-caption" className="text-xs font-medium">Caption (describes the table)</label>
          <div className="flex gap-1">
            <Input id="rce-table-caption" className="h-8" value={caption} onChange={(e) => setCaption(e.target.value)} />
            <Button type="submit" size="sm" className="h-8">Set</Button>
          </div>
        </form>
        <div className="mt-2 grid grid-cols-2 gap-1 text-xs">
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.addRowBefore())}>Row above</Button>
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.addRowAfter())}>Row below</Button>
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.addColumnBefore())}>Column left</Button>
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.addColumnAfter())}>Column right</Button>
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.deleteRow())}>Delete row</Button>
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.deleteColumn())}>Delete column</Button>
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.toggleHeaderRow().fixTableHeaderScopes())}>Header row</Button>
          <Button type="button" size="sm" variant="outline" className="h-7 text-xs" onClick={run((c) => c.toggleHeaderColumn().fixTableHeaderScopes())}>Header column</Button>
          <Button type="button" size="sm" variant="outline" className="col-span-2 h-7 text-xs" onClick={() => { editor.chain().focus().deleteTable().run(); setOpen(false); }}>Delete table</Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}

/**
 * Roving-tabindex toolbar for RichContentEditorV2.
 * @param {RCEToolbarProps} props
 */
export default function RCEToolbar({
  editor, courseId, disabled = false, onInsertImage, onEditImage, onLink, onFindReplace,
}) {
  const [showHeadingMenu, setShowHeadingMenu] = useState(false);
  const [showPicker, setShowPicker] = useState(false);
  const [showMath, setShowMath] = useState(false);
  const [showEmbed, setShowEmbed] = useState(false);
  const toolbarRef = useRef(null);

  /** Arrow-key navigation across toolbar controls (a11y). */
  const onKeyDown = useCallback((e) => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
    if (e.target.tagName === 'SELECT' || e.target.tagName === 'INPUT') return;
    const buttons = Array.from(
      toolbarRef.current?.querySelectorAll('button:not([disabled]), select:not([disabled])') ?? []
    );
    const idx = buttons.indexOf(document.activeElement);
    if (idx === -1) return;
    e.preventDefault();
    let nextIdx = idx;
    if (e.key === 'ArrowRight') nextIdx = (idx + 1) % buttons.length;
    else if (e.key === 'ArrowLeft') nextIdx = (idx - 1 + buttons.length) % buttons.length;
    else if (e.key === 'Home') nextIdx = 0;
    else if (e.key === 'End') nextIdx = buttons.length - 1;
    buttons[nextIdx]?.focus();
  }, []);

  const insertTable = useCallback(() => {
    editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).fixTableHeaderScopes().run();
  }, [editor]);

  const insertMath = useCallback((latex) => {
    if (!latex) return;
    // Insert as a text node, never as an HTML string: building
    // `\(${latex}\)` into markup let "<" / "&" in the LaTeX (x<y, matrix
    // column separators) be parsed as tags/entities and corrupt the equation
    // (B101). The viewers render \( \) text with KaTeX.
    editor.chain().focus().insertContent({ type: 'text', text: `\\(${latex}\\)` }).run();
    setShowMath(false);
  }, [editor]);

  const insertEmbed = useCallback((attrs) => {
    if (!attrs?.src) return;
    editor.chain().focus().setIframe(attrs).run();
    setShowEmbed(false);
  }, [editor]);

  const insertContentLink = useCallback((href, label) => {
    if (!href) return;
    // Text node + link mark: the label is a title and must not be parsed as
    // HTML ("Grades <Unit 2>" used to lose "<Unit 2>") (B106).
    editor.chain().focus().insertContent({
      type: 'text',
      text: label || href,
      marks: [{ type: 'link', attrs: { href } }],
    }).run();
    setShowPicker(false);
  }, [editor]);

  const clearFormatting = useCallback(() => {
    // Remove inline formatting but keep links (as Canvas's "clear formatting" does).
    const chain = editor.chain().focus();
    Object.keys(editor.schema.marks).filter((m) => m !== 'link').forEach((m) => chain.unsetMark(m, { extendEmptyMarkRange: true }));
    chain.unsetTextAlign().run();
  }, [editor]);

  const textStyle = editor.getAttributes('textStyle');
  const imageSelected = editor.isActive('image');
  const off = disabled;

  return (
    <>
      <div
        ref={toolbarRef}
        role="toolbar"
        aria-label="Rich text formatting"
        aria-disabled={off ? 'true' : undefined}
        onKeyDown={onKeyDown}
        className="rce-toolbar flex flex-wrap items-center gap-0.5 border-b border-input bg-muted/30 p-1"
      >
        {/* Inline formatting */}
        <ToolbarBtn disabled={off} label="Bold (Ctrl+B)" active={editor.isActive('bold')} onClick={() => editor.chain().focus().toggleBold().run()}><Bold /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Italic" active={editor.isActive('italic')} onClick={() => editor.chain().focus().toggleItalic().run()}><Italic /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Underline" active={editor.isActive('underline')} onClick={() => editor.chain().focus().toggleUnderline().run()}><UnderlineIcon /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Strikethrough" active={editor.isActive('strike')} onClick={() => editor.chain().focus().toggleStrike().run()}><Strikethrough /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Subscript" active={editor.isActive('subscript')} onClick={() => editor.chain().focus().toggleSubscript().run()}><SubIcon /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Superscript" active={editor.isActive('superscript')} onClick={() => editor.chain().focus().toggleSuperscript().run()}><SupIcon /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Inline code" active={editor.isActive('code')} onClick={() => editor.chain().focus().toggleCode().run()}><Code /></ToolbarBtn>

        <Sep />

        {/* Font size + colours */}
        <select
          aria-label="Font size"
          title="Font size"
          disabled={off}
          value={textStyle.fontSize || ''}
          onChange={(e) => {
            const v = e.target.value;
            if (v) editor.chain().focus().setFontSize(v).run();
            else editor.chain().focus().unsetFontSize().run();
          }}
          className="h-8 rounded-md border border-input bg-background px-1 text-xs"
        >
          <option value="">Size</option>
          {FONT_SIZES.map((s) => <option key={s} value={s}>{s}</option>)}
          {textStyle.fontSize && !FONT_SIZES.includes(textStyle.fontSize) && (
            <option value={textStyle.fontSize}>{textStyle.fontSize}</option>
          )}
        </select>
        <ColorButton disabled={off} label="Text colour" icon={<Baseline />} current={textStyle.color}
          onPick={(c) => editor.chain().focus().setColor(c).run()}
          onClear={() => editor.chain().focus().unsetColor().run()} />
        <ColorButton disabled={off} label="Background colour" icon={<Highlighter />} current={textStyle.backgroundColor}
          onPick={(c) => editor.chain().focus().setBackgroundColor(c).run()}
          onClear={() => editor.chain().focus().unsetBackgroundColor().unsetHighlight().run()} />
        <ToolbarBtn disabled={off} label="Clear formatting" onClick={clearFormatting}><RemoveFormatting /></ToolbarBtn>

        <Sep />

        {/* Heading group */}
        <div className="relative">
          <Button type="button" variant="ghost" size="sm" className="h-8 px-2" disabled={off} aria-label="Block format" aria-haspopup="menu" aria-expanded={showHeadingMenu} onClick={() => setShowHeadingMenu((s) => !s)}>
            {editor.isActive('heading', { level: 1 }) ? <Heading1 /> : editor.isActive('heading', { level: 2 }) ? <Heading2 /> : editor.isActive('heading', { level: 3 }) ? <Heading3 /> : <Heading2 />}
            <ChevronDown className="!size-3" />
          </Button>
          {showHeadingMenu && (
            <div role="menu" className="absolute left-0 top-full z-50 mt-1 flex flex-col rounded-md border bg-popover p-1 shadow-md">
              {[2, 3, 4, 5, 6].map((lvl) => (
                <button key={lvl} role="menuitem" type="button"
                  onClick={() => { editor.chain().focus().toggleHeading({ level: lvl }).run(); setShowHeadingMenu(false); }}
                  className={`px-3 py-1 text-left text-sm rounded hover:bg-accent ${editor.isActive('heading', { level: lvl }) ? 'bg-accent' : ''}`}>
                  Heading {lvl}
                </button>
              ))}
              <button role="menuitem" type="button"
                onClick={() => { editor.chain().focus().setParagraph().run(); setShowHeadingMenu(false); }}
                className="px-3 py-1 text-left text-sm rounded hover:bg-accent">Paragraph</button>
            </div>
          )}
        </div>

        <Sep />

        {/* Lists + indentation */}
        <ToolbarBtn disabled={off} label="Bullet list" active={editor.isActive('bulletList')} onClick={() => editor.chain().focus().toggleBulletList().run()}><List /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Ordered list" active={editor.isActive('orderedList')} onClick={() => editor.chain().focus().toggleOrderedList().run()}><ListOrdered /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Task list" active={editor.isActive('taskList')} onClick={() => editor.chain().focus().toggleTaskList().run()}><ListChecks /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Increase indent" onClick={() => editor.chain().focus().indent().run()}><IndentIcon /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Decrease indent" onClick={() => editor.chain().focus().outdent().run()}><OutdentIcon /></ToolbarBtn>

        <Sep />

        {/* Blocks */}
        <ToolbarBtn disabled={off} label="Quote" active={editor.isActive('blockquote')} onClick={() => editor.chain().focus().toggleBlockquote().run()}><Quote /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Code block" active={editor.isActive('codeBlock')} onClick={() => editor.chain().focus().toggleCodeBlock().run()}><Code2 /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Horizontal rule" onClick={() => editor.chain().focus().setHorizontalRule().run()}><Minus /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Collapsible section" onClick={() => editor.chain().focus().insertDetails().run()}><ListCollapse /></ToolbarBtn>

        <Sep />

        {/* Insert */}
        <ToolbarBtn disabled={off} label="Insert link (Ctrl+K)" active={editor.isActive('link')} onClick={() => onLink?.()}><LinkIcon /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Insert image" onClick={() => onInsertImage?.()}><ImageIcon /></ToolbarBtn>
        <ToolbarBtn disabled={off || !imageSelected} label="Image options (alt text, size)" onClick={() => onEditImage?.()}><Pencil /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Insert table" onClick={insertTable}><TableIcon /></ToolbarBtn>
        <TableOptions editor={editor} disabled={off} />
        <ToolbarBtn disabled={off} label="Insert math equation" onClick={() => setShowMath(true)}><Sigma /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Embed media" onClick={() => setShowEmbed(true)}><Film /></ToolbarBtn>
        {courseId != null && (
          <ToolbarBtn disabled={off} label="Course content picker" onClick={() => setShowPicker(true)}><FolderSearch /></ToolbarBtn>
        )}

        <Sep />

        {/* Alignment + direction */}
        <ToolbarBtn disabled={off} label="Align left" active={editor.isActive({ textAlign: 'left' })} onClick={() => editor.chain().focus().setTextAlign('left').run()}><AlignLeft /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Align center" active={editor.isActive({ textAlign: 'center' })} onClick={() => editor.chain().focus().setTextAlign('center').run()}><AlignCenter /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Align right" active={editor.isActive({ textAlign: 'right' })} onClick={() => editor.chain().focus().setTextAlign('right').run()}><AlignRight /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Align justify" active={editor.isActive({ textAlign: 'justify' })} onClick={() => editor.chain().focus().setTextAlign('justify').run()}><AlignJustify /></ToolbarBtn>
        <ToolbarBtn disabled={off} label="Left-to-right text" active={editor.isActive({ dir: 'ltr' })}
          onClick={() => (editor.isActive({ dir: 'ltr' }) ? editor.chain().focus().unsetTextDirection().run() : editor.chain().focus().setTextDirection('ltr').run())}>
          <span className="text-[10px] font-semibold">LTR</span>
        </ToolbarBtn>
        <ToolbarBtn disabled={off} label="Right-to-left text" active={editor.isActive({ dir: 'rtl' })}
          onClick={() => (editor.isActive({ dir: 'rtl' }) ? editor.chain().focus().unsetTextDirection().run() : editor.chain().focus().setTextDirection('rtl').run())}>
          <span className="text-[10px] font-semibold">RTL</span>
        </ToolbarBtn>

        <Sep />

        {/* Find + history */}
        <ToolbarBtn disabled={off} label="Find and replace (Ctrl+F)" onClick={() => onFindReplace?.()}><Search /></ToolbarBtn>
        <ToolbarBtn label="Undo" disabled={off || !editor.can().undo()} onClick={() => editor.chain().focus().undo().run()}><Undo /></ToolbarBtn>
        <ToolbarBtn label="Redo" disabled={off || !editor.can().redo()} onClick={() => editor.chain().focus().redo().run()}><Redo /></ToolbarBtn>
      </div>

      {showPicker && courseId != null && (
        <ContentPicker
          courseId={courseId}
          onInsert={insertContentLink}
          onClose={() => setShowPicker(false)}
        />
      )}
      <MathInputDialog open={showMath} onClose={() => setShowMath(false)} onInsert={insertMath} />
      <EmbedMediaDialog open={showEmbed} onClose={() => setShowEmbed(false)} onInsert={insertEmbed} />
    </>
  );
}
