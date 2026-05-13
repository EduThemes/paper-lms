import React from 'react';
import { useParams } from 'react-router-dom';
import Layout from '../components/Layout';
import CurrencyList from '../components/gamification/CurrencyList';

// GamificationCurrenciesPage hosts the W2-B currency editor.
//
// Two routes mount this page:
//   * /admin/gamification/currencies         → site scope (tenant admin)
//   * /courses/:courseId/gamification/currencies → course scope (instructor)
//
// Scope is inferred from the presence of :courseId in the URL; the
// CurrencyList component derives the API path from that.
export default function GamificationCurrenciesPage() {
  const { courseId } = useParams();
  return (
    <Layout>
      <div className="max-w-4xl mx-auto py-6">
        <CurrencyList courseId={courseId ? Number(courseId) : undefined} />
      </div>
    </Layout>
  );
}
