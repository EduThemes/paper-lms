import { useState, useCallback } from 'react';
import { detectCrossCourseLinks } from '../utils/crossCourseLinks';

export default function useCrossCourseCheck(courseId) {
  const [issues, setIssues] = useState([]);
  const [pendingSave, setPendingSave] = useState(null);

  const checkAndSave = useCallback((html, saveFn) => {
    if (!courseId) {
      saveFn();
      return;
    }
    const found = detectCrossCourseLinks(html, courseId);
    if (found.length > 0) {
      setIssues(found);
      setPendingSave(() => saveFn);
    } else {
      saveFn();
    }
  }, [courseId]);

  const dismiss = useCallback(() => {
    setIssues([]);
    setPendingSave(null);
  }, []);

  const confirm = useCallback(() => {
    if (pendingSave) pendingSave();
    setIssues([]);
    setPendingSave(null);
  }, [pendingSave]);

  return { issues, checkAndSave, dismiss, confirm };
}
