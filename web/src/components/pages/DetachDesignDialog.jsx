import PropTypes from 'prop-types';
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '../ui/dialog';
import { Button } from '../ui/button';

// Confirms "Edit as HTML" on a designed page. Saving HTML by hand detaches
// the page from its design (wiki_page[design] = null): it keeps its look,
// but the designer can no longer open it. Page history can restore the
// design, since every revision snapshots it.
export default function DetachDesignDialog({ open, onOpenChange, onConfirm }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit this page as HTML?</DialogTitle>
          <DialogDescription>
            This page was made with the page designer. When you save HTML edits, the page is detached from its
            design: it keeps how it looks now, but it can no longer be opened in the designer. To undo, restore
            an earlier version from the page history.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="gap-2 sm:gap-0">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" onClick={onConfirm}>
            Edit as HTML
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
DetachDesignDialog.propTypes = {
  open: PropTypes.bool.isRequired,
  onOpenChange: PropTypes.func.isRequired,
  onConfirm: PropTypes.func.isRequired,
};
