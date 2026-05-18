import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Link, useParams, useLocation } from 'react-router-dom';
import { ChevronDown, Home, CheckSquare, BarChart2 } from 'lucide-react';
import { useCourseUI } from '../contexts/CourseUIContext';
import { useAuth } from '../contexts/AuthContext';
import { useClickOutside } from '../hooks/useClickOutside';
import { api } from '../services/api';

const isInstructorOrAdmin = (user, enrollments) => {
  if (!user) return false;
  if (user.role === 'admin') return true;
  if (enrollments) {
    // Only check the current user's enrollment, not all enrollments in the course
    const myEnrollment = enrollments.find(e =>
      e.user_id === user.id || e.user?.id === user.id
    );
    if (myEnrollment) {
      return myEnrollment.type === 'TeacherEnrollment' || myEnrollment.type === 'TaEnrollment' ||
        myEnrollment.role === 'TeacherEnrollment' || myEnrollment.role === 'TaEnrollment';
    }
  }
  return false;
};

// Complete map of tab id -> path/label for all available tabs
const TAB_REGISTRY = {
  home:            { path: '', label: 'Home' },
  announcements:   { path: '/announcements', label: 'Announcements' },
  assignments:     { path: '/assignments', label: 'Assignments' },
  modules:         { path: '/modules', label: 'Modules' },
  grades:          { path: '/grades', label: 'Grades' },   // path overridden for teachers below
  people:          { path: '/people', label: 'People' },
  quizzes:         { path: '/quizzes', label: 'Quizzes' },
  discussions:     { path: '/discussions', label: 'Discussions' },
  files:           { path: '/files', label: 'Files' },
  pages:           { path: '/pages', label: 'Pages' },
  rubrics:         { path: '/rubrics', label: 'Rubrics' },
  outcomes:        { path: '/outcomes', label: 'Outcomes' },
  groups:          { path: '/groups', label: 'Groups' },
  collaborations:  { path: '/collaborations', label: 'Collaborations' },
  conferences:     { path: '/conferences', label: 'Conferences' },
  syllabus:        { path: '/syllabus', label: 'Syllabus' },
  attendance:      { path: '/attendance', label: 'Attendance' },
  calendar:        { path: '/calendar', label: 'Calendar' },
  question_banks:  { path: '/question_banks', label: 'Question Banks' },
  accommodations:  { path: '/accommodations', label: 'Accommodations' },
  blueprint:       { path: '/blueprint', label: 'Blueprint' },
  pacing:          { path: '/pacing', label: 'Pacing' },
  analytics:       { path: '/analytics', label: 'Analytics' },
  audit_log:       { path: '/audit_log', label: 'Audit Log' },
  content_import:  { path: '/content_import', label: 'Import Content' },
  external_tools:  { path: '/external_tools', label: 'External Tools' },
  settings:        { path: '/settings', label: 'Settings' },
  leaderboard:     { path: '/leaderboard', label: 'Leaderboard' },
};

// Teacher-only tab IDs (not shown to students)
// `leaderboard` was teacher-only in W3-A; W3-B widens student access
// (server-side pseudonym substitution + tenant-mode render policy
// handles privacy), so it's no longer in this set.
const TEACHER_ONLY_TAB_IDS = new Set([
  'question_banks', 'accommodations', 'blueprint', 'pacing',
  'analytics', 'audit_log', 'content_import', 'external_tools', 'settings',
]);

// Default tab order (used when no customization exists)
const DEFAULT_PRIMARY_IDS = ['home', 'announcements', 'assignments', 'modules', 'grades', 'people'];
const DEFAULT_MORE_IDS = [
  'quizzes', 'discussions', 'files', 'pages', 'rubrics', 'outcomes',
  'groups', 'collaborations', 'conferences', 'syllabus', 'attendance', 'calendar',
  'leaderboard',
  'question_banks', 'accommodations', 'blueprint', 'pacing',
  'analytics', 'audit_log', 'content_import', 'external_tools', 'settings',
];

const PRIMARY_TAB_COUNT = 6;

const getSimplifiedTabs = (gradesPath) => [
  { path: '', label: 'Home', icon: Home },
  { path: gradesPath, label: 'My Work', icon: CheckSquare },
  { path: gradesPath, label: 'Grades', icon: BarChart2 },
];

