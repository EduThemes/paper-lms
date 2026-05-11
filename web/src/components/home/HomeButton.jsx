import React from 'react';
import { useNavigate } from 'react-router-dom';
import { resolveIcon } from '../../utils/iconResolver';

const HomeButton = ({ button, uiMode, todaysLesson, continueData, courseId }) => {
  const navigate = useNavigate();
  const isK2 = uiMode === 'k2';
  const is35 = uiMode === '3-5';

  const resolveURL = () => {
    switch (button.button_type) {
      case 'todays_lesson':
        if (!todaysLesson) return null;
        if (todaysLesson.link_url) return todaysLesson.link_url;
        if (todaysLesson.link_type === 'module') return `/courses/${courseId}/modules`;
        if (todaysLesson.link_type === 'page' && todaysLesson.link_id) return `/courses/${courseId}/pages/${todaysLesson.link_id}`;
        if (todaysLesson.link_type === 'assignment' && todaysLesson.link_id) return `/courses/${courseId}/assignments/${todaysLesson.link_id}`;
        return `/courses/${courseId}`;
      case 'continue':
        return continueData?.url || `/courses/${courseId}`;
      case 'my_work':
        return `/courses/${courseId}/gradebook`;
      case 'inbox':
        return '/inbox';
      case 'announcements':
        return `/courses/${courseId}/announcements`;
      case 'custom':
        if (button.link_type === 'external_url') return button.link_url;
        if (button.link_type === 'page' && button.link_id) return `/courses/${courseId}/pages/${button.link_id}`;
        if (button.link_type === 'module' && button.link_id) return `/courses/${courseId}/modules`;
        if (button.link_type === 'assignment' && button.link_id) return `/courses/${courseId}/assignments/${button.link_id}`;
        if (button.link_type === 'discussion' && button.link_id) return `/courses/${courseId}/discussions/${button.link_id}`;
        return button.link_url || `/courses/${courseId}`;
      default:
        return `/courses/${courseId}`;
    }
  };

  const handleClick = () => {
    const url = resolveURL();
    if (!url) return;
    if (url.startsWith('http')) {
      window.open(url, '_blank', 'noopener');
    } else {
      navigate(url);
    }
  };

  const Icon = resolveIcon(button.icon);
  const label = button.button_type === 'todays_lesson'
    ? (todaysLesson?.label || button.label || "Today's Lesson")
    : button.button_type === 'continue'
    ? (continueData?.title || button.label || 'Continue')
    : button.label;

  // Style varies by mode
  const bgColor = button.color || 'rgb(var(--color-brand-600))';

  const buttonPadding = isK2 ? 'p-10 min-h-[160px]' : is35 ? 'p-8' : 'p-6';
  const iconSize = isK2 ? 'w-20 h-20' : is35 ? 'w-12 h-12' : 'w-8 h-8';

  const bgStyle = isK2
    ? { backgroundColor: bgColor }
    : is35
    ? { backgroundColor: bgColor }
    : { backgroundColor: bgColor + '15' };

  const iconColor = isK2 || is35 ? 'text-white' : '';
  const textColor = isK2 || is35 ? 'text-white' : 'text-text-primary';

  return (
    <button
      onClick={handleClick}
      className={`${buttonPadding} rounded-xl flex flex-col items-center justify-center gap-3 transition-transform hover:scale-105 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-brand-500`}
      style={bgStyle}
      aria-label={label}
    >
      {Icon && <Icon className={`${iconSize} ${iconColor}`} style={!isK2 && !is35 ? { color: bgColor } : undefined} />}
      {!isK2 && (
        <span className={`font-semibold text-center ${is35 ? 'text-lg' : 'text-sm'} ${textColor}`}>
          {label}
        </span>
      )}
    </button>
  );
};

export default HomeButton;
