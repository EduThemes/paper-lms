import React from 'react';
import { render, screen, waitFor, fireEvent, act } from '@testing-library/react';
import { contentItemsToEditorContent, safeUrl } from '../ltiEditorContent';
import { toCourseNavTabs } from '../../../hooks/useLtiCourseNavTabs';
import { launchOptionsFromSearch } from '../../../pages/ExternalToolLaunchPage';
import { getModuleItemLink } from '../../../utils/moduleItems';
import AssignmentExternalToolPanel, { assignmentUsesExternalTool } from '../AssignmentExternalToolPanel';
import LtiDeepLinkModal from '../LtiDeepLinkModal';
import { ltiApi } from '../../../services/api_lti';

vi.mock('../../../services/api_lti', async (importOriginal) => {
  const actual = await importOriginal();
  return {
    ...actual,
    ltiApi: {
      courseTools: vi.fn(),
      getAssignmentTool: vi.fn(),
      setAssignmentTool: vi.fn(),
      removeAssignmentTool: vi.fn(),
    },
  };
});

vi.mock('../../Layout', () => ({ default: ({ children }) => <div>{children}</div> }));
vi.mock('../../CourseNav', () => ({ default: () => null }));

describe('contentItemsToEditorContent (editor_button, G008)', () => {
  test('turns links, resource links and images into editor nodes and drops unsafe URLs', () => {
    const out = contentItemsToEditorContent([
      { type: 'ltiResourceLink', title: 'Quiz', url: '/courses/1/external_tools/2?resource_link_id=u' },
      { type: 'link', url: 'https://example.com/a' },
      { type: 'image', url: 'https://example.com/i.png', title: 'Pic' },
      { type: 'link', url: 'javascript:alert(1)' },
      { type: 'html', html: '<p>Hi</p>' },
      { type: 'unknown' },
    ]);
    expect(out).toHaveLength(4);
    expect(out[0].content[0]).toMatchObject({ text: 'Quiz', marks: [{ type: 'link', attrs: { href: '/courses/1/external_tools/2?resource_link_id=u' } }] });
    expect(out[1].content[0].text).toBe('https://example.com/a');
    expect(out[2]).toEqual({ type: 'image', attrs: { src: 'https://example.com/i.png', alt: 'Pic' } });
    expect(out[3]).toBe('<p>Hi</p>');
    expect(safeUrl('data:text/html,x')).toBe('');
  });
});

describe('course navigation tabs (G131)', () => {
  test('maps placement tools to CourseNav tabs', () => {
    expect(toCourseNavTabs([{ id: 4, text: 'Library', name: 'Lib' }, { id: 5, name: 'Other' }])).toEqual([
      { path: '/external_tools/4/launch/course_navigation', label: 'Library' },
      { path: '/external_tools/5/launch/course_navigation', label: 'Other' },
    ]);
    expect(toCourseNavTabs(null)).toEqual([]);
  });
});

describe('ExternalToolLaunchPage options', () => {
  test('forwards resource link / module item / assignment, else the placement', () => {
    expect(launchOptionsFromSearch(new URLSearchParams('module_item_id=9'), 'course_navigation')).toEqual({ module_item_id: '9' });
    expect(launchOptionsFromSearch(new URLSearchParams(''), 'course_navigation')).toEqual({ placement: 'course_navigation' });
    expect(launchOptionsFromSearch(new URLSearchParams(''))).toEqual({});
  });
});

describe('module ExternalTool items', () => {
  test('link to the course tool page with the module item', () => {
    expect(getModuleItemLink({ id: 12, type: 'ContextExternalTool', content_id: 4 }, 3)).toBe('/courses/3/external_tools/4?module_item_id=12');
    expect(getModuleItemLink({ id: 12, type: 'ExternalTool', content_id: null }, 3)).toBeNull();
  });
});

describe('AssignmentExternalToolPanel', () => {
  beforeEach(() => vi.clearAllMocks());

  test('launches the configured tool for the assignment', async () => {
    ltiApi.getAssignmentTool.mockResolvedValue({ tool_id: 4, tool_name: 'Quiz Tool', title: 'Unit 1' });
    ltiApi.courseTools.mockResolvedValue([]);
    render(<AssignmentExternalToolPanel courseId="3" assignment={{ id: 70, submission_types: ['external_tool'] }} isTeacher={false} />);
    const frame = await screen.findByTitle('Unit 1');
    expect(frame.getAttribute('src')).toBe('/api/v1/courses/3/external_tools/4/launch?assignment_id=70');
  });

  test('a student sees nothing for an assignment without a tool', async () => {
    ltiApi.getAssignmentTool.mockRejectedValue(Object.assign(new Error('nf'), { status: 404 }));
    const { container } = render(<AssignmentExternalToolPanel courseId="3" assignment={{ id: 70, submission_types: ['online_text_entry'] }} isTeacher={false} />);
    await waitFor(() => expect(ltiApi.getAssignmentTool).toHaveBeenCalled());
    expect(container.innerHTML).toBe('');
  });

  test('a teacher attaches a tool', async () => {
    ltiApi.getAssignmentTool.mockRejectedValue(Object.assign(new Error('nf'), { status: 404 }));
    ltiApi.courseTools.mockImplementation((_c, placement) => Promise.resolve(placement ? [] : [{ id: 4, name: 'Quiz Tool' }]));
    ltiApi.setAssignmentTool.mockResolvedValue({ tool_id: 4, tool_name: 'Quiz Tool', title: 'Unit 1' });
    render(<AssignmentExternalToolPanel courseId="3" assignment={{ id: 70, submission_types: ['external_tool'] }} isTeacher />);
    const select = await screen.findByLabelText('Tool');
    await screen.findByRole('option', { name: 'Quiz Tool' });
    fireEvent.change(select, { target: { value: '4' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(ltiApi.setAssignmentTool).toHaveBeenCalledWith('3', 70, { tool_id: 4, url: '' }));
    expect(assignmentUsesExternalTool({ submission_types: 'online_url,external_tool' })).toBe(true);
  });
});

describe('LtiDeepLinkModal', () => {
  test('reports a same-origin deep linking result', async () => {
    const onResult = vi.fn();
    render(<LtiDeepLinkModal open title="Pick" src="/api/v1/x" onResult={onResult} onClose={() => {}} />);
    await act(async () => {
      window.dispatchEvent(new MessageEvent('message', {
        origin: window.location.origin,
        data: { subject: 'LtiDeepLinkingResponse', result: { items: [{ type: 'link', url: 'https://e.example.com' }] } },
      }));
      window.dispatchEvent(new MessageEvent('message', { origin: 'https://evil.example.net', data: { subject: 'LtiDeepLinkingResponse' } }));
    });
    expect(onResult).toHaveBeenCalledTimes(1);
    expect(onResult.mock.calls[0][0].items).toHaveLength(1);
  });
});
