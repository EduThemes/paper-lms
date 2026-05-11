import { useEffect, useCallback } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';

/**
 * Warns the user before navigating away from a page with unsaved changes.
 * Handles both in-app navigation (via history pushState interception) and
 * tab close/refresh (beforeunload).
 *
 * Note: Uses pushState interception instead of useBlocker, which requires
 * a data router (createBrowserRouter). This app uses BrowserRouter.
 *
 * @param {boolean} isDirty - Whether the form has unsaved changes
 * @param {string} [message] - Custom confirmation message (browser may override for beforeunload)
 */
export default function useUnsavedChanges(isDirty, message = 'You have unsaved changes. Are you sure you want to leave?') {
  // Block browser back/forward navigation
  useEffect(() => {
    if (!isDirty) return;

    const handlePopState = (e) => {
      if (window.confirm(message)) return;
      // User cancelled — push the current URL back to undo the navigation
      window.history.pushState(null, '', window.location.href);
    };

    // Push a dummy state so popstate fires when user hits back
    window.history.pushState(null, '', window.location.href);
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, [isDirty, message]);

  // Block tab close / refresh
  useEffect(() => {
    if (!isDirty) return;
    const handler = (e) => {
      e.preventDefault();
      e.returnValue = message;
      return message;
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [isDirty, message]);
}
