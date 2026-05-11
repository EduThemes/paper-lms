import React, { useState, useEffect } from 'react';
import { Calendar, Plus, Edit2, Trash2, Clock, CheckCircle } from 'lucide-react';
import { api } from '../services/api';
import Layout from '../components/Layout';
import { useAuth } from '../contexts/AuthContext';

const EnrollmentTermsPage = () => {
  const { user } = useAuth();
  const [terms, setTerms] = useState([]);
  const [currentTerm, setCurrentTerm] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingTerm, setEditingTerm] = useState(null);
  const [sortField, setSortField] = useState('start_at');
  const [sortDirection, setSortDirection] = useState('desc');
  const [formData, setFormData] = useState({
    name: '',
    sis_term_id: '',
    start_at: '',
    end_at: '',
    grading_period_group_id: '',
  });

  const accountId = 1;

  const fetchTerms = async () => {
    try {
      const response = await fetch(`/api/v1/accounts/${accountId}/terms?per_page=100`, {
        credentials: 'include',
      });
      if (!response.ok) throw new Error('Failed to fetch terms');
      const data = await response.json();
      setTerms(data.enrollment_terms || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const fetchCurrentTerm = async () => {
    try {
      const response = await fetch(`/api/v1/accounts/${accountId}/terms/current`, {
        credentials: 'include',
      });
      if (response.ok) {
        const data = await response.json();
        setCurrentTerm(data);
      }
    } catch {
      // No current term is fine
    }
  };

  useEffect(() => {
    fetchTerms();
    fetchCurrentTerm();
  }, []);

  const resetForm = () => {
    setFormData({ name: '', sis_term_id: '', start_at: '', end_at: '', grading_period_group_id: '' });
    setEditingTerm(null);
    setShowForm(false);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError(null);

    const payload = {
      enrollment_term: {
        name: formData.name,
        sis_term_id: formData.sis_term_id,
        ...(formData.start_at ? { start_at: new Date(formData.start_at).toISOString() } : {}),
        ...(formData.end_at ? { end_at: new Date(formData.end_at).toISOString() } : {}),
        ...(formData.grading_period_group_id ? { grading_period_group_id: parseInt(formData.grading_period_group_id, 10) } : {}),
      },
    };

    try {
      const url = editingTerm
        ? `/api/v1/accounts/${accountId}/terms/${editingTerm.id}`
        : `/api/v1/accounts/${accountId}/terms`;
      const method = editingTerm ? 'PUT' : 'POST';

      const response = await fetch(url, {
        method,
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.errors?.[0]?.message || 'Failed to save term');
      }

      resetForm();
      fetchTerms();
      fetchCurrentTerm();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleEdit = (term) => {
    setEditingTerm(term);
    setFormData({
      name: term.name || '',
      sis_term_id: term.sis_term_id || '',
      start_at: term.start_at ? term.start_at.slice(0, 16) : '',
      end_at: term.end_at ? term.end_at.slice(0, 16) : '',
      grading_period_group_id: term.grading_period_group_id ? String(term.grading_period_group_id) : '',
    });
    setShowForm(true);
  };

  const handleDelete = async (termId) => {
    if (!window.confirm('Are you sure you want to delete this enrollment term?')) return;
    try {
      const response = await fetch(`/api/v1/accounts/${accountId}/terms/${termId}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (!response.ok) throw new Error('Failed to delete term');
      fetchTerms();
      fetchCurrentTerm();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleSort = (field) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('asc');
    }
  };

  const sortedTerms = [...terms].sort((a, b) => {
    let aVal = a[sortField];
    let bVal = b[sortField];
    if (aVal == null) aVal = '';
    if (bVal == null) bVal = '';
    if (typeof aVal === 'string') {
      const cmp = aVal.localeCompare(bVal);
      return sortDirection === 'asc' ? cmp : -cmp;
    }
    const cmp = aVal < bVal ? -1 : aVal > bVal ? 1 : 0;
    return sortDirection === 'asc' ? cmp : -cmp;
  });

  const formatDate = (dateStr) => {
    if (!dateStr) return '--';
    return new Date(dateStr).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const getTermStatus = (term) => {
    const now = new Date();
    if (term.workflow_state === 'deleted') return 'deleted';
    if (term.start_at && term.end_at) {
      const start = new Date(term.start_at);
      const end = new Date(term.end_at);
      if (now < start) return 'upcoming';
      if (now > end) return 'completed';
      return 'active';
    }
    return 'active';
  };

  const statusBadge = (status) => {
    const styles = {
      active: 'bg-accent-success/20 text-accent-success',
      upcoming: 'bg-brand-100 text-brand-800',
      completed: 'bg-surface-2 text-text-secondary',
      deleted: 'bg-accent-danger/20 text-accent-danger',
    };
    return (
      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${styles[status] || styles.active}`}>
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </span>
    );
  };

  const SortHeader = ({ field, label }) => (
    <th
      className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider cursor-pointer hover:text-text-secondary select-none"
      onClick={() => handleSort(field)}
      role="columnheader"
      aria-sort={sortField === field ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleSort(field); } }}
    >
      {label}
      {sortField === field && (
        <span className="ml-1">{sortDirection === 'asc' ? '\u2191' : '\u2193'}</span>
      )}
    </th>
  );

  if (loading) {
    return (
      <Layout>
        <div className="text-center py-12 text-text-tertiary" role="status" aria-live="polite">
          Loading enrollment terms...
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="mb-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <Calendar className="w-6 h-6 text-indigo-600" aria-hidden="true" />
            <h1 className="text-2xl font-bold text-text-primary">Enrollment Terms</h1>
          </div>
          <button
            onClick={() => { resetForm(); setShowForm(!showForm); }}
            className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
            aria-expanded={showForm}
          >
            <Plus className="w-4 h-4" aria-hidden="true" />
            <span>New Term</span>
          </button>
        </div>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger p-3 rounded-md mb-4" role="alert">
          {error}
          <button onClick={() => setError(null)} className="ml-2 text-accent-danger hover:text-accent-danger text-sm" aria-label="Dismiss error">
            Dismiss
          </button>
        </div>
      )}

      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6" role="region" aria-label={editingTerm ? 'Edit enrollment term' : 'Create enrollment term'}>
          <h2 className="text-lg font-semibold mb-4">{editingTerm ? 'Edit Term' : 'Create New Term'}</h2>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label htmlFor="term-name" className="block text-sm font-medium text-text-secondary mb-1">
                  Name <span className="text-accent-danger">*</span>
                </label>
                <input
                  id="term-name"
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="e.g. Fall 2026"
                  required
                  aria-required="true"
                />
              </div>
              <div>
                <label htmlFor="term-sis-id" className="block text-sm font-medium text-text-secondary mb-1">
                  SIS Term ID
                </label>
                <input
                  id="term-sis-id"
                  type="text"
                  value={formData.sis_term_id}
                  onChange={(e) => setFormData({ ...formData, sis_term_id: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="e.g. FALL2026"
                />
              </div>
              <div>
                <label htmlFor="term-start" className="block text-sm font-medium text-text-secondary mb-1">
                  Start Date
                </label>
                <input
                  id="term-start"
                  type="datetime-local"
                  value={formData.start_at}
                  onChange={(e) => setFormData({ ...formData, start_at: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                />
              </div>
              <div>
                <label htmlFor="term-end" className="block text-sm font-medium text-text-secondary mb-1">
                  End Date
                </label>
                <input
                  id="term-end"
                  type="datetime-local"
                  value={formData.end_at}
                  onChange={(e) => setFormData({ ...formData, end_at: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                />
              </div>
              <div>
                <label htmlFor="term-grading-group" className="block text-sm font-medium text-text-secondary mb-1">
                  Grading Period Group ID
                </label>
                <input
                  id="term-grading-group"
                  type="number"
                  value={formData.grading_period_group_id}
                  onChange={(e) => setFormData({ ...formData, grading_period_group_id: e.target.value })}
                  className="w-full border border-border-strong rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="Optional"
                  min="1"
                />
              </div>
            </div>
            <div className="flex items-center space-x-3">
              <button
                type="submit"
                className="bg-brand-600 text-white px-4 py-2 rounded-md text-sm hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2"
              >
                {editingTerm ? 'Update Term' : 'Create Term'}
              </button>
              <button
                type="button"
                onClick={resetForm}
                className="text-text-secondary hover:text-text-primary text-sm"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {terms.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <Calendar className="w-12 h-12 text-text-disabled mx-auto mb-4" aria-hidden="true" />
          <h3 className="text-lg font-medium text-text-primary mb-1">No enrollment terms</h3>
          <p className="text-text-tertiary text-sm">Create your first enrollment term to organize courses by academic period.</p>
        </div>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-border-default" role="table" aria-label="Enrollment terms">
            <thead className="bg-surface-1">
              <tr>
                <SortHeader field="name" label="Name" />
                <SortHeader field="start_at" label="Start Date" />
                <SortHeader field="end_at" label="End Date" />
                <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">Status</th>
                <th className="px-4 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">SIS ID</th>
                <th className="px-4 py-3 text-right text-xs font-medium text-text-tertiary uppercase tracking-wider">Actions</th>
              </tr>
            </thead>
            <tbody className="bg-surface-0 divide-y divide-border-default">
              {sortedTerms.map((term) => {
                const status = getTermStatus(term);
                const isCurrent = currentTerm && currentTerm.id === term.id;
                return (
                  <tr
                    key={term.id}
                    className={`hover:bg-surface-1 ${isCurrent ? 'bg-indigo-50' : ''}`}
                  >
                    <td className="px-4 py-3 whitespace-nowrap">
                      <div className="flex items-center space-x-2">
                        <span className="text-sm font-medium text-text-primary">{term.name}</span>
                        {isCurrent && (
                          <span className="inline-flex items-center space-x-1 px-2 py-0.5 rounded-full text-xs font-medium bg-indigo-100 text-indigo-800">
                            <CheckCircle className="w-3 h-3" aria-hidden="true" />
                            <span>Current</span>
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary">
                      <div className="flex items-center space-x-1">
                        <Clock className="w-3.5 h-3.5 text-text-disabled" aria-hidden="true" />
                        <span>{formatDate(term.start_at)}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary">
                      {formatDate(term.end_at)}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap">
                      {statusBadge(status)}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-sm text-text-tertiary font-mono">
                      {term.sis_term_id || '--'}
                    </td>
                    <td className="px-4 py-3 whitespace-nowrap text-right">
                      <div className="flex items-center justify-end space-x-2">
                        <button
                          onClick={() => handleEdit(term)}
                          className="text-text-disabled hover:text-brand-600 p-1 rounded focus:outline-none focus:ring-2 focus:ring-brand-500"
                          aria-label={`Edit ${term.name}`}
                          title="Edit term"
                        >
                          <Edit2 className="w-4 h-4" aria-hidden="true" />
                        </button>
                        <button
                          onClick={() => handleDelete(term.id)}
                          className="text-text-disabled hover:text-accent-danger p-1 rounded focus:outline-none focus:ring-2 focus:ring-accent-danger"
                          aria-label={`Delete ${term.name}`}
                          title="Delete term"
                        >
                          <Trash2 className="w-4 h-4" aria-hidden="true" />
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </Layout>
  );
};

export default EnrollmentTermsPage;
