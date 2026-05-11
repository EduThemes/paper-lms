import React from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '../ui/dialog';

const Row = ({ keys, label }) => (
  <div className="flex items-center justify-between py-2 border-b border-border-subtle last:border-0">
    <span className="text-sm text-text-secondary">{label}</span>
    <span className="flex items-center gap-1">
      {keys.map((k) => (
        <kbd
          key={k}
          className="px-2 py-0.5 text-xs font-mono bg-surface-2 border border-border-strong rounded shadow-sm"
        >
          {k}
        </kbd>
      ))}
    </span>
  </div>
);

const ShortcutsDialog = ({ open, onOpenChange }) => (
  <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Keyboard shortcuts</DialogTitle>
        <DialogDescription>Available while taking the quiz.</DialogDescription>
      </DialogHeader>
      <div className="mt-2">
        <Row keys={['j']} label="Next question" />
        <Row keys={['k']} label="Previous question" />
        <Row keys={['1', '–', '9']} label="Jump to question 1–9" />
        <Row keys={['?']} label="Show this help" />
      </div>
    </DialogContent>
  </Dialog>
);

export default ShortcutsDialog;
