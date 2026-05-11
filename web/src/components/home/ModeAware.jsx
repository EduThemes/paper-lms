import React from 'react';
import { useCourseUI } from '../../contexts/CourseUIContext';

export const ModeText = ({ children, className = '' }) => {
  const { isK2, is35 } = useCourseUI();
  if (isK2) return null;
  const sizeClass = is35 ? 'text-lg font-semibold' : 'text-sm';
  return <span className={`${sizeClass} ${className}`}>{children}</span>;
};

export const ModeIcon = ({ icon: Icon, className = '' }) => {
  const { isK2, is35 } = useCourseUI();
  const sizeClass = isK2 ? 'w-16 h-16' : is35 ? 'w-10 h-10' : 'w-5 h-5';
  return <Icon className={`${sizeClass} ${className}`} />;
};

export const ModeCard = ({ children, className = '' }) => {
  const { isK2, is35 } = useCourseUI();
  const padding = isK2 ? 'p-8' : is35 ? 'p-6' : 'p-4';
  const rounded = isK2 ? 'rounded-2xl' : 'rounded-lg';
  return <div className={`bg-surface-0 shadow ${rounded} ${padding} ${className}`}>{children}</div>;
};
