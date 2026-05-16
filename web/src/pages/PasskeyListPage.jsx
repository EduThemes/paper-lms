import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { KeyRound, Trash2, Plus, Cloud, Pencil, Check, X } from 'lucide-react';
import Layout from '../components/Layout';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

// PasskeyListPage shows the user's registered passkeys with rename
// + revoke controls. Sits at /users/self/passkeys.
export default function PasskeyListPage() {
  const navigate = useNavigate();
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [editingId, setEditingId] = useState(null);
  const [draftName, setDraftName] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetch(`${API_URL}/users/self/passkeys`, { credentials: 'include' });
      if (!res.ok) throw new Error(`list failed (${res.status})`);
      const data = await res.json();
      setRows(data.passkeys || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const revoke = async (id) => {
    if (!confirm('Revoke this passkey? You will no longer be able to sign in with this device.')) return;
    const res = await fetch(`${API_URL}/users/self/passkeys/${id}`, {
      method: 'DELETE',
      credentials: 'include',
    });
    if (res.ok || res.status === 204) {
      setRows((r) => r.filter((p) => p.id !== id));
    } else {
      setError(`revoke failed (${res.status})`);
    }
  };

  const saveName = async (id) => {
    const res = await fetch(`${API_URL}/users/self/passkeys/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({ nickname: draftName }),
    });
    if (res.ok) {
      setRows((rs) => rs.map((r) => (r.id === id ? { ...r, nickname: draftName } : r)));
      setEditingId(null);
    } else {
      setError(`rename failed (${res.status})`);
    }
  };

  return (
    <Layout>
      <div className="max-w-3xl mx-auto p-6 space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold flex items-center gap-2">
            <KeyRound className="text-blue-600" />
            Your passkeys
          </h1>
          <button
            onClick={() => navigate('/users/self/passkeys/enroll')}
            className="px-3 py-2 rounded bg-blue-600 text-white hover:bg-blue-700 flex items-center gap-1"
          >
            <Plus size={16} />
            Add passkey
          </button>
        </div>

        {error && (
          <div className="rounded border border-red-300 bg-red-50 p-3 text-sm text-red-700">
            {error}
          </div>
        )}

        {loading ? (
          <p className="text-gray-500">Loading…</p>
        ) : rows.length === 0 ? (
          <div className="rounded border border-dashed border-gray-300 p-8 text-center">
            <p className="text-gray-600">
              You have no passkeys yet. Add one to sign in without a password.
            </p>
          </div>
        ) : (
          <ul className="divide-y divide-gray-200 rounded border border-gray-200">
            {rows.map((p) => (
              <li key={p.id} className="p-4 flex items-center justify-between gap-4">
                <div className="flex-1">
                  {editingId === p.id ? (
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        value={draftName}
                        onChange={(e) => setDraftName(e.target.value)}
                        className="px-2 py-1 border border-gray-300 rounded flex-1"
                        maxLength={80}
                      />
                      <button
                        onClick={() => saveName(p.id)}
                        className="p-1 text-green-600 hover:bg-green-50 rounded"
                        aria-label="Save"
                      >
                        <Check size={18} />
                      </button>
                      <button
                        onClick={() => setEditingId(null)}
                        className="p-1 text-gray-500 hover:bg-gray-50 rounded"
                        aria-label="Cancel"
                      >
                        <X size={18} />
                      </button>
                    </div>
                  ) : (
                    <div className="flex items-center gap-2">
                      <div className="font-medium">
                        {p.nickname || <span className="text-gray-400">Unnamed passkey</span>}
                      </div>
                      {p.backup_state && (
                        <span className="inline-flex items-center gap-1 text-xs text-blue-700 bg-blue-50 px-2 py-0.5 rounded">
                          <Cloud size={12} /> Synced
                        </span>
                      )}
                      <button
                        onClick={() => {
                          setEditingId(p.id);
                          setDraftName(p.nickname || '');
                        }}
                        className="p-1 text-gray-500 hover:bg-gray-100 rounded"
                        aria-label="Rename"
                      >
                        <Pencil size={14} />
                      </button>
                    </div>
                  )}
                  <div className="text-xs text-gray-500 mt-1">
                    Added {new Date(p.created_at).toLocaleDateString()}
                    {p.last_used_at ? ` · Last used ${new Date(p.last_used_at).toLocaleDateString()}` : ' · Never used'}
                  </div>
                </div>
                <button
                  onClick={() => revoke(p.id)}
                  className="p-2 text-red-600 hover:bg-red-50 rounded"
                  aria-label="Revoke"
                  title="Revoke this passkey"
                >
                  <Trash2 size={18} />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Layout>
  );
}
