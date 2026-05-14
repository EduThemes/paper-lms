import React from 'react';
import { useParams } from 'react-router-dom';
import Layout from '../components/Layout';
import RecipesList from '../components/gamification/RecipesList';

// GamificationRecipesPage hosts the W2-E.3 recipe builder list view.
// Mirrors GamificationCurrenciesPage / GamificationBadgesPage: scope
// inferred from `:courseId` presence in the URL.
//
//   /admin/gamification/recipes                      → site (tenant admin)
//   /courses/:courseId/gamification/recipes          → course (instructor)
export default function GamificationRecipesPage() {
  const { courseId } = useParams();
  return (
    <Layout>
      <div className="max-w-6xl mx-auto py-6">
        <RecipesList courseId={courseId ? Number(courseId) : undefined} />
      </div>
    </Layout>
  );
}
