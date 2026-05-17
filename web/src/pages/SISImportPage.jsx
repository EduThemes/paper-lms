import React, { useState, useEffect, useCallback } from 'react';
import { Upload, Download, AlertCircle, CheckCircle, Clock, RefreshCw, X, ChevronDown, FileText } from 'lucide-react';
import Layout from '../components/Layout';
import { getCSRFToken } from '../services/api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';
const ACCOUNT_ID = 1;

const apiFetch = (url, options = {}) => fetch(url, {
  ...options,
  credentials: 'include',
  headers: { 'Content-Type': 'application/json', ...options.headers },
});

const SISImportPage = () => {
  const [activeTab, setActiveTab] = useState('import');
  const [importType, setImportType] = useState('users');
  const [file, setFile] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [batches, setBatches] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [selectedBatch, setSelectedBatch] = useState(null);
  const [batchErrors, setBatchErrors] = useState([]);
  const [loadingErrors, setLoadingErrors] = useState(false);

  const fetchBatches = useCallback(async () => {
    try {
      const response = await apiFetch(`${API_URL}/accounts/${ACCOUNT_ID}/sis_imports?page=1&per_page=50`);
      if (!response.ok) throw new Error('Failed to fetch import history');
      const data = await response.json();
      setBatches(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchBatches();
  }, [fetchBatches]);

  const handleUpload = async (e) => {
    e.preventDefault();
    if (!file) {
      setError('Please select a CSV file to upload');
      return;
    }

    setUploading(true);
    setError(null);
    setSuccess(null);

    try {
      const formData = new FormData();
      formData.append('import_type', importType);
      formData.append('attachment', file);

      const response = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/sis_imports`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'X-CSRF-Token': getCSRFToken() },
        body: formData,
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Upload failed');
      }

      const data = await response.json();
      setSuccess(`SIS import completed. Status: ${data.workflow_state}. Processed ${data.processed_rows} of ${data.total_rows} rows.`);
      setFile(null);
      // Reset file input
      const fileInput = document.getElementById('csv-file-input');
      if (fileInput) fileInput.value = '';
      fetchBatches();
    } catch (err) {
      setError(err.message);
    } finally {
      setUploading(false);
    }
  };

  const handleViewErrors = async (batch) => {
    setSelectedBatch(batch);
    setLoadingErrors(true);
    setBatchErrors([]);

    try {
      const response = await apiFetch(`${API_URL}/accounts/${ACCOUNT_ID}/sis_imports/${batch.id}/errors`);
      if (!response.ok) throw new Error('Failed to fetch errors');
      const data = await response.json();
      setBatchErrors(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoadingErrors(false);
    }
  };

  const handleExport = (type) => {
    const url = `${API_URL}/accounts/${ACCOUNT_ID}/sis_exports/${type}.csv`;
    fetch(url, { credentials: 'include' })
      .then((res) => {
        if (!res.ok) throw new Error('Export failed');
        return res.blob();
      })
      .then((blob) => {
        const downloadUrl = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = downloadUrl;
        a.download = `${type}.csv`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(downloadUrl);
      })
      .catch((err) => setError(err.message));
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const getStateBadge = (state) => {
    switch (state) {
      case 'imported':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-accent-success/20 text-accent-success">
            <CheckCircle className="w-3 h-3" />
            Imported
          </span>
        );
      case 'imported_with_messages':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-accent-warning/20 text-accent-warning">
            <AlertCircle className="w-3 h-3" />
            Imported with warnings
          </span>
        );
      case 'importing':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-brand-100 text-brand-800">
            <Clock className="w-3 h-3" />
            Importing
          </span>
        );
      case 'failed':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-accent-danger/20 text-accent-danger">
            <AlertCircle className="w-3 h-3" />
            Failed
          </span>
        );
      case 'created':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-surface-2 text-text-secondary">
            <Clock className="w-3 h-3" />
            Created
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-surface-2 text-text-secondary">
            {state}
          </span>
        );
    }
  };

  return (
    <Layout>
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-text-primary">SIS Import / Export</h2>
        <p className="text-text-secondary mt-1">
          Import and export data using CSV files for users, courses, sections, and enrollments.
        </p>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <AlertCircle className="w-4 h-4 flex-shrink-0" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {success && (
        <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <CheckCircle className="w-4 h-4 flex-shrink-0" />
          {success}
          <button onClick={() => setSuccess(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-border-default mb-6">
        <nav className="flex space-x-8">
          <button
            onClick={() => setActiveTab('import')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'import'
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <span className="flex items-center gap-2">
              <Upload className="w-4 h-4" />
              Import
            </span>
          </button>
          <button
            onClick={() => setActiveTab('export')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'export'
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <span className="flex items-center gap-2">
              <Download className="w-4 h-4" />
              Export
            </span>
          </button>
        </nav>
      </div>

      {/* Import Tab */}
      {activeTab === 'import' && (
        <div>
          {/* Upload Form */}
          <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
            <h3 className="text-lg font-semibold text-text-primary mb-4">Upload CSV</h3>
            <form onSubmit={handleUpload} className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-text-secondary mb-1">
                    Import Type
                  </label>
                  <div className="relative">
                    <select
                      value={importType}
                      onChange={(e) => setImportType(e.target.value)}
                      className="w-full appearance-none rounded-md border border-border-strong bg-surface-0 px-3 py-2 pr-8 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                    >
                      <option value="users">Users</option>
                      <option value="courses">Courses</option>
                      <option value="sections">Sections</option>
                      <option value="enrollments">Enrollments</option>
                    </select>
                    <ChevronDown className="absolute right-2.5 top-2.5 w-4 h-4 text-text-disabled pointer-events-none" />
                  </div>
                </div>
                <div>
                  <label className="block text-sm font-medium text-text-secondary mb-1">
                    CSV File
                  </label>
                  <input
                    id="csv-file-input"
                    type="file"
                    accept=".csv"
                    onChange={(e) => setFile(e.target.files[0] || null)}
                    className="w-full rounded-md border border-border-strong px-3 py-1.5 text-sm file:mr-3 file:py-1 file:px-3 file:rounded-md file:border-0 file:text-sm file:font-medium file:bg-brand-50 file:text-brand-700 hover:file:bg-brand-100"
                  />
                </div>
              </div>

              <div className="bg-surface-1 rounded-md p-3 text-xs text-text-secondary">
                <p className="font-medium mb-1">Expected CSV columns:</p>
                {importType === 'users' && <p>user_id, login_id, password, first_name, last_name, email, status</p>}
                {importType === 'courses' && <p>course_id, short_name, long_name, account_id, term_id, status</p>}
                {importType === 'sections' && <p>section_id, course_id, name, status</p>}
                {importType === 'enrollments' && <p>course_id, user_id, role, section_id, status</p>}
              </div>

              <div className="flex items-center gap-3">
                <button
                  type="submit"
                  disabled={uploading || !file}
                  className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <Upload className="w-4 h-4" />
                  {uploading ? 'Uploading...' : 'Upload & Import'}
                </button>
                {file && (
                  <span className="text-sm text-text-tertiary flex items-center gap-1">
                    <FileText className="w-4 h-4" />
                    {file.name}
                  </span>
                )}
              </div>
            </form>
          </div>

          {/* Import History */}
          <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
            <div className="px-6 py-4 border-b border-border-default flex items-center justify-between">
              <h3 className="text-lg font-semibold text-text-primary">Import History</h3>
              <button
                onClick={() => { setLoading(true); fetchBatches(); }}
                className="text-text-tertiary hover:text-text-secondary p-1"
                title="Refresh"
              >
                <RefreshCw className="w-4 h-4" />
              </button>
            </div>

            {loading ? (
              <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading import history...
</div>
            ) : batches.length === 0 ? (
              <div className="text-center py-12">
                <Upload className="w-12 h-12 text-text-disabled mx-auto mb-4" />
                <h3 className="text-lg font-medium text-text-primary mb-1">No Imports Yet</h3>
                <p className="text-text-tertiary text-sm">
                  Upload a CSV file above to start importing data.
                </p>
              </div>
            ) : (
              <table className="min-w-full divide-y divide-border-default">
                <thead className="bg-surface-1">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      ID
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Progress
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Rows
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Created
                    </th>
                    <th className="px-6 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-surface-0 divide-y divide-border-default">
                  {batches.map((batch) => (
                    <tr key={batch.id} className="hover:bg-surface-1">
                      <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-text-primary">
                        #{batch.id}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        {getStateBadge(batch.workflow_state)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center gap-2">
                          <div className="w-24 bg-border-default rounded-full h-2">
                            <div
                              className={`h-2 rounded-full ${
                                batch.workflow_state === 'failed'
                                  ? 'bg-accent-danger'
                                  : batch.workflow_state === 'imported'
                                  ? 'bg-accent-success'
                                  : batch.workflow_state === 'imported_with_messages'
                                  ? 'bg-accent-warning'
                                  : 'bg-brand-500'
                              }`}
                              style={{ width: `${batch.progress}%` }}
                            />
                          </div>
                          <span className="text-xs text-text-tertiary">{batch.progress}%</span>
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-text-secondary">
                        {batch.processed_rows} / {batch.total_rows}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">
                        {formatDate(batch.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-right">
                        {(batch.workflow_state === 'imported_with_messages' || batch.workflow_state === 'failed') && (
                          <button
                            onClick={() => handleViewErrors(batch)}
                            className="text-brand-600 hover:text-brand-800 text-sm font-medium"
                          >
                            View Errors
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {/* Error Viewer Modal */}
          {selectedBatch && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
              <div className="bg-surface-0 rounded-lg shadow-xl max-w-3xl w-full mx-4 max-h-[80vh] flex flex-col">
                <div className="px-6 py-4 border-b border-border-default flex items-center justify-between">
                  <h3 className="font-semibold text-text-primary">
                    Errors for Import #{selectedBatch.id}
                  </h3>
                  <button
                    onClick={() => { setSelectedBatch(null); setBatchErrors([]); }}
                    className="text-text-disabled hover:text-text-secondary"
                  >
                    <X className="w-5 h-5" />
                  </button>
                </div>
                <div className="overflow-auto flex-1 p-6">
                  {loadingErrors ? (
                    <div className="text-center py-8 text-text-tertiary">Loading errors...</div>
                  ) : batchErrors.length === 0 ? (
                    <div className="text-center py-8 text-text-tertiary">No errors found for this import.</div>
                  ) : (
                    <table className="min-w-full divide-y divide-border-default">
                      <thead className="bg-surface-1">
                        <tr>
                          <th className="px-4 py-2 text-left text-xs font-medium text-text-tertiary uppercase">
                            Row
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-text-tertiary uppercase">
                            File
                          </th>
                          <th className="px-4 py-2 text-left text-xs font-medium text-text-tertiary uppercase">
                            Message
                          </th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border-default">
                        {batchErrors.map((err, idx) => (
                          <tr key={err.id || idx} className="hover:bg-surface-1">
                            <td className="px-4 py-2 text-sm text-text-primary whitespace-nowrap">
                              {err.row}
                            </td>
                            <td className="px-4 py-2 text-sm text-text-secondary whitespace-nowrap">
                              {err.file || '-'}
                            </td>
                            <td className="px-4 py-2 text-sm text-accent-danger">
                              {err.message}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
                <div className="px-6 py-3 border-t border-border-default flex justify-end">
                  <button
                    onClick={() => { setSelectedBatch(null); setBatchErrors([]); }}
                    className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
                  >
                    Close
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Export Tab */}
      {activeTab === 'export' && (
        <div className="bg-surface-0 rounded-lg shadow p-6">
          <h3 className="text-lg font-semibold text-text-primary mb-2">Export Data as CSV</h3>
          <p className="text-text-secondary text-sm mb-6">
            Download the current data from Paper LMS in SIS-compatible CSV format.
          </p>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <button
              onClick={() => handleExport('users')}
              className="flex flex-col items-center gap-3 p-6 border-2 border-border-default rounded-lg hover:border-brand-500 hover:bg-brand-50 transition-colors"
            >
              <Download className="w-8 h-8 text-brand-600" />
              <div className="text-center">
                <p className="font-medium text-text-primary">Users CSV</p>
                <p className="text-xs text-text-tertiary mt-1">user_id, login_id, name, email</p>
              </div>
            </button>

            <button
              onClick={() => handleExport('courses')}
              className="flex flex-col items-center gap-3 p-6 border-2 border-border-default rounded-lg hover:border-brand-500 hover:bg-brand-50 transition-colors"
            >
              <Download className="w-8 h-8 text-accent-success" />
              <div className="text-center">
                <p className="font-medium text-text-primary">Courses CSV</p>
                <p className="text-xs text-text-tertiary mt-1">course_id, name, code, account</p>
              </div>
            </button>

            <button
              onClick={() => handleExport('sections')}
              className="flex flex-col items-center gap-3 p-6 border-2 border-border-default rounded-lg hover:border-brand-500 hover:bg-brand-50 transition-colors"
            >
              <Download className="w-8 h-8 text-purple-600" />
              <div className="text-center">
                <p className="font-medium text-text-primary">Sections CSV</p>
                <p className="text-xs text-text-tertiary mt-1">section_id, course_id, name</p>
              </div>
            </button>

            <button
              onClick={() => handleExport('enrollments')}
              className="flex flex-col items-center gap-3 p-6 border-2 border-border-default rounded-lg hover:border-brand-500 hover:bg-brand-50 transition-colors"
            >
              <Download className="w-8 h-8 text-orange-600" />
              <div className="text-center">
                <p className="font-medium text-text-primary">Enrollments CSV</p>
                <p className="text-xs text-text-tertiary mt-1">course_id, user_id, role, section</p>
              </div>
            </button>
          </div>
        </div>
      )}
    </Layout>
  );
};

export default SISImportPage;
