import React from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription,
} from '../ui/dialog';

const isMac = typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform || '');
const MOD = isMac ? 'Cmd' : 'Ctrl';
const ALT = isMac ? 'Option' : 'Alt';

/** Shortcut list shown by the dialog (also used by tests). */
export const RCE_SHORTCUTS = [
  { keys: `${ALT}+0`, action: 'Open this list of keyboard shortcuts' },
  { keys: `${ALT}+F10`, action: 'Move focus to the editor toolbar (arrow keys move between buttons)' },
  { keys: `${ALT}+F8`, action: 'Open the accessibility checker' },
  { keys: `${MOD}+F`, action: 'Find and replace' },
  { keys: `${MOD}+Shift+F`, action: 'Toggle full screen (Escape leaves full screen)' },
  { keys: `${MOD}+B`, action: 'Bold' },
  { keys: `${MOD}+I`, action: 'Italic' },
  { keys: `${MOD}+U`, action: 'Underline' },
  { keys: `${MOD}+K`, action: 'Insert or edit a link' },
  { keys: `${MOD}+,`, action: 'Subscript' },
  { keys: `${MOD}+.`, action: 'Superscript' },
  { keys: `${MOD}+Alt+1 … 6`, action: 'Heading 1 to 6' },
  { keys: `${MOD}+Alt+0`, action: 'Paragraph' },
  { keys: `${MOD}+Shift+7 / 8`, action: 'Numbered / bulleted list' },
  { keys: 'Tab / Shift+Tab', action: 'Indent / outdent a list item' },
  { keys: `${MOD}+Z / ${MOD}+Shift+Z`, action: 'Undo / redo' },
];

export default function KeyboardShortcutsDialog({ open, onClose }) {
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <DialogDescription>Shortcuts available while editing rich content.</DialogDescription>
        </DialogHeader>
        <table className="w-full text-sm">
          <caption className="sr-only">Editor keyboard shortcuts</caption>
          <thead>
            <tr className="text-left text-xs text-muted-foreground">
              <th scope="col" className="py-1 pr-4">Keys</th>
              <th scope="col" className="py-1">Action</th>
            </tr>
          </thead>
          <tbody>
            {RCE_SHORTCUTS.map((s) => (
              <tr key={s.keys} className="border-t">
                <td className="py-1 pr-4"><kbd className="rounded border bg-muted px-1.5 py-0.5 text-xs">{s.keys}</kbd></td>
                <td className="py-1">{s.action}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </DialogContent>
    </Dialog>
  );
}
