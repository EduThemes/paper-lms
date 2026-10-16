import type { PageDoc, Section, Theme } from '../types';
import { createPage } from '../index';

/** A doc that contains every section type (incl. rich text with links and a divider). */
export function everySection(theme: Theme, layout: PageDoc['layout']): PageDoc {
  const doc = createPage(theme, 'home', 'en', layout);
  const extra: Section[] = [
    { id: 'obj', type: 'objectives', heading: 'Goals', items: ['Explain cells', 'Compare tissues'] },
    { id: 'sch', type: 'schedule', heading: 'Schedule', items: [{ id: 'a', title: 'Unit 1', dates: 'Weeks 1–3', summary: 'Basics', href: '/courses/{course}/modules/9' }, { id: 'b', title: 'Unit 2' }] },
    { id: 'c1', type: 'callout', variant: 'note', heading: 'Note', body: '<p>A note with <a href="https://example.org/x">a source</a>.</p>' },
    { id: 'c2', type: 'callout', variant: 'tip', heading: 'Tip', body: 'Loose text' },
    { id: 'c3', type: 'callout', variant: 'important', heading: 'Important', body: '<ul><li>One</li><li>Two</li></ul>' },
    { id: 'txt', type: 'text', body: '<p>No heading here.</p>' },
    { id: 'div', type: 'divider' },
  ];
  doc.sections.push(...extra);
  return doc;
}
