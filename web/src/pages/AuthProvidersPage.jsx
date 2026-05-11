import React, { useState, useEffect, useCallback } from 'react';
import { Shield, Plus, Trash2, Edit3, AlertTriangle, X, TestTube2, Check } from 'lucide-react';
import Layout from '../components/Layout';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';
const getHeaders = () => ({
  'Content-Type': 'application/json',
});

const ACCOUNT_ID = 1;

const AUTH_TYPES = [
  { value: 'saml', label: 'SAML' },
  { value: 'ldap', label: 'LDAP' },
  { value: 'cas', label: 'CAS' },
];

const EMPTY_FORM = {
  auth_type: 'saml',
  position: 1,
  // SAML
  idp_entity_id: '',
  log_in_url: '',
  log_out_url: '',
  certificate_fingerprint: '',
  // LDAP
  ldap_host: '',
  ldap_port: '',
  ldap_base: '',
  ldap_filter: '',
  ldap_bind_dn: '',
  ldap_bind_password: '',
  ldap_use_tls: false,
  ldap_login_attribute: 'uid',
  // CAS
  cas_base_url: '',
  cas_login_url: '',
  cas_validate_url: '',
  cas_logout_url: '',
  // General
  jit_provisioning: false,
};

const AuthProvidersPage = () => {
  const [providers, setProviders] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(null);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState(null);
  const [formData, setFormData] = useState({ ...EMPTY_FORM });
  const [submitting, setSubmitting] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState(null);
  const [deleting, setDeleting] = useState(false);
  const [testingId, setTestingId] = useState(null);

  const fetchProviders = useCallback(async () => {
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/authentication_providers?per_page=100`, {
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) throw new Error('Failed to fetch authentication providers');
      const data = await res.json();
      setProviders(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchProviders();
  }, [fetchProviders]);

  const resetForm = () => {
    setFormData({ ...EMPTY_FORM });
    setEditingId(null);
    setShowForm(false);
  };

  const handleEdit = (provider) => {
    setFormData({
      auth_type: provider.auth_type || 'saml',
      position: provider.position || 1,
      idp_entity_id: provider.idp_entity_id || '',
      log_in_url: provider.log_in_url || '',
      log_out_url: provider.log_out_url || '',
      certificate_fingerprint: provider.certificate_fingerprint || '',
      ldap_host: provider.ldap_host || '',
      ldap_port: provider.ldap_port || '',
      ldap_base: provider.ldap_base || '',
      ldap_filter: provider.ldap_filter || '',
      ldap_bind_dn: provider.ldap_bind_dn || '',
      ldap_bind_password: '',
      ldap_use_tls: provider.ldap_use_tls || false,
      ldap_login_attribute: provider.ldap_login_attribute || 'uid',
      cas_base_url: provider.cas_base_url || '',
      cas_login_url: provider.cas_login_url || '',
      cas_validate_url: provider.cas_validate_url || '',
      cas_logout_url: provider.cas_logout_url || '',
      jit_provisioning: provider.jit_provisioning || false,
    });
    setEditingId(provider.id);
    setShowForm(true);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    setSuccess(null);

    const body = { ...formData };
    if (body.ldap_port) {
      body.ldap_port = parseInt(body.ldap_port, 10) || 0;
    }
    body.position = parseInt(body.position, 10) || 1;

    try {
      let res;
      if (editingId) {
        res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/authentication_providers/${editingId}`, {
          method: 'PUT',
          credentials: 'include', headers: getHeaders(),
          body: JSON.stringify(body),
        });
      } else {
        res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/authentication_providers`, {
          method: 'POST',
          credentials: 'include', headers: getHeaders(),
          body: JSON.stringify(body),
        });
      }

      if (!res.ok) {
        const errData = await res.json();
        throw new Error(errData.errors?.[0]?.message || 'Failed to save provider');
      }

      setSuccess(editingId ? 'Provider updated successfully.' : 'Provider created successfully.');
      resetForm();
      fetchProviders();
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id) => {
    setDeleting(true);
    setError(null);
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/authentication_providers/${id}`, {
        method: 'DELETE',
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) throw new Error('Failed to delete provider');
      setDeleteConfirm(null);
      setSuccess('Provider deleted successfully.');
      fetchProviders();
    } catch (err) {
      setError(err.message);
    } finally {
      setDeleting(false);
    }
  };

  const handleTestConnection = async (id) => {
    setTestingId(id);
    setError(null);
    setSuccess(null);
    try {
      const res = await fetch(`${API_URL}/accounts/${ACCOUNT_ID}/authentication_providers/${id}/test`, {
        method: 'POST',
        credentials: 'include', headers: getHeaders(),
      });
      if (!res.ok) throw new Error('Failed to test connection');
      const data = await res.json();
      if (data.success) {
        setSuccess(data.message);
      } else {
        setError(data.message);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setTestingId(null);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '-';
    return new Date(dateStr).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  const authTypeLabel = (type) => {
    switch (type) {
      case 'saml': return 'SAML';
      case 'ldap': return 'LDAP';
      case 'cas': return 'CAS';
      default: return type;
    }
  };

  const authTypeBadgeClass = (type) => {
    switch (type) {
      case 'saml': return 'bg-brand-100 text-brand-800';
      case 'ldap': return 'bg-purple-100 text-purple-800';
      case 'cas': return 'bg-accent-warning/20 text-accent-warning';
      default: return 'bg-surface-2 text-text-primary';
    }
  };

  const renderTypeFields = () => {
    switch (formData.auth_type) {
      case 'saml':
        return (
          <>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">IdP Entity ID</label>
              <input
                type="text"
                value={formData.idp_entity_id}
                onChange={(e) => setFormData({ ...formData, idp_entity_id: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://idp.example.com/entity"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Log In URL</label>
              <input
                type="text"
                value={formData.log_in_url}
                onChange={(e) => setFormData({ ...formData, log_in_url: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://idp.example.com/sso/saml"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Log Out URL</label>
              <input
                type="text"
                value={formData.log_out_url}
                onChange={(e) => setFormData({ ...formData, log_out_url: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://idp.example.com/sso/saml/logout"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Certificate Fingerprint</label>
              <input
                type="text"
                value={formData.certificate_fingerprint}
                onChange={(e) => setFormData({ ...formData, certificate_fingerprint: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="AB:CD:EF:12:34:56:78:90..."
              />
            </div>
          </>
        );
      case 'ldap':
        return (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  LDAP Host <span className="text-accent-danger">*</span>
                </label>
                <input
                  type="text"
                  value={formData.ldap_host}
                  onChange={(e) => setFormData({ ...formData, ldap_host: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="ldap.example.com"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  LDAP Port <span className="text-accent-danger">*</span>
                </label>
                <input
                  type="number"
                  value={formData.ldap_port}
                  onChange={(e) => setFormData({ ...formData, ldap_port: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="389"
                  required
                />
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">
                LDAP Base DN <span className="text-accent-danger">*</span>
              </label>
              <input
                type="text"
                value={formData.ldap_base}
                onChange={(e) => setFormData({ ...formData, ldap_base: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="dc=example,dc=com"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">LDAP Filter</label>
              <input
                type="text"
                value={formData.ldap_filter}
                onChange={(e) => setFormData({ ...formData, ldap_filter: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="(objectClass=person)"
              />
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Bind DN</label>
                <input
                  type="text"
                  value={formData.ldap_bind_dn}
                  onChange={(e) => setFormData({ ...formData, ldap_bind_dn: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder="cn=admin,dc=example,dc=com"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Bind Password</label>
                <input
                  type="password"
                  value={formData.ldap_bind_password}
                  onChange={(e) => setFormData({ ...formData, ldap_bind_password: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  placeholder={editingId ? '(leave blank to keep current)' : 'Enter password'}
                />
              </div>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">Login Attribute</label>
              <input
                type="text"
                value={formData.ldap_login_attribute}
                onChange={(e) => setFormData({ ...formData, ldap_login_attribute: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="uid"
              />
              <p className="text-xs text-text-tertiary mt-1">The LDAP attribute to use for login (e.g., uid, sAMAccountName, mail).</p>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="ldap_use_tls"
                checked={formData.ldap_use_tls}
                onChange={(e) => setFormData({ ...formData, ldap_use_tls: e.target.checked })}
                className="rounded border-border-strong text-brand-600 focus:ring-brand-500"
              />
              <label htmlFor="ldap_use_tls" className="text-sm font-medium text-text-secondary">
                Use TLS (LDAPS)
              </label>
            </div>
          </>
        );
      case 'cas':
        return (
          <>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">CAS Base URL</label>
              <input
                type="text"
                value={formData.cas_base_url}
                onChange={(e) => setFormData({ ...formData, cas_base_url: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://cas.example.com/cas"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">CAS Login URL</label>
              <input
                type="text"
                value={formData.cas_login_url}
                onChange={(e) => setFormData({ ...formData, cas_login_url: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://cas.example.com/cas/login"
              />
              <p className="text-xs text-text-tertiary mt-1">Leave blank to auto-derive from base URL.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">CAS Validate URL</label>
              <input
                type="text"
                value={formData.cas_validate_url}
                onChange={(e) => setFormData({ ...formData, cas_validate_url: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://cas.example.com/cas/serviceValidate"
              />
              <p className="text-xs text-text-tertiary mt-1">Leave blank to auto-derive from base URL.</p>
            </div>
            <div>
              <label className="block text-sm font-medium text-text-secondary mb-1">CAS Logout URL</label>
              <input
                type="text"
                value={formData.cas_logout_url}
                onChange={(e) => setFormData({ ...formData, cas_logout_url: e.target.value })}
                className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                placeholder="https://cas.example.com/cas/logout"
              />
            </div>
          </>
        );
      default:
        return null;
    }
  };

  return (
    <Layout>
      <div className="flex justify-between items-center mb-6">
        <div>
          <h2 className="text-2xl font-bold text-text-primary">Authentication Providers</h2>
          <p className="text-text-secondary mt-1">
            Configure SAML, LDAP, and CAS authentication for your institution.
          </p>
        </div>
        <button
          onClick={() => {
            if (showForm && !editingId) {
              resetForm();
            } else {
              setFormData({ ...EMPTY_FORM });
              setEditingId(null);
              setShowForm(true);
            }
          }}
          className="flex items-center gap-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
        >
          <Plus className="w-4 h-4" />
          Add Provider
        </button>
      </div>

      {error && (
        <div className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {success && (
        <div className="bg-accent-success/10 border border-accent-success/30 text-accent-success px-4 py-3 rounded-md mb-6 flex items-center gap-2">
          <Check className="w-4 h-4 flex-shrink-0" />
          {success}
          <button onClick={() => setSuccess(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Create / Edit Form */}
      {showForm && (
        <div className="bg-surface-0 rounded-lg shadow p-6 mb-6">
          <h3 className="font-semibold mb-4">
            {editingId ? 'Edit Authentication Provider' : 'Add Authentication Provider'}
          </h3>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">
                  Authentication Type <span className="text-accent-danger">*</span>
                </label>
                <div className="flex gap-1 bg-surface-2 p-1 rounded-md">
                  {AUTH_TYPES.map((type) => (
                    <button
                      key={type.value}
                      type="button"
                      onClick={() => setFormData({ ...formData, auth_type: type.value })}
                      className={`flex-1 py-2 px-3 rounded text-sm font-medium transition-colors ${
                        formData.auth_type === type.value
                          ? 'bg-surface-0 shadow text-brand-700'
                          : 'text-text-secondary hover:text-text-primary'
                      }`}
                    >
                      {type.label}
                    </button>
                  ))}
                </div>
              </div>
              <div>
                <label className="block text-sm font-medium text-text-secondary mb-1">Position</label>
                <input
                  type="number"
                  value={formData.position}
                  onChange={(e) => setFormData({ ...formData, position: e.target.value })}
                  className="w-full rounded-md border border-border-strong px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500"
                  min="1"
                />
                <p className="text-xs text-text-tertiary mt-1">Order in which providers are tried during authentication.</p>
              </div>
            </div>

            <hr className="my-4" />

            <h4 className="text-sm font-semibold text-text-primary mb-2">
              {formData.auth_type === 'saml' && 'SAML Configuration'}
              {formData.auth_type === 'ldap' && 'LDAP Configuration'}
              {formData.auth_type === 'cas' && 'CAS Configuration'}
            </h4>

            {renderTypeFields()}

            <hr className="my-4" />

            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="jit_provisioning"
                checked={formData.jit_provisioning}
                onChange={(e) => setFormData({ ...formData, jit_provisioning: e.target.checked })}
                className="rounded border-border-strong text-brand-600 focus:ring-brand-500"
              />
              <label htmlFor="jit_provisioning" className="text-sm font-medium text-text-secondary">
                Enable Just-In-Time (JIT) Provisioning
              </label>
            </div>
            <p className="text-xs text-text-tertiary -mt-2 ml-6">
              Automatically create user accounts on first login.
            </p>

            <div className="flex gap-3 pt-2">
              <button
                type="submit"
                disabled={submitting}
                className="bg-accent-success text-white px-4 py-2 rounded-md hover:bg-accent-success/90 text-sm disabled:opacity-50"
              >
                {submitting ? 'Saving...' : editingId ? 'Update Provider' : 'Create Provider'}
              </button>
              <button
                type="button"
                onClick={resetForm}
                className="text-text-tertiary hover:text-text-secondary px-4 py-2 text-sm"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Delete Confirmation */}
      {deleteConfirm !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-surface-0 rounded-lg shadow-xl p-6 max-w-sm mx-4">
            <h3 className="font-semibold text-text-primary mb-2">Delete Authentication Provider</h3>
            <p className="text-sm text-text-secondary mb-4">
              Are you sure you want to delete this authentication provider? Users who rely on this provider will no longer be able to log in through it.
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setDeleteConfirm(null)}
                className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary"
                disabled={deleting}
              >
                Cancel
              </button>
              <button
                onClick={() => handleDelete(deleteConfirm)}
                disabled={deleting}
                className="bg-accent-danger text-white px-4 py-2 rounded-md hover:bg-accent-danger/90 text-sm disabled:opacity-50"
              >
                {deleting ? 'Deleting...' : 'Delete Provider'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Providers List */}
      {loading ? (
        <div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  Loading authentication providers...
</div>
      ) : providers.length === 0 ? (
        <div className="bg-surface-0 rounded-lg shadow p-12 text-center">
          <Shield className="w-12 h-12 text-text-disabled mx-auto mb-4" />
          <h3 className="text-lg font-medium text-text-primary mb-1">No Authentication Providers</h3>
          <p className="text-text-tertiary text-sm">
            No authentication providers have been configured. Add a SAML, LDAP, or CAS provider to enable single sign-on.
          </p>
        </div>
      ) : (
        <div className="bg-surface-0 rounded-lg shadow overflow-hidden">
          <table className="min-w-full divide-y divide-border-default">
            <thead className="bg-surface-1">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Position
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  Details
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  JIT
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-text-tertiary uppercase tracking-wider">
                  State
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
              {providers.map((provider) => (
                <tr key={provider.id} className="hover:bg-surface-1">
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-text-secondary">
                    {provider.position}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span className={`text-xs px-2 py-1 rounded-full font-medium ${authTypeBadgeClass(provider.auth_type)}`}>
                      {authTypeLabel(provider.auth_type)}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    <div className="text-sm text-text-primary">
                      {provider.auth_type === 'saml' && (
                        <span title={provider.idp_entity_id}>
                          {provider.idp_entity_id ? `IdP: ${provider.idp_entity_id}` : 'SAML (not configured)'}
                        </span>
                      )}
                      {provider.auth_type === 'ldap' && (
                        <span>
                          {provider.ldap_host ? `${provider.ldap_host}:${provider.ldap_port}` : 'LDAP (not configured)'}
                        </span>
                      )}
                      {provider.auth_type === 'cas' && (
                        <span title={provider.cas_base_url}>
                          {provider.cas_base_url || 'CAS (not configured)'}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    {provider.jit_provisioning ? (
                      <span className="text-accent-success">Enabled</span>
                    ) : (
                      <span className="text-text-disabled">Disabled</span>
                    )}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap">
                    <span
                      className={`text-xs px-2 py-1 rounded-full ${
                        provider.workflow_state === 'active'
                          ? 'bg-accent-success/20 text-accent-success'
                          : 'bg-surface-2 text-text-secondary'
                      }`}
                    >
                      {provider.workflow_state || 'active'}
                    </span>
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-text-tertiary">
                    {formatDate(provider.created_at)}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-right">
                    <div className="flex items-center justify-end gap-2">
                      {provider.auth_type === 'ldap' && (
                        <button
                          onClick={() => handleTestConnection(provider.id)}
                          disabled={testingId === provider.id}
                          className="text-brand-600 hover:text-brand-800 p-1 disabled:opacity-50"
                          title="Test LDAP Connection"
                        >
                          <TestTube2 className="w-4 h-4" />
                        </button>
                      )}
                      <button
                        onClick={() => handleEdit(provider)}
                        className="text-text-tertiary hover:text-text-secondary p-1"
                        title="Edit provider"
                      >
                        <Edit3 className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(provider.id)}
                        className="text-accent-danger hover:text-accent-danger p-1"
                        title="Delete provider"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Layout>
  );
};

export default AuthProvidersPage;
