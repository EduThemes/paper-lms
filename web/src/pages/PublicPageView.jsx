import React, { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { BookOpen } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import { sanitizeHTML } from '../components/RichContentViewer';

const PublicPageView = () => {
  const { t } = useTranslation();
  const { courseId, slug } = useParams();
  const [page, setPage] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchPage = async () => {
      try {
        const data = await api.getPublicPage(courseId, slug);
        setPage(data);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };
    fetchPage();
  }, [courseId, slug]);

  if (loading) {
    return (
      <div className="min-h-screen bg-surface-1 flex items-center justify-center">
        <div className="text-text-tertiary">{t('common.loading')}</div>
      </div>
    );
  }

  if (error || !page) {
    return (
      <div className="min-h-screen bg-surface-1 flex items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-text-primary mb-2">{t('publicPage.notFoundTitle')}</h1>
          <p className="text-text-tertiary mb-4">{t('publicPage.notFoundDescription')}</p>
          <Link to="/login" className="text-brand-600 hover:underline">{t('publicPage.goToLogin')}</Link>
        </div>
      </div>
    );
  }

  // Website mode: full-width, no sidebar, clean layout
  if (page.website_mode) {
    return (
      <div className="min-h-screen bg-surface-0 flex flex-col">
        <main className="flex-1 max-w-4xl mx-auto px-6 py-12 w-full">
          <h1 className="text-3xl font-bold text-text-primary mb-8">{page.title}</h1>
          <div
            className="prose prose-lg max-w-none"
            dangerouslySetInnerHTML={{ __html: sanitizeHTML(page.body) }}
          />
        </main>
        <footer className="border-t border-border-default py-4 text-center text-sm text-text-disabled">
          {t('publicPage.poweredBy')}
        </footer>
      </div>
    );
  }

  // Normal public mode: minimal sidebar (logo only) + page content
  return (
    <div className="min-h-screen bg-surface-1 flex">
      <aside className="fixed inset-y-0 left-0 z-30 flex flex-col items-center w-16 bg-[#2D3B45]">
        <div className="flex items-center justify-center h-14 border-b border-white/10 w-full">
          <Link to="/" className="text-white" title={t('nav.paperLms')}>
            <BookOpen className="w-6 h-6 text-accent-danger" />
          </Link>
        </div>
      </aside>

      <div className="flex-1 ml-16">
        <main className="max-w-4xl mx-auto px-6 py-8">
          <h1 className="text-2xl font-bold text-text-primary mb-6">{page.title}</h1>
          <div className="bg-surface-0 rounded-lg shadow p-6">
            <div
              className="prose max-w-none"
              dangerouslySetInnerHTML={{ __html: sanitizeHTML(page.body) }}
            />
          </div>
        </main>
      </div>
    </div>
  );
};

export default PublicPageView;
