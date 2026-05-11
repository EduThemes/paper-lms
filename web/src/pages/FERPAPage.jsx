import React, { useState, useEffect, useCallback } from 'react';
import { Shield, Trash2, Download, Clock, Plus, Edit2, Check, X, AlertTriangle, FileText, RefreshCw, Search, ToggleLeft, ToggleRight } from 'lucide-react';
import Layout from '../components/Layout';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';

const TAB_DELETION = 'deletion';
const TAB_RETENTION = 'retention';
const TAB_EXPORT = 'export';

const STATUS_COLORS = {
  pending: 'bg-accent-warning/20 text-accent-warning',
  approved: 'bg-brand-100 text-brand-800',
  completed: 'bg-accent-success/20 text-accent-success',
  processing: 'bg-indigo-100 text-indigo-800',
  failed: 'bg-accent-danger/20 text-accent-danger',
  ready: 'bg-accent-success/20 text-accent-success',
};

const formatDate = (dateStr) => {
  if (!dateStr) return '--';
  const d = new Date(dateStr);
  return d.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const FERPAPage = () => {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState(TAB_DELETION);
  const [error, setError] = useState(null);
  const [successMessage, setSuccessMessage] = useState(null);

  // ── Deletion Requests State ──
  const [deletionRequests, setDeletionRequests] = useState([]);
  const [deletionLoading, setDeletionLoading] = useState(false);
  const [deletionPage, setDeletionPage] = useState(1);
  const [deletionHasMore, setDeletionHasMore] = useState(false);
  const [approvingId, setApprovingId] = useState(null);

  // ── Retention Policies State ──
  const [policies, setPolicies] = useState([]);
  const [policiesLoading, setPoliciesLoading] = useState(false);
  const [policiesPage, setPoliciesPage] = useState(1);
  const [policiesHasMore, setPoliciesHasMore] = useState(false);
  const [showPolicyForm, setShowPolicyForm] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState(null);
  const [policyForm, setPolicyForm] = useState({
    name: '',
    data_type: '',
    retention_period_days: '',
    description: '',
    active: true,
  });

  // ── Data Export State ──
  const [exportUserId, setExportUserId] = useState('');
  const [exportLoading, setExportLoading] = useState(false);
  const [exportRequests, setExportRequests] = useState([]);
  const [pollingExportId, setPollingExportId] = useState(null);
  const [pollingUserId, setPollingUserId] = useState(null);

  // ── Auto-dismiss success messages ──
  useEffect(() => {
    if (successMessage) {
      const timer = setTimeout(() => setSuccessMessage(null), 4000);
      return () => clearTimeout(timer);
    }
  }, [successMessage]);

  // ── Fetch Deletion Requests ──
  const fetchDeletionRequests = useCallback(async (page = 1) => {
    setDeletionLoading(true);
    setError(null);
    try {
      const result = await api.getPendingDeletionRequests(page, 20);
      setDeletionRequests(result.data || []);
      setDeletionHasMore(!!result.pagination?.next);
      setDeletionPage(page);
    } catch (err) {
      setError(err.message);
      setDeletionRequests([]);
    } finally {
      setDeletionLoading(false);
    }
  }, []);

  // ── Fetch Retention Policies ──
  const fetchPolicies = useCallback(async (page = 1) => {
    setPoliciesLoading(true);
    setError(null);
    try {
      const result = await api.getRetentionPolicies(page, 20);
      setPolicies(result.data || []);
      setPoliciesHasMore(!!result.pagination?.next);
      setPoliciesPage(page);
    } catch (err) {
      setError(err.message);
      setPolicies([]);
    } finally {
      setPoliciesLoading(false);
    }
  }, []);

  // ── Load data when tab changes ──
  useEffect(() => {
    if (activeTab === TAB_DELETION) {
      fetchDeletionRequests(1);
    } else if (activeTab === TAB_RETENTION) {
      fetchPolicies(1);
    }
  }, [activeTab, fetchDeletionRequests, fetchPolicies]);

  // ── Poll export status ──
  useEffect(() => {
    if (!pollingExportId || !pollingUserId) return;
    const interval = setInterval(async () => {
      try {
        const result = await api.getDataExportRequest(pollingUserId, pollingExportId);
        setExportRequests((prev) =>
          prev.map((r) => (r.id === pollingExportId ? result : r))
        );
        if (result.status === 'completed' || result.status === 'ready' || result.status === 'failed') {
          setPollingExportId(null);
          setPollingUserId(null);
          if (result.status === 'completed' || result.status === 'ready') {
            setSuccessMessage('Data export is ready for download.');
          }
        }
      } catch {
        // Polling error, stop polling
        setPollingExportId(null);
        setPollingUserId(null);
      }
    }, 3000);
    return () => clearInterval(interval);
  }, [pollingExportId, pollingUserId]);

  // ── Approve Deletion Request ──
  const handleApproveDeletion = async (requestId) => {
    if (!window.confirm('Are you sure you want to approve this data deletion request? This action cannot be undone. All user data will be permanently removed.')) {
      return;
    }
    setApprovingId(requestId);
    setError(null);
    try {
      await api.approveDeletionRequest(requestId);
      setSuccessMessage('Deletion request approved successfully.');
      fetchDeletionRequests(deletionPage);
    } catch (err) {
      setError(err.message);
    } finally {
      setApprovingId(null);
    }
  };

  // ── Retention Policy CRUD ──
  const resetPolicyForm = () => {
    setPolicyForm({
      name: '',
      data_type: '',
      retention_period_days: '',
      description: '',
      active: true,
    });
    setEditingPolicy(null);
    setShowPolicyForm(false);
  };

  const handleEditPolicy = (policy) => {
    setEditingPolicy(policy);
    setPolicyForm({
      name: policy.name || '',
      data_type: policy.data_type || '',
      retention_period_days: policy.retention_period_days || '',
      description: policy.description || '',
      active: policy.active !== undefined ? policy.active : true,
    });
    setShowPolicyForm(true);
  };

  const handleSubmitPolicy = async (e) => {
    e.preventDefault();
    setError(null);

    const payload = {
      name: policyForm.name,
      data_type: policyForm.data_type,
      retention_period_days: parseInt(policyForm.retention_period_days, 10),
      description: policyForm.description,
      active: policyForm.active,
    };

    try {
      if (editingPolicy) {
        await api.updateRetentionPolicy(editingPolicy.id, payload);
        setSuccessMessage('Retention policy updated successfully.');
      } else {
        await api.createRetentionPolicy(payload);
        setSuccessMessage('Retention policy created successfully.');
      }
      resetPolicyForm();
      fetchPolicies(policiesPage);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDeletePolicy = async (policyId) => {
    if (!window.confirm('Are you sure you want to delete this retention policy?')) return;
    setError(null);
    try {
      await api.deleteRetentionPolicy(policyId);
      setSuccessMessage('Retention policy deleted.');
      fetchPolicies(policiesPage);
    } catch (err) {
      setError(err.message);
    }
  };

  // ── Data Export ──
  const handleCreateExport = async (e) => {
    e.preventDefault();
    if (!exportUserId.trim()) return;
    setExportLoading(true);
    setError(null);
    try {
      const result = await api.createDataExportRequest(exportUserId.trim());
      setExportRequests((prev) => [result, ...prev]);
      setPollingExportId(result.id);
      setPollingUserId(exportUserId.trim());
      setSuccessMessage('Data export request created. Processing will begin shortly.');
      setExportUserId('');
    } catch (err) {
      setError(err.message);
    } finally {
      setExportLoading(false);
    }
  };

  const getStatusBadge = (status) => {
    const colorClass = STATUS_COLORS[status] || 'bg-surface-2 text-text-primary';
    return (
      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${colorClass}`}>
        {status}
      </span>
    );
  };

  return (
    <Layout>
      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center space-x-3">
          <Shield className="w-8 h-8 text-brand-600" aria-hidden="true" />
          <div>
            <h1 className="text-2xl font-bold text-text-primary">FERPA Compliance</h1>
            <p className="text-sm text-text-tertiary">
              Manage data privacy in compliance with the Family Educational Rights and Privacy Act
            </p>
          </div>
        </div>
      </div>

      {/* Error Alert */}
      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger p-3 rounded-md mb-4 flex items-center justify-between" role="alert">
          <div className="flex items-center space-x-2">
            <AlertTriangle className="w-4 h-4 flex-shrink-0" aria-hidden="true" />
            <span className="text-sm">{error}</span>
          </div>
          <button onClick={() => setError(null)} className="text-accent-danger hover:text-accent-danger text-sm" aria-label="Dismiss error">
            <X className="w-4 h-4" aria-hidden="true" />
          </button>
        </div>
      )}

      {/* Success Alert */}
      {successMessage && (
        <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success p-3 rounded-md mb-4 flex items-center justify-between" role="status">
          <div className="flex items-center space-x-2">
            <Check className="w-4 h-4 flex-shrink-0" aria-hidden="true" />
            <span className="text-sm">{successMessage}</span>
          </div>
          <button onClick={() => setSuccessMessage(null)} className="text-accent-success hover:text-accent-success text-sm" aria-label="Dismiss message">
            <X className="w-4 h-4" aria-hidden="true" />
          </button>
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-border-default mb-6" role="tablist" aria-label="FERPA compliance tabs">
        <nav className="flex space-x-8">
          <button
            role="tab"
            aria-selected={activeTab === TAB_DELETION}
            aria-controls="panel-deletion"
            onClick={() => setActiveTab(TAB_DELETION)}
            className={`pb-3 px-1 border-b-2 font-medium text-sm transition-colors flex items-center space-x-2 ${
              activeTab === TAB_DELETION
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <Trash2 className="w-4 h-4" aria-hidden="true" />
            <span>Data Deletion Requests</span>
          </button>
          <button
            role="tab"
            aria-selected={activeTab === TAB_RETENTION}
            aria-controls="panel-retention"
            onClick={() => setActiveTab(TAB_RETENTION)}
            className={`pb-3 px-1 border-b-2 font-medium text-sm transition-colors flex items-center space-x-2 ${
              activeTab === TAB_RETENTION
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <Clock className="w-4 h-4" aria-hidden="true" />
            <span>Retention Policies</span>
          </button>
          <button
            role="tab"
            aria-selected={activeTab === TAB_EXPORT}
            aria-controls="panel-export"
            onClick={() => setActiveTab(TAB_EXPORT)}
            className={`pb-3 px-1 border-b-2 font-medium text-sm transition-colors flex items-center space-x-2 ${
              activeTab === TAB_EXPORT
                ? 'border-brand-500 text-brand-600'
                : 'border-transparent text-text-tertiary hover:text-text-secondary hover:border-border-strong'
            }`}
          >
            <Download className="w-4 h-4" aria-hidden="true" />
            <span>Data Export</span>
          </button>
        </nav>
      </div>

      {/* ═══════════════════════════════════════════════════════ */}
      {/* Tab 1: Data Deletion Requests                         */}
      {/* ═══════════════════════════════════════════════════════ */}
      {activeTab === TAB_DELETION && (
        <div id="panel-deletion" role="tabpanel" aria-labelledby="tab-deletion">
          <div className="mb-4 flex items-center justify-between">
            <p className="text-sm text-text-secondary">
              Review and approve pending data deletion requests from users exercising their FERPA right to request deletion of educational records.
            </p>
            <button
              onClick={() => fetchDeletionRequests(deletionPage)}
              className="flex items-center space-x-1 text-sm text-text-tertiary hover:text-text-secondary"
              aria-label="Refresh deletion requests"
            >
              <RefreshCw className="w-4 h-4" aria-hidden="true" />
              <span>Refresh</span>
            </button>
          </div>

          {deletionLoading ? (
            <div className="text-center py-12 text-text-tertiary" role="status" aria-live="polite">
              Loading deletion requests...
            </div>
          ) : deletionRequests.length === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow-sm p-8 text-center">
              <Trash2 className="w-12 h-12 text-text-disabled mx-auto mb-3" aria-hidden="true" />
              <p className="text-text-tertiary font-medium">No pending deletion requests</p>
              <p className="text-text-disabled text-sm mt-1">
                Deletion requests will appear here when users submit them.
              </p>
            </div>
          ) : (
            <div className="bg-surface-0 rounded-lg shadow-sm overflow-hidden">
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-border-default" aria-label="Data deletion requests">
                  <thead className="bg-surface-1">
                    <tr>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">ID</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">User</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Reason</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Status</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Requested</th>
                      <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="bg-surface-0 divide-y divide-border-default">
                    {deletionRequests.map((req) => (
                      <tr key={req.id} className="hover:bg-surface-1">
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary font-mono">
                          #{req.id}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap">
                          <div className="text-sm font-medium text-text-primary">
                            {req.user_name || req.user_email || `User ${req.user_id}`}
                          </div>
                          {req.user_email && req.user_name && (
                            <div className="text-xs text-text-tertiary">{req.user_email}</div>
                          )}
                          <div className="text-xs text-text-disabled">ID: {req.user_id}</div>
                        </td>
                        <td className="px-4 py-3 text-sm text-text-secondary max-w-xs">
                          <p className="truncate" title={req.reason}>
                            {req.reason || 'No reason provided'}
                          </p>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap">
                          {getStatusBadge(req.status || 'pending')}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary">
                          {formatDate(req.created_at)}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-right">
                          {(req.status === 'pending' || !req.status) && (
                            <button
                              onClick={() => handleApproveDeletion(req.id)}
                              disabled={approvingId === req.id}
                              className="inline-flex items-center space-x-1 px-3 py-1.5 bg-accent-danger text-white text-sm rounded-md hover:bg-accent-danger/90 disabled:opacity-50 disabled:cursor-not-allowed transition-colors focus:outline-none focus:ring-2 focus:ring-accent-danger focus:ring-offset-2"
                              aria-label={`Approve deletion request #${req.id}`}
                            >
                              {approvingId === req.id ? (
                                <>
                                  <RefreshCw className="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
                                  <span>Approving...</span>
                                </>
                              ) : (
                                <>
                                  <Check className="w-3.5 h-3.5" aria-hidden="true" />
                                  <span>Approve</span>
                                </>
                              )}
                            </button>
                          )}
                          {req.status === 'approved' && (
                            <span className="text-sm text-brand-600 flex items-center justify-end space-x-1">
                              <Check className="w-3.5 h-3.5" aria-hidden="true" />
                              <span>Approved</span>
                            </span>
                          )}
                          {req.status === 'completed' && (
                            <span className="text-sm text-accent-success flex items-center justify-end space-x-1">
                              <Check className="w-3.5 h-3.5" aria-hidden="true" />
                              <span>Completed</span>
                            </span>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div className="px-4 py-3 bg-surface-1 border-t border-border-default flex items-center justify-between">
                <button
                  onClick={() => fetchDeletionRequests(deletionPage - 1)}
                  disabled={deletionPage <= 1}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <span className="text-sm text-text-secondary">Page {deletionPage}</span>
                <button
                  onClick={() => fetchDeletionRequests(deletionPage + 1)}
                  disabled={!deletionHasMore}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* ═══════════════════════════════════════════════════════ */}
      {/* Tab 2: Retention Policies                              */}
      {/* ═══════════════════════════════════════════════════════ */}
      {activeTab === TAB_RETENTION && (
        <div id="panel-retention" role="tabpanel" aria-labelledby="tab-retention">
          <div className="mb-4 flex items-center justify-between">
            <p className="text-sm text-text-secondary">
              Configure how long different types of educational data are retained before automatic purging.
            </p>
            <button
              onClick={() => { resetPolicyForm(); setShowPolicyForm(!showPolicyForm); }}
              className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              aria-expanded={showPolicyForm}
            >
              <Plus className="w-4 h-4" aria-hidden="true" />
              <span>New Policy</span>
            </button>
          </div>

          {/* Create / Edit Policy Form */}
          {showPolicyForm && (
            <div className="bg-surface-0 rounded-lg shadow-sm p-6 mb-6 border border-border-default" role="region" aria-label={editingPolicy ? 'Edit retention policy' : 'Create retention policy'}>
              <h2 className="text-lg font-semibold text-text-primary mb-4">
                {editingPolicy ? 'Edit Retention Policy' : 'Create Retention Policy'}
              </h2>
              <form onSubmit={handleSubmitPolicy} className="space-y-4">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="policy-name" className="block text-sm font-medium text-text-secondary mb-1">
                      Policy Name <span className="text-accent-danger">*</span>
                    </label>
                    <input
                      id="policy-name"
                      type="text"
                      value={policyForm.name}
                      onChange={(e) => setPolicyForm({ ...policyForm, name: e.target.value })}
                      className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                      placeholder="e.g. Student Grades Retention"
                      required
                      aria-required="true"
                    />
                  </div>
                  <div>
                    <label htmlFor="policy-data-type" className="block text-sm font-medium text-text-secondary mb-1">
                      Data Type <span className="text-accent-danger">*</span>
                    </label>
                    <input
                      id="policy-data-type"
                      type="text"
                      value={policyForm.data_type}
                      onChange={(e) => setPolicyForm({ ...policyForm, data_type: e.target.value })}
                      className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                      placeholder="e.g. grades, submissions, enrollments"
                      required
                      aria-required="true"
                    />
                  </div>
                  <div>
                    <label htmlFor="policy-retention-days" className="block text-sm font-medium text-text-secondary mb-1">
                      Retention Period (days) <span className="text-accent-danger">*</span>
                    </label>
                    <input
                      id="policy-retention-days"
                      type="number"
                      min="1"
                      value={policyForm.retention_period_days}
                      onChange={(e) => setPolicyForm({ ...policyForm, retention_period_days: e.target.value })}
                      className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                      placeholder="e.g. 2555 (7 years)"
                      required
                      aria-required="true"
                    />
                  </div>
                  <div className="flex items-end">
                    <div className="flex items-center space-x-3 pb-2">
                      <button
                        type="button"
                        onClick={() => setPolicyForm({ ...policyForm, active: !policyForm.active })}
                        className="flex-shrink-0 focus:outline-none focus:ring-2 focus:ring-brand-500 rounded"
                        role="switch"
                        aria-checked={policyForm.active}
                        aria-label="Toggle policy active status"
                      >
                        {policyForm.active ? (
                          <ToggleRight className="w-8 h-5 text-brand-600" aria-hidden="true" />
                        ) : (
                          <ToggleLeft className="w-8 h-5 text-text-disabled" aria-hidden="true" />
                        )}
                      </button>
                      <span className="text-sm font-medium text-text-secondary">
                        {policyForm.active ? 'Active' : 'Inactive'}
                      </span>
                    </div>
                  </div>
                </div>
                <div>
                  <label htmlFor="policy-description" className="block text-sm font-medium text-text-secondary mb-1">
                    Description
                  </label>
                  <textarea
                    id="policy-description"
                    value={policyForm.description}
                    onChange={(e) => setPolicyForm({ ...policyForm, description: e.target.value })}
                    className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                    rows={3}
                    placeholder="Describe what data this policy covers and why it is retained for this duration."
                  />
                </div>
                <div className="flex items-center space-x-3">
                  <button
                    type="submit"
                    className="bg-brand-600 text-white px-4 py-2 rounded-md text-sm hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
                  >
                    {editingPolicy ? 'Update Policy' : 'Create Policy'}
                  </button>
                  <button
                    type="button"
                    onClick={resetPolicyForm}
                    className="text-text-secondary hover:text-text-primary text-sm px-4 py-2"
                  >
                    Cancel
                  </button>
                </div>
              </form>
            </div>
          )}

          {/* Policies List */}
          {policiesLoading ? (
            <div className="text-center py-12 text-text-tertiary" role="status" aria-live="polite">
              Loading retention policies...
            </div>
          ) : policies.length === 0 ? (
            <div className="bg-surface-0 rounded-lg shadow-sm p-8 text-center">
              <Clock className="w-12 h-12 text-text-disabled mx-auto mb-3" aria-hidden="true" />
              <p className="text-text-tertiary font-medium">No retention policies configured</p>
              <p className="text-text-disabled text-sm mt-1">
                Create retention policies to define how long different types of data should be kept.
              </p>
            </div>
          ) : (
            <div className="bg-surface-0 rounded-lg shadow-sm overflow-hidden">
              <div className="overflow-x-auto">
                <table className="min-w-full divide-y divide-border-default" aria-label="Retention policies">
                  <thead className="bg-surface-1">
                    <tr>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Name</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Data Type</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Retention Period</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Status</th>
                      <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Description</th>
                      <th scope="col" className="px-4 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="bg-surface-0 divide-y divide-border-default">
                    {policies.map((policy) => (
                      <tr key={policy.id} className="hover:bg-surface-1">
                        <td className="px-4 py-3 whitespace-nowrap text-sm font-medium text-text-primary">
                          {policy.name}
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap">
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800">
                            {policy.data_type}
                          </span>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary">
                          <div className="flex items-center space-x-1">
                            <Clock className="w-3.5 h-3.5 text-text-disabled" aria-hidden="true" />
                            <span>{policy.retention_period_days} days</span>
                            {policy.retention_period_days >= 365 && (
                              <span className="text-text-disabled text-xs">
                                ({(policy.retention_period_days / 365).toFixed(1)} years)
                              </span>
                            )}
                          </div>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap">
                          {policy.active ? (
                            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-accent-success/20 text-accent-success">
                              Active
                            </span>
                          ) : (
                            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-surface-2 text-text-secondary">
                              Inactive
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-sm text-text-tertiary max-w-xs">
                          <p className="truncate" title={policy.description}>
                            {policy.description || '--'}
                          </p>
                        </td>
                        <td className="px-4 py-3 whitespace-nowrap text-right">
                          <div className="flex items-center justify-end space-x-2">
                            <button
                              onClick={() => handleEditPolicy(policy)}
                              className="text-text-disabled hover:text-brand-600 p-1 rounded focus:outline-none focus:ring-2 focus:ring-brand-500"
                              aria-label={`Edit policy ${policy.name}`}
                              title="Edit policy"
                            >
                              <Edit2 className="w-4 h-4" aria-hidden="true" />
                            </button>
                            <button
                              onClick={() => handleDeletePolicy(policy.id)}
                              className="text-text-disabled hover:text-accent-danger p-1 rounded focus:outline-none focus:ring-2 focus:ring-accent-danger"
                              aria-label={`Delete policy ${policy.name}`}
                              title="Delete policy"
                            >
                              <Trash2 className="w-4 h-4" aria-hidden="true" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div className="px-4 py-3 bg-surface-1 border-t border-border-default flex items-center justify-between">
                <button
                  onClick={() => fetchPolicies(policiesPage - 1)}
                  disabled={policiesPage <= 1}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Previous
                </button>
                <span className="text-sm text-text-secondary">Page {policiesPage}</span>
                <button
                  onClick={() => fetchPolicies(policiesPage + 1)}
                  disabled={!policiesHasMore}
                  className="px-3 py-1 text-sm border border-border-strong rounded hover:bg-surface-2 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Next
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* ═══════════════════════════════════════════════════════ */}
      {/* Tab 3: Data Export                                      */}
      {/* ═══════════════════════════════════════════════════════ */}
      {activeTab === TAB_EXPORT && (
        <div id="panel-export" role="tabpanel" aria-labelledby="tab-export">
          <div className="mb-4">
            <p className="text-sm text-text-secondary">
              Generate data export packages for users exercising their FERPA right to inspect and review their educational records.
            </p>
          </div>

          {/* Export Request Form */}
          <div className="bg-surface-0 rounded-lg shadow-sm p-6 mb-6 border border-border-default">
            <h2 className="text-lg font-semibold text-text-primary mb-4 flex items-center space-x-2">
              <Download className="w-5 h-5 text-brand-600" aria-hidden="true" />
              <span>Create Data Export Request</span>
            </h2>
            <form onSubmit={handleCreateExport} className="flex items-end space-x-4">
              <div className="flex-1 max-w-sm">
                <label htmlFor="export-user-id" className="block text-sm font-medium text-text-secondary mb-1">
                  User ID <span className="text-accent-danger">*</span>
                </label>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-disabled" aria-hidden="true" />
                  <input
                    id="export-user-id"
                    type="text"
                    value={exportUserId}
                    onChange={(e) => setExportUserId(e.target.value)}
                    className="w-full pl-10 pr-4 py-2 border border-border-strong rounded-md text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                    placeholder="Enter user ID"
                    required
                    aria-required="true"
                  />
                </div>
              </div>
              <button
                type="submit"
                disabled={exportLoading || !exportUserId.trim()}
                className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                {exportLoading ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin" aria-hidden="true" />
                    <span>Creating...</span>
                  </>
                ) : (
                  <>
                    <Download className="w-4 h-4" aria-hidden="true" />
                    <span>Request Export</span>
                  </>
                )}
              </button>
            </form>
            <p className="mt-2 text-xs text-text-disabled">
              This will generate a comprehensive data package containing all educational records for the specified user.
            </p>
          </div>

          {/* Export Requests List */}
          <div>
            <h3 className="text-sm font-medium text-text-secondary mb-3">Recent Export Requests</h3>
            {exportRequests.length === 0 ? (
              <div className="bg-surface-0 rounded-lg shadow-sm p-8 text-center">
                <FileText className="w-12 h-12 text-text-disabled mx-auto mb-3" aria-hidden="true" />
                <p className="text-text-tertiary font-medium">No export requests yet</p>
                <p className="text-text-disabled text-sm mt-1">
                  Submit a user ID above to create a data export request.
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {exportRequests.map((exportReq) => (
                  <div
                    key={exportReq.id}
                    className="bg-surface-0 rounded-lg shadow-sm border border-border-default p-4 flex items-center justify-between"
                  >
                    <div className="flex items-center space-x-4">
                      <div className="flex-shrink-0">
                        <FileText className="w-8 h-8 text-text-disabled" aria-hidden="true" />
                      </div>
                      <div>
                        <div className="flex items-center space-x-2">
                          <span className="text-sm font-medium text-text-primary">
                            Export #{exportReq.id}
                          </span>
                          {getStatusBadge(exportReq.status || 'processing')}
                        </div>
                        <div className="text-xs text-text-tertiary mt-0.5">
                          User ID: {exportReq.user_id} | Created: {formatDate(exportReq.created_at)}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      {(exportReq.status === 'processing' || exportReq.status === 'pending') && (
                        <div className="flex items-center space-x-1 text-sm text-indigo-600">
                          <RefreshCw className="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
                          <span>Processing...</span>
                        </div>
                      )}
                      {(exportReq.status === 'completed' || exportReq.status === 'ready') && exportReq.download_url && (
                        <a
                          href={exportReq.download_url}
                          className="inline-flex items-center space-x-1 px-3 py-1.5 bg-accent-success text-white text-sm rounded-md hover:bg-accent-success/90 transition-colors focus:outline-none focus:ring-2 focus:ring-accent-success focus:ring-offset-2"
                          target="_blank"
                          rel="noopener noreferrer"
                          aria-label={`Download export #${exportReq.id}`}
                        >
                          <Download className="w-3.5 h-3.5" aria-hidden="true" />
                          <span>Download</span>
                        </a>
                      )}
                      {exportReq.status === 'failed' && (
                        <span className="text-sm text-accent-danger flex items-center space-x-1">
                          <AlertTriangle className="w-3.5 h-3.5" aria-hidden="true" />
                          <span>Failed</span>
                        </span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </Layout>
  );
};

export default FERPAPage;
