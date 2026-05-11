import React from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '../ui/dialog';

const RestoreAnswersDialog = ({ open, count, savedAt, onRestore, onDiscard }) => {
  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onDiscard(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Restore unsaved answers?</DialogTitle>
          <DialogDescription>
            We found {count} answer{count === 1 ? '' : 's'} saved locally on this device
            {savedAt ? ` from ${savedAt.toLocaleString()}` : ''}.
            Would you like to restore them?
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <button
            type="button"
            onClick={onDiscard}
            className="px-4 py-2 text-sm font-medium rounded border border-border-strong hover:bg-surface-1 focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            Discard
          </button>
          <button
            type="button"
            onClick={onRestore}
            className="px-4 py-2 text-sm font-medium rounded bg-brand-600 text-white hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            Restore
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default RestoreAnswersDialog;
