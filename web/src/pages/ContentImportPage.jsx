import React, { useState, useEffect, useRef } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import { Upload, CheckCircle, AlertCircle, Clock, FileArchive } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';

const STATUS_ICONS = {
  created: Clock,
  running: Clock,
  completed: CheckCircle,
  failed: AlertCircle,
};

const STATUS_COLORS = {
  created: 'text-text-tertiary bg-surface-2',
  running: 'text-brand-600 bg-brand-50',
  completed: 'text-accent-success bg-accent-success/10',
  failed: 'text-accent-danger bg-accent-danger/10',
};

const ContentImportPage = () => {
  const { t } = useTranslation();
  const { courseId } = useParams();
  const { user } = useAuth();
  const isTeacher = useIsTeacher(courseId);
  const [migrations, setMigrations] = useState([]);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [loading, setLoading] = useState(true);
  const fileRef = useRef(null);
  const pollRef = useRef(null);

  const fetchMigrations = async () => {
    try {
      const result = await api.getContentMigrations(courseId);
      setMigrations(result.data || []);
    } catch (err) {
      // Not critical if migration list fails
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchMigrations();
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [courseId]);

  // Poll for running migrations
  useEffect(() => {
    const hasRunning = migrations.some(m => m.workflow_state === 'running' || m.workflow_state === 'created');
    if (hasRunning) {
      pollRef.current = setInterval(fetchMigrations, 5000);
    } else if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [migrations]);

  const handleUpload = async (e) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const validTypes = ['.imscc', '.zip', '.xml'];
    const ext = file.name.toLowerCase().slice(file.name.lastIndexOf('.'));
    if (!validTypes.includes(ext)) {
      setError(t('contentImport.invalidFileType'));
      return;
    }

    setUploading(true);
    setError(null);
    setSuccess(null);

    try {
      await api.importContentPackage(courseId, file);
      setSuccess(t('contentImport.uploadSuccess', { name: file.name }));
      // Refresh migration list
      setTimeout(fetchMigrations, 1000);
    } catch (err) {
      setError(err.message || t('contentImport.uploadFailed'));
    } finally {
      setUploading(false);
      if (fileRef.current) fileRef.current.value = '';
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString(undefined, {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: 'numeric', minute: '2-digit',
    });
  };

  const formatType = (type) => {
    const labelKeys = {
      common_cartridge_importer: 'contentImport.typeCommonCartridge',
      canvas_cartridge_importer: 'contentImport.typeCanvasExport',
      qti_converter: 'contentImport.typeQtiQuiz',
      moodle_converter: 'contentImport.typeMoodleBackup',
      course_copy_importer: 'contentImport.typeCourseCopy',
    };
    if (labelKeys[type]) return t(labelKeys[type]);
    return type || t('contentImport.typeFallback');
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}`} replace />;
  if (isTeacher === null) return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  {t('common.loading')}
</div></Layout>;

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6">
        <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">
          {t('contentImport.backToCourse')}
        </Link>
        <h2 className="text-2xl font-bold text-text-primary mt-2">{t('contentImport.title')}</h2>
        <p className="text-text-tertiary text-sm mt-1">
          {t('contentImport.description')}
        </p>
      </div>

      {/* Upload Area */}
      <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
        <div
          className={`border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
            uploading ? 'border-blue-300 bg-brand-50' : 'border-border-strong hover:border-blue-400'
          }`}
        >
          <FileArchive className="w-12 h-12 text-text-disabled mx-auto mb-3" />
          <p className="text-text-secondary font-medium mb-1">
            {uploading ? t('contentImport.uploading') : t('contentImport.uploadAPackage')}
          </p>
          <p className="text-text-tertiary text-sm mb-4">
            {t('contentImport.supportedFormats')}
          </p>
          <label className={`inline-flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium cursor-pointer transition-colors ${
            uploading
              ? 'bg-gray-300 text-text-tertiary cursor-not-allowed'
              : 'bg-brand-600 text-white hover:bg-brand-700'
          }`}>
            <Upload className="w-4 h-4" />
            {uploading ? t('contentImport.uploading') : t('contentImport.chooseFile')}
            <input
              ref={fileRef}
              type="file"
              accept=".imscc,.zip,.xml"
              onChange={handleUpload}
              disabled={uploading}
              className="hidden"
            />
          </label>
        </div>

        {error && (
          <div className="mt-4 p-3 bg-accent-danger/10 text-accent-danger rounded-md text-sm flex items-center gap-2">
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            {error}
          </div>
        )}
        {success && (
          <div className="mt-4 p-3 bg-accent-success/10 text-accent-success rounded-md text-sm flex items-center gap-2">
            <CheckCircle className="w-4 h-4 flex-shrink-0" />
            {success}
          </div>
        )}
      </div>

      {/* Migration History */}
      <div className="bg-surface-0 rounded-lg shadow">
        <div className="p-4 border-b">
          <h3 className="font-semibold text-text-primary">{t('contentImport.history')}</h3>
        </div>
        {loading ? (
          <div className="p-6 text-center text-text-tertiary">{t('common.loading')}</div>
        ) : migrations.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">{t('contentImport.noImportsYet')}</div>
        ) : (
          <div className="divide-y">
            {migrations.map((m) => {
              const StatusIcon = STATUS_ICONS[m.workflow_state] || Clock;
              const statusColor = STATUS_COLORS[m.workflow_state] || STATUS_COLORS.created;

              return (
                <div key={m.id} className="p-4 flex items-center justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <div className={`p-2 rounded-full ${statusColor}`}>
                      <StatusIcon className="w-4 h-4" />
                    </div>
                    <div className="min-w-0">
                      <div className="text-sm font-medium text-text-primary">
                        {formatType(m.migration_type)}
                      </div>
                      <div className="text-xs text-text-tertiary">
                        {formatDate(m.created_at)}
                      </div>
                    </div>
                  </div>
                  <div className="text-right">
                    <span className={`text-xs font-medium px-2 py-1 rounded-full ${statusColor}`}>
                      {m.workflow_state === 'running' ? t('contentImport.statusProcessing') :
                       m.workflow_state === 'created' ? t('contentImport.statusQueued') :
                       m.workflow_state === 'completed' ? t('contentImport.statusCompleted') :
                       m.workflow_state === 'failed' ? t('contentImport.statusFailed') :
                       m.workflow_state}
                    </span>
                    {m.workflow_state === 'running' && m.progress_url && (
                      <div className="mt-1 w-32 bg-border-default rounded-full h-1.5">
                        <div
                          className="bg-brand-600 h-1.5 rounded-full transition-all"
                          style={{ width: `${m.completion || 0}%` }}
                        />
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </Layout>
  );
};

export default ContentImportPage;
