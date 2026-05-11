import { useEffect } from 'react';

/**
 * Consolidates click-outside + Escape-key dismissal logic for popovers,
 * dropdowns, and modals.
 *
 * Listeners are attached only while `isOpen` is true and removed on close
 * or unmount.
 *
 * @param {React.RefObject} ref - Ref pointing to the element that defines
 *   the "inside" boundary. Clicks inside this element are ignored.
 * @param {boolean} isOpen - Whether the dismissable surface is currently open.
 * @param {() => void} onClose - Called when the user dismisses (Escape or
 *   outside click).
 * @param {{ closeOnEscape?: boolean, closeOnOutsideClick?: boolean }} [options]
 */
const useDismissable = (ref, isOpen, onClose, options = {}) => {
  const { closeOnEscape = true, closeOnOutsideClick = true } = options;

  useEffect(() => {
    if (!isOpen) return undefined;

    const handleMouseDown = (e) => {
      if (!closeOnOutsideClick) return;
      if (ref?.current && !ref.current.contains(e.target)) {
        onClose();
      }
    };

    const handleKeyDown = (e) => {
      if (closeOnEscape && e.key === 'Escape') {
        onClose();
      }
    };

    if (closeOnOutsideClick) {
      document.addEventListener('mousedown', handleMouseDown);
    }
    if (closeOnEscape) {
      document.addEventListener('keydown', handleKeyDown);
    }

    return () => {
      document.removeEventListener('mousedown', handleMouseDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [ref, isOpen, onClose, closeOnEscape, closeOnOutsideClick]);
};

export default useDismissable;
