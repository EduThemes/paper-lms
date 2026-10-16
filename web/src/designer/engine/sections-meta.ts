import type { SectionType } from './types';

export const SECTION_TYPES: { type: SectionType; label: string; description: string }[] = [
  { type: 'title', label: 'Course title', description: 'The course or page name as real, editable text, with an optional subtitle.' },
  { type: 'banner', label: 'Banner image', description: 'A wide theme image shown in full (never cropped). Describe it, or leave the description empty if it is decorative.' },
  { type: 'welcome', label: 'Welcome', description: 'A friendly heading and a few sentences that greet students.' },
  { type: 'this-week', label: 'This week', description: 'A highlighted panel with this week’s dates and a numbered checklist of what to do.' },
  { type: 'quick-links', label: 'Quick links', description: 'Icon buttons to Modules, Syllabus, Grades and other course pages.' },
  { type: 'instructor', label: 'Instructor', description: 'Your photo or initials, a short bio, email and office hours.' },
  { type: 'objectives', label: 'Objectives / checklist', description: 'A short list of goals or steps, each with a check mark.' },
  { type: 'schedule', label: 'Schedule', description: 'Units or weeks with dates and a one-line summary.' },
  { type: 'callout', label: 'Callout', description: 'A boxed note, tip or important message.' },
  { type: 'text', label: 'Text', description: 'A heading and free text: paragraphs, lists and links.' },
  { type: 'divider', label: 'Divider', description: 'A simple line to separate parts of the page.' },
];
