import React, { useRef, useState, useCallback } from 'react';
import {
  Bold, Italic, Underline as UnderlineIcon, Strikethrough, Code,
  Heading1, Heading2, Heading3, ChevronDown,
  List, ListOrdered, ListChecks,
  Quote, Code2, Minus,
  Link as LinkIcon, Image as ImageIcon, Table as TableIcon,
  AlignLeft, AlignCenter, AlignRight, AlignJustify,
  Undo, Redo, Sigma, Film, FolderSearch,
} from 'lucide-react';
import { Button } from '../ui/button';
import { sanitizeHTML } from '../RichContentViewer';
import ContentPicker from './ContentPicker';
import MathInputDialog from './MathInputDialog';
import EmbedMediaDialog from './EmbedMediaDialog';

/**
 * @typedef {Object} RCEToolbarProps
 * @property {import('@tiptap/react').Editor} editor
 * @property {(string|number)=} courseId
 */

const ToolbarBtn = React.forwardRef(function ToolbarBtn(
  { active, label, onClick, children, disabled },
  ref
) {
  return (
    <Button
      ref={ref}
      type="button"
      variant="ghost"
      size="icon"
      aria-label={label}
      aria-pressed={active ? 'true' : 'false'}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={`h-8 w-8 ${active ? 'bg-accent text-accent-foreground' : ''}`}
    >
      {children}
    </Button>
  );
});

/**
 * Roving-tabindex toolbar for RichContentEditorV2.
 * @param {RCEToolbarProps} props
 */
