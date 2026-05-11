import React, { createContext, useContext, useState, useEffect, useMemo, useCallback } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { api } from '../services/api';
import { useAuth } from './AuthContext';

const CourseUIContext = createContext(null);

const STAFF_ENROLLMENT_TYPES = new Set([
  'TeacherEnrollment',
  'TaEnrollment',
  'DesignerEnrollment',
]);

const VALID_PREVIEW_MODES = new Set(['standard', 'k2', '3-5']);

const defaultValue = {
  course: null,
  setCourse: () => {},
  loading: false,
  uiMode: 'standard',
  effectiveMode: 'standard',
  isK2: false,
  is35: false,
  isSimplified: false,
  isStaff: false,
  isPreview: false,
  setPreviewMode: () => {},
  exitPreview: () => {},
};

export const useCourseUI = () => {
  const ctx = useContext(CourseUIContext);
  return ctx || defaultValue;
};

export const CourseUIProvider = ({ children }) => {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [course, setCourse] = useState(null);
  const [enrollments, setEnrollments] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!courseId) {
      setCourse(null);
      setEnrollments(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);

    Promise.allSettled([
      api.getCourse(courseId),
      api.getEnrollments(courseId, 1, 200),
    ]).then(([courseRes, enrollRes]) => {
      if (cancelled) return;
      if (courseRes.status === 'fulfilled') setCourse(courseRes.value);
      setEnrollments(enrollRes.status === 'fulfilled' ? (enrollRes.value?.data || []) : []);
      setLoading(false);
    });

    return () => { cancelled = true; };
  }, [courseId]);

  const isStaff = useMemo(() => {
    if (!user) return false;
    if (user.role === 'admin' || user.role === 'site_admin') return true;
    if (!enrollments) return false;
    const mine = enrollments.find(e => e.user_id === user.id || e.user?.id === user.id);
    if (!mine) return false;
    return STAFF_ENROLLMENT_TYPES.has(mine.type) || STAFF_ENROLLMENT_TYPES.has(mine.role);
  }, [user, enrollments]);

  const uiMode = course?.ui_mode || 'standard';
  const previewParam = searchParams.get('preview');
  const isPreview = isStaff && VALID_PREVIEW_MODES.has(previewParam);

  // Effective mode = what the layout actually renders.
  //   Students:           course.ui_mode (k2 / 3-5 / standard) — age-appropriate UI.
  //   Staff (admin/teacher/TA/designer): always 'standard' so they keep full chrome,
  //     unless they explicitly opt into preview via ?preview=k2|3-5.
  const effectiveMode = isStaff
    ? (isPreview ? previewParam : 'standard')
    : uiMode;

  const setPreviewMode = useCallback((mode) => {
    const next = new URLSearchParams(searchParams);
    if (mode && VALID_PREVIEW_MODES.has(mode) && mode !== 'standard') {
      next.set('preview', mode);
    } else {
      next.delete('preview');
    }
    setSearchParams(next, { replace: true });
  }, [searchParams, setSearchParams]);

  const exitPreview = useCallback(() => setPreviewMode(null), [setPreviewMode]);

  const value = useMemo(() => ({
    course,
    setCourse,
    loading,
    uiMode,
    effectiveMode,
    isK2: effectiveMode === 'k2',
    is35: effectiveMode === '3-5',
    isSimplified: effectiveMode === 'k2' || effectiveMode === '3-5',
    isStaff,
    isPreview,
    setPreviewMode,
    exitPreview,
  }), [course, loading, uiMode, effectiveMode, isStaff, isPreview, setPreviewMode, exitPreview]);

  return (
    <CourseUIContext.Provider value={value}>
      {children}
    </CourseUIContext.Provider>
  );
};
