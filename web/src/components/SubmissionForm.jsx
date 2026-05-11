import React, { useState } from 'react';
import { Send, Clock, CheckCircle, Upload, Link as LinkIcon, FileText, Download, Paperclip } from 'lucide-react';
import { sanitizeHTML } from './RichContentViewer';

const SubmissionForm = ({ courseId, assignmentId, existingSubmission, onSubmit, submissionTypes = ['online_text_entry'] }) => {
  const types = Array.isArray(submissionTypes)
    ? submissionTypes.map(t => t.trim())
    : String(submissionTypes).split(',').map(t => t.trim());

  const hasText = types.includes('online_text_entry');
  const hasUpload = types.includes('online_upload');
  const hasUrl = types.includes('online_url');
  const submittableTypes = [hasText && 'text', hasUpload && 'file', hasUrl && 'url'].filter(Boolean);

  const [activeTab, setActiveTab] = useState(submittableTypes[0] || 'text');
  const [body, setBody] = useState('');
  const [url, setUrl] = useState('');
  const [file, setFile] = useState(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess(false);
    try {
      if (activeTab === 'text') {
        if (!body.trim()) return;
        await onSubmit({ submission_type: 'online_text_entry', body });
        setBody('');
      } else if (activeTab === 'url') {
        if (!url.trim()) return;
        await onSubmit({ submission_type: 'online_url', url });
        setUrl('');
      } else if (activeTab === 'file') {
        if (!file) return;
        await onSubmit({ submission_type: 'online_upload', file });
        setFile(null);
      }
      setSuccess(true);
      setTimeout(() => setSuccess(false), 5000);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const isValid = () => {
    if (activeTab === 'text') return body.trim().length > 0;
    if (activeTab === 'url') return url.trim().length > 0;
    if (activeTab === 'file') return file !== null;
    return false;
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleString();
  };

  const tabLabel = { text: 'Text Entry', file: 'File Upload', url: 'Website URL' };
  const tabIcon = { text: FileText, file: Upload, url: LinkIcon };

  return (
    <div className="space-y-4">
      {existingSubmission && existingSubmission.workflow_state !== 'unsubmitted' && (
        <div className="bg-surface-1 rounded-lg p-4 border">
          <div className="flex items-center justify-between mb-2">
            <h4 className="font-medium text-text-primary flex items-center space-x-2">
              <CheckCircle className="w-4 h-4 text-accent-success" />
              <span>Previous Submission</span>
            </h4>
            {existingSubmission.attempt && (
              <span className="text-xs text-text-tertiary bg-border-default px-2 py-1 rounded">
                Attempt {existingSubmission.attempt}
              </span>
            )}
          </div>
          {existingSubmission.submitted_at && (
            <div className="flex items-center space-x-1 text-sm text-text-tertiary mb-2">
              <Clock className="w-3 h-3" />
              <span>Submitted {formatDate(existingSubmission.submitted_at)}</span>
            </div>
          )}
          {existingSubmission.body && (
            <div
              className="text-sm text-text-secondary prose max-w-none bg-surface-0 p-3 rounded border"
              dangerouslySetInnerHTML={{ __html: sanitizeHTML(existingSubmission.body) }}
            />
          )}
          {existingSubmission.url && (
            <div className="text-sm text-text-secondary bg-surface-0 p-3 rounded border">
              <a href={existingSubmission.url} target="_blank" rel="noopener noreferrer" className="text-brand-600 hover:underline">
                {existingSubmission.url}
              </a>
            </div>
          )}
          {existingSubmission.attachments && existingSubmission.attachments.length > 0 && (
            <div className="bg-surface-0 p-3 rounded border">
              <p className="text-xs font-medium text-text-tertiary mb-2 flex items-center gap-1">
                <Paperclip className="w-3 h-3" /> Attached Files
              </p>
              <div className="space-y-1.5">
                {existingSubmission.attachments.map((att) => (
                  <a
                    key={att.id}
                    href={att.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="flex items-center gap-2 text-sm text-brand-600 hover:underline"
                  >
                    <Download className="w-3.5 h-3.5 flex-shrink-0" />
                    <span>{att.display_name || att.filename}</span>
                    {att.size && (
                      <span className="text-xs text-text-disabled">
                        ({att.size >= 1048576
                          ? `${(att.size / 1048576).toFixed(1)} MB`
                          : `${(att.size / 1024).toFixed(1)} KB`})
                      </span>
                    )}
                  </a>
                ))}
              </div>
            </div>
          )}
          {existingSubmission.grade !== null && existingSubmission.grade !== undefined && (
            <div className="mt-2 text-sm">
              <span className="font-medium text-text-secondary">Grade: </span>
              <span className="text-brand-600 font-semibold">{existingSubmission.grade}</span>
              {existingSubmission.score !== null && existingSubmission.score !== undefined && (
                <span className="text-text-tertiary"> ({existingSubmission.score} pts)</span>
              )}
            </div>
          )}
        </div>
      )}

      {/* Submission type tabs */}
      {submittableTypes.length > 1 && (
        <div className="flex border-b border-border-default">
          {submittableTypes.map((tab) => {
            const Icon = tabIcon[tab];
            return (
              <button
                key={tab}
                type="button"
                onClick={() => setActiveTab(tab)}
                className={`flex items-center gap-2 px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === tab
                    ? 'border-brand-600 text-brand-600'
                    : 'border-transparent text-text-tertiary hover:text-text-secondary'
                }`}
              >
                <Icon className="w-4 h-4" />
                {tabLabel[tab]}
              </button>
            );
          })}
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <label className="block text-sm font-medium text-text-secondary mb-1">
          {existingSubmission && existingSubmission.workflow_state !== 'unsubmitted'
            ? 'Resubmit'
            : 'Your Submission'}
        </label>

        {activeTab === 'text' && (
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={6}
            className="w-full border border-border-strong rounded-lg p-3 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500 resize-y"
            placeholder="Enter your submission text here..."
            disabled={submitting}
          />
        )}

        {activeTab === 'url' && (
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            className="w-full border border-border-strong rounded-lg p-3 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
            placeholder="https://example.com"
            disabled={submitting}
          />
        )}

        {activeTab === 'file' && (
          <div className="border-2 border-dashed border-border-strong rounded-lg p-6 text-center hover:border-blue-400 transition-colors">
            {file ? (
              <div className="flex items-center justify-center gap-3">
                <Upload className="w-5 h-5 text-brand-600" />
                <span className="text-sm text-text-primary font-medium">{file.name}</span>
                <span className="text-xs text-text-disabled">({(file.size / 1024).toFixed(1)} KB)</span>
                <button
                  type="button"
                  onClick={() => setFile(null)}
                  className="text-accent-danger hover:text-accent-danger text-sm ml-2"
                >
                  Remove
                </button>
              </div>
            ) : (
              <label className="cursor-pointer">
                <Upload className="w-8 h-8 text-text-disabled mx-auto mb-2" />
                <p className="text-sm text-text-secondary">Click to choose a file or drag and drop</p>
                <p className="text-xs text-text-disabled mt-1">PDF, Word, images, or other supported formats</p>
                <input
                  type="file"
                  onChange={(e) => setFile(e.target.files?.[0] || null)}
                  className="hidden"
                  disabled={submitting}
                />
              </label>
            )}
          </div>
        )}

        {success && (
          <div className="flex items-center gap-2 text-accent-success bg-accent-success/10 border border-accent-success/30 rounded-md p-3 text-sm">
            <CheckCircle className="w-4 h-4 flex-shrink-0" />
            <span>Submission recorded successfully{existingSubmission?.attempt ? ` (Attempt ${(existingSubmission.attempt || 0) + 1})` : ''}!</span>
          </div>
        )}
        {error && (
          <p className="text-accent-danger text-sm mt-1">{error}</p>
        )}
        <div className="flex items-center justify-between mt-2">
          {existingSubmission && existingSubmission.attempt > 0 && (
            <span className="text-xs text-text-disabled">
              Current attempt: {existingSubmission.attempt}
            </span>
          )}
          <button
            type="submit"
            disabled={submitting || !isValid()}
            className="inline-flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium ml-auto"
          >
            <Send className="w-4 h-4" />
            <span>{submitting ? 'Submitting...' : 'Submit Assignment'}</span>
          </button>
        </div>
      </form>
    </div>
  );
};

export default SubmissionForm;