export default function RCEToolbar({ editor, courseId }) {
  const [showHeadingMenu, setShowHeadingMenu] = useState(false);
  const [showPicker, setShowPicker] = useState(false);
  const [showMath, setShowMath] = useState(false);
  const [showEmbed, setShowEmbed] = useState(false);
  const toolbarRef = useRef(null);

  /** Arrow-key navigation across toolbar buttons (a11y). */
  const onKeyDown = useCallback((e) => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
    const buttons = Array.from(
      toolbarRef.current?.querySelectorAll('button:not([disabled])') ?? []
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

  const insertImage = useCallback(() => {
    const url = window.prompt('Image URL');
    if (!url) return;
    let alt = window.prompt('Alt text (required for accessibility):') || '';
    if (!alt.trim()) {
      const ok = window.confirm('Inserting an image without alt text harms screen-reader users. Insert anyway?');
      if (!ok) return;
      alt = '';
    }
    editor.chain().focus().setImage({ src: url, alt }).run();
  }, [editor]);

  const insertLink = useCallback(() => {
    const prev = editor.getAttributes('link').href || '';
    const url = window.prompt('Link URL', prev);
    if (url === null) return;
    if (url === '') {
      editor.chain().focus().extendMarkRange('link').unsetLink().run();
      return;
    }
    editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run();
  }, [editor]);

  const insertTable = useCallback(() => {
    editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
  }, [editor]);

  const insertMath = useCallback((latex) => {
    if (!latex) return;
    const html = `<span class="math-tex" data-latex="${latex.replace(/"/g, '&quot;')}">\\(${latex}\\)</span>`;
    editor.chain().focus().insertContent(sanitizeHTML(html)).run();
    setShowMath(false);
  }, [editor]);

  const insertEmbed = useCallback((iframeHTML) => {
    if (!iframeHTML) return;
    editor.chain().focus().insertContent(sanitizeHTML(iframeHTML)).run();
    setShowEmbed(false);
  }, [editor]);

  const insertContentLink = useCallback((href, label) => {
    if (!href) return;
    editor.chain().focus().insertContent(`<a href="${href}">${label || href}</a>`).run();
    setShowPicker(false);
  }, [editor]);

  return (
    <>
      <div
        ref={toolbarRef}
        role="toolbar"
        aria-label="Rich text formatting"
        onKeyDown={onKeyDown}
        className="flex flex-wrap items-center gap-0.5 border-b border-input bg-muted/30 p-1"
      >
        {/* Inline formatting */}
        <ToolbarBtn label="Bold (Ctrl+B)" active={editor.isActive('bold')} onClick={() => editor.chain().focus().toggleBold().run()}><Bold /></ToolbarBtn>
        <ToolbarBtn label="Italic" active={editor.isActive('italic')} onClick={() => editor.chain().focus().toggleItalic().run()}><Italic /></ToolbarBtn>
        <ToolbarBtn label="Underline" active={editor.isActive('underline')} onClick={() => editor.chain().focus().toggleUnderline().run()}><UnderlineIcon /></ToolbarBtn>
        <ToolbarBtn label="Strikethrough" active={editor.isActive('strike')} onClick={() => editor.chain().focus().toggleStrike().run()}><Strikethrough /></ToolbarBtn>
        <ToolbarBtn label="Inline code" active={editor.isActive('code')} onClick={() => editor.chain().focus().toggleCode().run()}><Code /></ToolbarBtn>

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        {/* Heading group */}
        <div className="relative">
          <Button type="button" variant="ghost" size="sm" className="h-8 px-2" aria-haspopup="menu" aria-expanded={showHeadingMenu} onClick={() => setShowHeadingMenu((s) => !s)}>
            {editor.isActive('heading', { level: 1 }) ? <Heading1 /> : editor.isActive('heading', { level: 2 }) ? <Heading2 /> : editor.isActive('heading', { level: 3 }) ? <Heading3 /> : <Heading2 />}
            <ChevronDown className="!size-3" />
          </Button>
          {showHeadingMenu && (
            <div role="menu" className="absolute left-0 top-full z-50 mt-1 flex flex-col rounded-md border bg-popover p-1 shadow-md">
              {[1, 2, 3].map((lvl) => (
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

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        {/* Lists */}
        <ToolbarBtn label="Bullet list" active={editor.isActive('bulletList')} onClick={() => editor.chain().focus().toggleBulletList().run()}><List /></ToolbarBtn>
        <ToolbarBtn label="Ordered list" active={editor.isActive('orderedList')} onClick={() => editor.chain().focus().toggleOrderedList().run()}><ListOrdered /></ToolbarBtn>
        <ToolbarBtn label="Task list" active={editor.isActive('taskList')} onClick={() => editor.chain().focus().toggleTaskList().run()}><ListChecks /></ToolbarBtn>

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        {/* Blocks */}
        <ToolbarBtn label="Quote" active={editor.isActive('blockquote')} onClick={() => editor.chain().focus().toggleBlockquote().run()}><Quote /></ToolbarBtn>
        <ToolbarBtn label="Code block" active={editor.isActive('codeBlock')} onClick={() => editor.chain().focus().toggleCodeBlock().run()}><Code2 /></ToolbarBtn>
        <ToolbarBtn label="Horizontal rule" onClick={() => editor.chain().focus().setHorizontalRule().run()}><Minus /></ToolbarBtn>

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        {/* Insert */}
        <ToolbarBtn label="Insert link" active={editor.isActive('link')} onClick={insertLink}><LinkIcon /></ToolbarBtn>
        <ToolbarBtn label="Insert image" onClick={insertImage}><ImageIcon /></ToolbarBtn>
        <ToolbarBtn label="Insert table" onClick={insertTable}><TableIcon /></ToolbarBtn>
        <ToolbarBtn label="Insert math equation" onClick={() => setShowMath(true)}><Sigma /></ToolbarBtn>
        <ToolbarBtn label="Embed media" onClick={() => setShowEmbed(true)}><Film /></ToolbarBtn>
        {courseId != null && (
          <ToolbarBtn label="Course content picker" onClick={() => setShowPicker(true)}><FolderSearch /></ToolbarBtn>
        )}

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        {/* Alignment */}
        <ToolbarBtn label="Align left" active={editor.isActive({ textAlign: 'left' })} onClick={() => editor.chain().focus().setTextAlign('left').run()}><AlignLeft /></ToolbarBtn>
        <ToolbarBtn label="Align center" active={editor.isActive({ textAlign: 'center' })} onClick={() => editor.chain().focus().setTextAlign('center').run()}><AlignCenter /></ToolbarBtn>
        <ToolbarBtn label="Align right" active={editor.isActive({ textAlign: 'right' })} onClick={() => editor.chain().focus().setTextAlign('right').run()}><AlignRight /></ToolbarBtn>
        <ToolbarBtn label="Align justify" active={editor.isActive({ textAlign: 'justify' })} onClick={() => editor.chain().focus().setTextAlign('justify').run()}><AlignJustify /></ToolbarBtn>

        <span className="mx-1 h-5 w-px bg-border" aria-hidden="true" />

        {/* History */}
        <ToolbarBtn label="Undo" disabled={!editor.can().undo()} onClick={() => editor.chain().focus().undo().run()}><Undo /></ToolbarBtn>
        <ToolbarBtn label="Redo" disabled={!editor.can().redo()} onClick={() => editor.chain().focus().redo().run()}><Redo /></ToolbarBtn>
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
