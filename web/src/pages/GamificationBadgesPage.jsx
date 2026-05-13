import React from 'react';
import { useParams } from 'react-router-dom';
import Layout from '../components/Layout';
import BadgesList from '../components/gamification/BadgesList';

// GamificationBadgesPage hosts the W2-D badge editor. Mirrors
// GamificationCurrenciesPage from W2-B: scope inferred from
// :courseId presence in the URL.
//
//   /admin/gamification/badges                      → site (tenant admin)
//   /courses/:courseId/gamification/badges          → course (instructor)
export default function GamificationBadgesPage() {
  const { courseId } = useParams();
  return (
    <Layout>
      <div className="max-w-5xl mx-auto py-6">
        <BadgesList courseId={courseId ? Number(courseId) : undefined} />
      </div>
    </Layout>
  );
}
