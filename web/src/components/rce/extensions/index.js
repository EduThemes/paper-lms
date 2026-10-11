import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Placeholder from '@tiptap/extension-placeholder';
import TableRow from '@tiptap/extension-table-row';
import TableCell from '@tiptap/extension-table-cell';
import TaskList from '@tiptap/extension-task-list';
import TaskItem from '@tiptap/extension-task-item';
import CharacterCount from '@tiptap/extension-character-count';
import Typography from '@tiptap/extension-typography';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Highlight from '@tiptap/extension-highlight';
import Iframe from './Iframe';
import Media from './Media';
import MathInline from './MathInline';
import PreservedAttributes from './PreservedAttributes';
import { TextStyle, Subscript, Superscript, InlineTag } from './InlineFormatting';
import { Container, Details, DetailsSummary, RawHtmlBlock } from './BlockNodes';
import { RceTable, RceTableHeader } from './RceTable';
import RceImage from './RceImage';

/**
 * The single extension list for RichContentEditorV2 (and its tests), so the
 * schema the tests exercise is the one teachers edit with.
 *
 * @param {{placeholder?: string}} opts
 */
export function buildRceExtensions({ placeholder = '' } = {}) {
  return [
    StarterKit.configure({
      // StarterKit v3 already includes Underline + Link; configured below.
      link: false,
      underline: false,
      codeBlock: { HTMLAttributes: { class: 'rce-code-block' } },
    }),
    Underline,
    Link.configure({
      openOnClick: false,
      autolink: true,
      defaultProtocol: 'https',
      // Opening in a new tab is a per-link choice (link dialog), not forced
      // on every link (G014). Existing target="_blank" is kept on parse.
      HTMLAttributes: { rel: 'noopener noreferrer', target: null },
    }),
    RceImage.configure({
      inline: false,
      allowBase64: false,
      HTMLAttributes: { class: 'rce-image' },
    }),
    Placeholder.configure({ placeholder }),
    RceTable.configure({ resizable: true }),
    TableRow, RceTableHeader, TableCell,
    TaskList,
    TaskItem.configure({ nested: true }),
    CharacterCount,
    Typography,
    TextAlign.configure({ types: ['heading', 'paragraph'] }),
    Highlight.configure({ multicolor: true }),
    TextStyle,
    Subscript,
    Superscript,
    InlineTag,
    Container,
    Details,
    DetailsSummary,
    RawHtmlBlock,
    Iframe,
    Media,
    MathInline,
    PreservedAttributes,
  ];
}

export default buildRceExtensions;