const CourseNav = () => {
  const { courseId } = useParams();
  const location = useLocation();
  const [moreOpen, setMoreOpen] = useState(false);
  const [enrollments, setEnrollments] = useState(null);
  const moreRef = useRef(null);
  const { course, isK2, is35 } = useCourseUI();
  const { user } = useAuth();

  useEffect(() => {
    if (!courseId || !user) return;
    // Fetch enrollments to determine role; cache in state
    api.getEnrollments(courseId, 1, 10)
      .then(result => setEnrollments(result.data || []))
      .catch(() => setEnrollments([]));
  }, [courseId, user]);

  // Close the "More" dropdown on outside click / Escape. Hook must sit above
  // the conditional `return null` paths below to keep hook order stable.
  const closeMore = useCallback(() => setMoreOpen(false), []);
  useClickOutside(moreRef, closeMore);

  if (!courseId) return null;

  // K-2 mode: no course navigation at all
  if (isK2) return null;

  const isTeacher = isInstructorOrAdmin(user, enrollments);
  const gradesPath = isTeacher ? '/gradebook' : '/grades';
  const simplifiedTabs = getSimplifiedTabs(gradesPath);

  // Build the effective tab list from course.navigation_tabs or defaults
  const navConfig = course?.navigation_tabs;
  let visibleTabs;

  if (Array.isArray(navConfig) && navConfig.length > 0) {
    // Use the saved custom order, filtering hidden tabs and teacher-only tabs for students
    visibleTabs = navConfig
      .sort((a, b) => a.position - b.position)
      .filter(t => {
        if (t.hidden) return false;
        if (!isTeacher && TEACHER_ONLY_TAB_IDS.has(t.id)) return false;
        return TAB_REGISTRY[t.id] != null;
      })
      .map(t => {
        const reg = TAB_REGISTRY[t.id];
        const path = t.id === 'grades' ? gradesPath : reg.path;
        return { path, label: reg.label };
      });
  } else {
    // Default: use the hardcoded primary + more order
    const allDefaultIds = [...DEFAULT_PRIMARY_IDS, ...DEFAULT_MORE_IDS];
    visibleTabs = allDefaultIds
      .filter(id => {
        if (!isTeacher && TEACHER_ONLY_TAB_IDS.has(id)) return false;
        return TAB_REGISTRY[id] != null;
      })
      .map(id => {
        const reg = TAB_REGISTRY[id];
        const path = id === 'grades' ? gradesPath : reg.path;
        return { path, label: reg.label };
      });
  }

  // Split into primary tabs and "more" tabs
  const primaryTabs = visibleTabs.slice(0, PRIMARY_TAB_COUNT);
  const moreTabs = visibleTabs.slice(PRIMARY_TAB_COUNT);

  const basePath = `/courses/${courseId}`;

  const isTabActive = (tabPath) => {
    const fullPath = basePath + tabPath;
    if (tabPath === '') return location.pathname === basePath;
    return location.pathname.startsWith(fullPath);
  };

  const isMoreActive = moreTabs.some((tab) => isTabActive(tab.path));

  // 3-5 mode: simplified tabs with icons
  if (is35) {
    const tabClass35 = (active) =>
      `px-4 py-2 text-base font-semibold border-b-2 transition-colors whitespace-nowrap flex items-center gap-2 ${
        active
          ? 'border-brand-600 text-brand-600'
          : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
      }`;

    return (
      <div className="border-b border-border-default bg-surface-0 -mx-6 px-6 mb-6">
        <div className="flex items-center">
          <nav className="flex items-center space-x-2" aria-label="Course navigation">
            {simplifiedTabs.map((tab) => (
              <Link
                key={tab.path + tab.label}
                to={basePath + tab.path}
                className={tabClass35(isTabActive(tab.path))}
              >
                {tab.icon && <tab.icon className="w-5 h-5" />}
                {tab.label}
              </Link>
            ))}
          </nav>
          <div className="flex-1" />
        </div>
      </div>
    );
  }

  // Standard mode
  const tabClass = (active) =>
    `px-3 py-2 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
      active
        ? 'border-blue-600 text-blue-600'
        : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
    }`;

  return (
    <div className="border-b border-border-default bg-surface-0 -mx-6 px-6 mb-6">
      <div className="flex items-center">
        <nav className="flex items-center space-x-1" aria-label="Course navigation">
          {primaryTabs.map((tab) => (
            <Link
              key={tab.path}
              to={basePath + tab.path}
              className={tabClass(isTabActive(tab.path))}
            >
              {tab.label}
            </Link>
          ))}

          {/* More dropdown */}
          {moreTabs.length > 0 && (
            <div className="relative" ref={moreRef}>
              <button
                onClick={() => setMoreOpen(!moreOpen)}
                className={`${tabClass(isMoreActive && !primaryTabs.some(t => isTabActive(t.path)))} inline-flex items-center gap-1`}
              >
                More
                <ChevronDown className={`w-3.5 h-3.5 transition-transform ${moreOpen ? 'rotate-180' : ''}`} />
              </button>

              {moreOpen && (
                <div className="absolute start-0 top-full mt-1 w-48 bg-surface-0 rounded-md shadow-lg border border-border-default py-1 z-40">
                  {moreTabs.map((tab) => (
                    <Link
                      key={tab.path}
                      to={basePath + tab.path}
                      onClick={() => setMoreOpen(false)}
                      className={`block px-4 py-2 text-sm ${
                        isTabActive(tab.path)
                          ? 'bg-brand-50 text-brand-600 font-medium'
                          : 'text-text-primary hover:bg-surface-1'
                      }`}
                    >
                      {tab.label}
                    </Link>
                  ))}
                </div>
              )}
            </div>
          )}
        </nav>

        {/* Right spacer for future gamification */}
        <div className="flex-1" />
      </div>
    </div>
  );
};

export default CourseNav;
