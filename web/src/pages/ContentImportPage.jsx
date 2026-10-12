import React, { useState } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { Copy, FileArchive, Download } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import Page from '../components/Page';
import CourseCopyPanel from '../components/contentMigration/CourseCopyPanel';
import PackageImportPanel from '../components/contentMigration/PackageImportPanel';
import MigrationHistory from '../components/contentMigration/MigrationHistory';
import { useContentMigrations } from '../services/api_content_migration';

// ContentImportPage is the course's "Import Course Content" hub (G067):
// copy content from another course (all or selected, with date
// shifting) or import a Canvas/Common Cartridge package, and follow the
// background jobs in the history list.
const ContentImportPage = () => {
  const { t } = useTranslation();
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [tab, setTab] = useState('copy');
  const migrations = useContentMigrations(isTeacher ? courseId : null);

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) {
    return (
      <Layout>
        <div className="py-12 text-center text-text-tertiary">{t('common.loading')}</div>
      </Layout>
    );
  }

  const tabs = [
    { id: 'copy', label: t('contentMigration.copy.title', 'Copy a course'), icon: Copy },
    { id: 'package', label: t('contentMigration.package.title', 'Import a package'), icon: FileArchive },
  ];

  return (
    <Page query={migrations} title={t('contentImport.title')}>
      {(list) => (
        <Layout>
          <CourseNav />
          <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
            <div>
              <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
                {t('contentImport.backToCourse')}
              </Link>
              <h2 className="text-2xl font-bold text-text-primary mt-2">{t('contentImport.title')}</h2>
              <p className="text-text-tertiary text-sm mt-1">
                {t('contentMigration.description', 'Copy content from another course you teach, or import a Canvas course export or Common Cartridge package.')}
              </p>
            </div>
            <Link
              to={`/courses/${courseId}/content_export`}
              className="inline-flex items-center gap-2 text-sm font-medium text-brand-600 hover:underline"
            >
              <Download className="w-4 h-4" />
              {t('contentMigration.export.link', 'Export course content')}
            </Link>
          </div>

          <div className="bg-surface-0 rounded-lg shadow mb-6">
            <div role="tablist" aria-label={t('contentImport.title')} className="flex border-b border-border-default px-4">
              {tabs.map(({ id, label, icon: Icon }) => (
                <button
                  key={id}
                  type="button"
                  role="tab"
                  id={`import-tab-${id}`}
                  aria-selected={tab === id}
                  aria-controls={`import-panel-${id}`}
                  onClick={() => setTab(id)}
                  className={`flex items-center gap-2 px-3 py-3 -mb-px border-b-2 text-sm font-medium ${
                    tab === id ? 'border-brand-500 text-brand-600' : 'border-transparent text-text-tertiary hover:text-text-secondary'
                  }`}
                >
                  <Icon className="w-4 h-4" />
                  {label}
                </button>
              ))}
            </div>
            <div className="p-6" role="tabpanel" id={`import-panel-${tab}`} aria-labelledby={`import-tab-${tab}`}>
              {tab === 'copy' ? <CourseCopyPanel courseId={courseId} /> : <PackageImportPanel courseId={courseId} />}
            </div>
          </div>

          <div className="bg-surface-0 rounded-lg shadow">
            <div className="p-4 border-b border-border-default">
              <h3 className="font-semibold text-text-primary">{t('contentImport.history')}</h3>
            </div>
            <MigrationHistory migrations={list} />
          </div>
        </Layout>
      )}
    </Page>
  );
};

export default ContentImportPage;
