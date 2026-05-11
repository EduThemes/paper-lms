import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  ZoomIn,
  ZoomOut,
  RotateCcw,
  FileText,
  AlertCircle,
  Loader,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { api } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import Layout from '../components/Layout';
import DocumentViewer from '../components/DocumentViewer';

const DocViewerPage = () => {
  const { courseId, assignmentId, userId } = useParams();
  const { user } = useAuth();
  const [submission, setSubmission] = useState(null);
  const [assignment, setAssignment] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [zoom, setZoom] = useState(100);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);

  // Determine if user is read-only (students viewing feedback)
  const [isReadOnly, setIsReadOnly] = useState(false);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const [submissionData, assignmentData] = await Promise.all([
        api.getSubmission(courseId, assignmentId, userId),
        api.getAssignment(courseId, assignmentId),
      ]);

      setSubmission(submissionData);
      setAssignment(assignmentData);

      // If the current user is the student who submitted, set read-only
      if (user && String(user.id) === String(userId)) {
        setIsReadOnly(true);
      }
    } catch (err) {
      setError(err.message || 'Failed to load document');
    } finally {
      setLoading(false);
    }
  }, [courseId, assignmentId, userId, user]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Zoom controls
  const handleZoomIn = () => setZoom(prev => Math.min(prev + 10, 200));
  const handleZoomOut = () => setZoom(prev => Math.max(prev - 10, 50));
  const handleZoomReset = () => setZoom(100);

  // Page navigation
  const handlePrevPage = () => setCurrentPage(prev => Math.max(prev - 1, 1));
  const handleNextPage = () => setCurrentPage(prev => Math.min(prev + 1, totalPages));

  // Determine if submission has viewable content
  const hasViewableContent = () => {
    if (!submission) return false;
    if (submission.body) return true;
    if (submission.submission_type === 'online_text_entry') return true;
    if (submission.submission_type === 'online_url') return true;
    return false;
  };

  // Get document content to display
  const getDocumentContent = () => {
    if (!submission) return '';
    if (submission.body) return submission.body;
    if (submission.url) {
      return `<div class="text-center p-8">
        <p class="text-text-secondary mb-4">This submission is a URL:</p>
        <a href="${submission.url}" target="_blank" rel="noopener noreferrer"
           class="text-brand-600 hover:underline break-all text-lg">${submission.url}</a>
      </div>`;
    }
    return '';
  };

  // Get document title
  const getDocumentTitle = () => {
    if (!assignment || !submission) return '';
    const submitterName = `User ${userId}`;
    return `${assignment.name} - ${submitterName}`;
  };

  if (loading) {
    return (
      <Layout>
        <div className="flex flex-col items-center justify-center h-[calc(100vh-120px)] text-text-tertiary">
          <Loader className="w-8 h-8 animate-spin mb-3" />
          <p className="text-lg">Loading document viewer...</p>
        </div>
      </Layout>
    );
  }

  if (error) {
    return (
      <Layout>
        <div className="flex flex-col items-center justify-center h-[calc(100vh-120px)] text-accent-danger">
          <AlertCircle className="w-12 h-12 mb-3" />
          <p className="text-lg font-medium">Error Loading Document</p>
          <p className="text-sm text-text-tertiary mt-1">{error}</p>
          <Link
            to={`/courses/${courseId}/assignments/${assignmentId}`}
            className="mt-4 text-brand-600 hover:underline text-sm"
          >
            Back to Assignment
          </Link>
        </div>
      </Layout>
    );
  }

  if (!hasViewableContent()) {
    return (
      <Layout>
        <div className="flex flex-col items-center justify-center h-[calc(100vh-120px)] text-text-disabled">
          <FileText className="w-16 h-16 mb-4" />
          <p className="text-xl font-medium text-text-secondary">No Viewable Content</p>
          <p className="text-sm text-text-tertiary mt-2 max-w-md text-center">
            This submission does not have content that can be viewed inline.
            {submission?.submission_type && (
              <span className="block mt-1">
                Submission type: <span className="font-medium">{submission.submission_type}</span>
              </span>
            )}
          </p>
          <div className="mt-6 flex gap-3">
            <Link
              to={`/courses/${courseId}/assignments/${assignmentId}`}
              className="text-brand-600 hover:underline text-sm flex items-center gap-1"
            >
              <ArrowLeft className="w-4 h-4" /> Back to Assignment
            </Link>
            <Link
              to={`/courses/${courseId}/gradebook`}
              className="text-brand-600 hover:underline text-sm"
            >
              SpeedGrader
            </Link>
          </div>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      {/* Top bar */}
      <div className="bg-surface-0 border-b border-border-default px-4 py-2 flex items-center justify-between -mx-4 -mt-4 mb-4">
        <div className="flex items-center gap-4">
          <Link
            to={`/courses/${courseId}/assignments/${assignmentId}`}
            className="flex items-center gap-1 text-sm text-text-secondary hover:text-brand-600 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to SpeedGrader</span>
          </Link>
          <div className="h-5 w-px bg-gray-300" />
          <div>
            <h1 className="text-sm font-semibold text-text-primary truncate max-w-md">
              {assignment?.name || 'Document Viewer'}
            </h1>
            <p className="text-xs text-text-tertiary">
              {submission?.submission_type && (
                <span className="capitalize">{submission.submission_type.replace(/_/g, ' ')}</span>
              )}
              {submission?.submitted_at && (
                <span> &middot; Submitted {new Date(submission.submitted_at).toLocaleString()}</span>
              )}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Zoom controls */}
          <div className="flex items-center gap-1 bg-surface-2 rounded-lg px-1 py-0.5">
            <button
              onClick={handleZoomOut}
              className="p-1 hover:bg-border-default rounded transition-colors"
              title="Zoom out"
              disabled={zoom <= 50}
            >
              <ZoomOut className="w-4 h-4 text-text-secondary" />
            </button>
            <span className="text-xs font-medium text-text-secondary w-10 text-center">{zoom}%</span>
            <button
              onClick={handleZoomIn}
              className="p-1 hover:bg-border-default rounded transition-colors"
              title="Zoom in"
              disabled={zoom >= 200}
            >
              <ZoomIn className="w-4 h-4 text-text-secondary" />
            </button>
            <button
              onClick={handleZoomReset}
              className="p-1 hover:bg-border-default rounded transition-colors"
              title="Reset zoom"
            >
              <RotateCcw className="w-3.5 h-3.5 text-text-secondary" />
            </button>
          </div>

          {/* Page navigation */}
          {totalPages > 1 && (
            <div className="flex items-center gap-1 bg-surface-2 rounded-lg px-1 py-0.5">
              <button
                onClick={handlePrevPage}
                className="p-1 hover:bg-border-default rounded transition-colors"
                disabled={currentPage <= 1}
              >
                <ChevronLeft className="w-4 h-4 text-text-secondary" />
              </button>
              <span className="text-xs font-medium text-text-secondary w-16 text-center">
                {currentPage} / {totalPages}
              </span>
              <button
                onClick={handleNextPage}
                className="p-1 hover:bg-border-default rounded transition-colors"
                disabled={currentPage >= totalPages}
              >
                <ChevronRight className="w-4 h-4 text-text-secondary" />
              </button>
            </div>
          )}

          {isReadOnly && (
            <span className="text-xs bg-surface-2 text-text-secondary px-2 py-1 rounded font-medium">
              View Only
            </span>
          )}
        </div>
      </div>

      {/* Document viewer with annotation support */}
      <div
        style={{
          height: 'calc(100vh - 180px)',
          transform: `scale(${zoom / 100})`,
          transformOrigin: 'top left',
          width: `${10000 / zoom}%`,
        }}
      >
        <DocumentViewer
          submissionId={submission?.id}
          courseId={courseId}
          assignmentId={assignmentId}
          userId={userId}
          readOnly={isReadOnly}
          documentContent={getDocumentContent()}
          documentTitle={getDocumentTitle()}
        />
      </div>
    </Layout>
  );
};

export default DocViewerPage;
