import Heading from '@tiptap/extension-heading';
import { textblockTypeInputRule } from '@tiptap/react';

// Heading 1 is the page title (the course or page header Paper renders
// around the content), so, as in Canvas, the editor offers Heading 2 to 6
// only. Every level still parses, so existing or pasted <h1> content is
// kept as written and the accessibility checker offers the fix; the
// shortcut and Markdown input rule just never create one.
export const CONTENT_HEADING_LEVELS = [2, 3, 4, 5, 6];

const ContentHeading = Heading.extend({
  addKeyboardShortcuts() {
    return Object.fromEntries(CONTENT_HEADING_LEVELS.map((level) => [
      `Mod-Alt-${level}`, () => this.editor.commands.toggleHeading({ level }),
    ]));
  },
  addInputRules() {
    // "## " makes Heading 2 … "###### " makes Heading 6; "# " stays text.
    return CONTENT_HEADING_LEVELS.map((level) => textblockTypeInputRule({
      find: new RegExp(`^(#{${level}})\\s$`),
      type: this.type,
      getAttributes: { level },
    }));
  },
});

export default ContentHeading;
