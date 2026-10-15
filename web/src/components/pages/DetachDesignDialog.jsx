import PropTypes from 'prop-types';
import { useTranslation } from 'react-i18next';
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from '../ui/dialog';
import { Button } from '../ui/button';

// Confirms "Edit as HTML" on a designed page. Saving HTML by hand detaches
// the page from its design (wiki_page[design] = null): the designer can no
// longer open it. Page history can restore the design, since every revision
// snapshots it.
//
// Whether it keeps its look depends on the account: with hand-written CSS
// allowed, the page's stylesheet stays (it becomes an ordinary page's CSS);
// with it off (customCssEnabled false), the server clears the stylesheet on
// detach, so the dialog says so instead of promising the look is kept.
export default function DetachDesignDialog({ open, onOpenChange, onConfirm, customCssEnabled = true }) {
  const { t } = useTranslation();
  const cssOff = customCssEnabled === false;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('pages.detachDesign.title')}</DialogTitle>
          <DialogDescription>
            {cssOff ? t('pages.detachDesign.bodyCssOff') : t('pages.detachDesign.body')}
            {cssOff && <> {t('pages.detachDesign.cssRemoved')}</>} {t('pages.detachDesign.undo')}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className="gap-2 sm:gap-0">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t('pages.detachDesign.cancel')}
          </Button>
          <Button type="button" onClick={onConfirm}>
            {t('pages.detachDesign.confirm')}
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
  customCssEnabled: PropTypes.bool,
};
