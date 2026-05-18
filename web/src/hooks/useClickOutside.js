import { useEffect } from 'react';

/**
 * Bare-bones click-outside + Escape dismissal hook for dropdowns and popovers
 * that don't need the full `useDismissable` toggle (no `isOpen` gating —
 * callers are expected to mount/unmount this hook only while the surface is
 * actually open, typically by gating the entire wrapping component on `open`).
 *
 * For modal-like surfaces that conditionally attach handlers based on an
 * open boolean, prefer `useDismissable` which exposes the gate explicitly.
 *
 * @param {React.RefObject<HTMLElement>} ref - boundary element. Clicks
 *   inside this element (or its descendants) are ignored.
 * @param {() => void} onClose - called on outside mousedown or Escape keydown.
 */
export function useClickOutside(ref, onClose) {
  useEffect(() => {
    const onDown = (e) => {
      if (ref.current && !ref.current.contains(e.target)) onClose();
    };
    const onEsc = (e) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onEsc);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onEsc);
    };
  }, [ref, onClose]);
}

export default useClickOutside;
