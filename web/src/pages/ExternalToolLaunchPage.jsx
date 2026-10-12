import React from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ExternalLink } from 'lucide-react';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import LtiLaunchFrame from '../components/lti/LtiLaunchFrame';
import { ltiApi, ltiLaunchUrl } from '../services/api_lti';

// Launch parameters this page forwards from its own query string.
const FORWARDED = ['resource_link_id', 'module_item_id', 'assignment_id'];

export function launchOptionsFromSearch(searchParams, routePlacement) {
  const opts = {};
  FORWARDED.forEach((k) => {
    const v = searchParams.get(k);
    if (v) opts[k] = v;
  });
  const placement = routePlacement || searchParams.get('placement');
  if (!Object.keys(opts).length && placement) {
    opts.placement = placement;
  }
  return opts;
}

/**
 * Course tool page (G131): course_navigation tabs, module ExternalTool
 * items and embedded resource links all land here and launch the tool in
 * an iframe.
 */
export default function ExternalToolLaunchPage() {
  const { courseId, toolId, placement } = useParams();
  const [searchParams] = useSearchParams();
  const toolsQuery = useQuery({
    queryKey: ['lti', 'courseTools', String(courseId)],
    queryFn: () => ltiApi.courseTools(courseId),
    enabled: Boolean(courseId),
  });
  const tool = (toolsQuery.data || []).find((t) => String(t.id) === String(toolId));
  const src = ltiLaunchUrl(courseId, toolId, launchOptionsFromSearch(searchParams, placement));
  const title = tool?.text || tool?.name || 'External tool';

  return (
    <Layout>
      <CourseNav />
      <div className="mb-3 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-text-primary">{title}</h1>
        <a href={src} target="_blank" rel="noopener noreferrer" className="flex items-center gap-1 text-sm text-brand-600 hover:underline">
          <ExternalLink className="h-4 w-4" />
          Open in a new window
        </a>
      </div>
      <LtiLaunchFrame src={src} title={title} height={720} />
    </Layout>
  );
}
