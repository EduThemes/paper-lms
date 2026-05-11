import React from 'react';
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '../ui/dialog';
import { Button } from '../ui/button';

/**
 * @typedef {Object} RestoreAutosaveModalProps
 * @property {boolean} open
 * @property {number=} savedAt - Timestamp (ms) when the draft was saved
 * @property {() => void} onRestore
 * @property {() => void} onDiscard
 */

/** Format a timestamp as a human "X ago" string. */
function timeAgo(ts) {
  if (!ts) return 'a moment ago';
  const seconds = Math.max(1, Math.floor((Date.now() - ts) / 1000));
  if (seconds < 60) return `${seconds} second${seconds === 1 ? '' : 's'} ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`;
  const days = Math.floor(hours / 24);
  return `${days} day${days === 1 ? '' : 's'} ago`;
}

/**
 * Prompts the user to restore (or discard) an autosaved RCE draft.
 * @param {RestoreAutosaveModalProps} props
 */
export default function RestoreAutosaveModal({ open, savedAt, onRestore, onDiscard }) {
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onDiscard(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Unsaved draft found</DialogTitle>
          <DialogDescription>
            We saved a draft from {timeAgo(savedAt)}. Restore it, or discard and continue with the
            current content?
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onDiscard} type="button">Discard</Button>
          <Button onClick={onRestore} type="button">Restore draft</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
