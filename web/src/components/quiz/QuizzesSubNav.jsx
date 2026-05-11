import React from 'react';
import { Link, useParams, useLocation } from 'react-router-dom';
import { ClipboardList, Layers, BookOpen, BarChart3 } from 'lucide-react';

/**
 * Sub-navigation rendered under CourseNav on every Quizzes-section page.
 * Shows: Quizzes | Item Banks | Stimuli (and Item Analysis when scoped to a quiz).
 *
 * Matches the existing CourseNav styling — light border-bottom and small
 * tab buttons, neutral by default with a brand-coloured active state.
 */
const QuizzesSubNav = ({ quizId = null }) => {
  const { courseId } = useParams();
  const location = useLocation();
  if (!courseId) return null;

  const basePath = `/courses/${courseId}`;
  const tabs = [
    { path: '/quizzes', label: 'Quizzes', icon: ClipboardList, match: '/quizzes' },
    { path: '/item-banks', label: 'Item Banks', icon: Layers, match: '/item-banks' },
    { path: '/stimuli', label: 'Stimuli', icon: BookOpen, match: '/stimuli' },
  ];
  if (quizId) {
    tabs.push({
      path: `/quizzes/${quizId}/item-analysis`,
      label: 'Item Analysis',
      icon: BarChart3,
      match: '/item-analysis',
    });
  }

  const isActive = (tab) => {
    if (tab.path === '/quizzes') {
      // Highlight Quizzes only when on the list/edit/take view, not on the
      // /item-banks etc sibling pages.
      const onQuizSection = location.pathname.startsWith(`${basePath}/quizzes`);
      const onSubsection = ['/item-banks', '/stimuli'].some(s => location.pathname.includes(s));
      const onItemAnalysis = location.pathname.includes('/item-analysis');
      return onQuizSection && !onSubsection && !onItemAnalysis;
    }
    return location.pathname.includes(tab.match);
  };

  return (
    <div className="border-b border-border-default bg-surface-0 -mx-6 px-6 mb-4" aria-label="Quizzes sub-navigation">
      <nav className="flex items-center gap-1 overflow-x-auto">
        {tabs.map(tab => {
          const Icon = tab.icon;
          const active = isActive(tab);
          return (
            <Link
              key={tab.path}
              to={basePath + tab.path}
              className={`inline-flex items-center gap-1.5 px-3 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${
                active
                  ? 'border-brand-600 text-brand-600'
                  : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              {tab.label}
            </Link>
          );
        })}
      </nav>
    </div>
  );
};

export default QuizzesSubNav;
