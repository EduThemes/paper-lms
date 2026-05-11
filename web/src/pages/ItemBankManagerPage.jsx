import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Navigate } from 'react-router-dom';
import { Plus, Layers, Pencil, Trash2, Check, X, FileQuestion, Send, Shuffle } from 'lucide-react';
import { api } from '../services/api';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import QuizzesSubNav from '../components/quiz/QuizzesSubNav';
import { TYPE_LABELS } from '../components/quiz/itemTypes/types';

/**
 * Item Bank Manager — list, create, rename, delete banks; view their items;
 * push items into an existing quiz (single or random-draw).
 */
const ItemBankManagerPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const [banks, setBanks] = useState([]);
  const [selectedBank, setSelectedBank] = useState(null);
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [message, setMessage] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [newBankTitle, setNewBankTitle] = useState('');
  const [editingBankId, setEditingBankId] = useState(null);
  const [editBankTitle, setEditBankTitle] = useState('');

  // Add-to-quiz dialog state
  const [pushDialog, setPushDialog] = useState(null); // { mode: 'select'|'random', bank }
  const [quizzes, setQuizzes] = useState([]);
  const [targetQuizId, setTargetQuizId] = useState('');
  const [randomCount, setRandomCount] = useState(5);
  const [selectedItemIds, setSelectedItemIds] = useState([]);
  const [pushing, setPushing] = useState(false);

  const fetchBanks = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const data = await api.listQuestionBanks(courseId);
      setBanks(data || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => { fetchBanks(); }, [fetchBanks]);

  useEffect(() => {
    if (!selectedBank) { setItems([]); return; }
    let cancelled = false;
    api.listBankQuestions(courseId, selectedBank.id)
      .then(data => { if (!cancelled) setItems(data || []); })
      .catch(err => { if (!cancelled) setError(err.message); });
    return () => { cancelled = true; };
  }, [selectedBank, courseId]);

  // Load course quizzes lazily when a push dialog opens.
  useEffect(() => {
    if (!pushDialog) return;
    api.getQuizzes(courseId, 1, 200)
      .then(result => setQuizzes(result.data || []))
      .catch(() => setQuizzes([]));
  }, [pushDialog, courseId]);

  const handleCreate = async () => {
    if (!newBankTitle.trim()) return;
    try {
      const created = await api.createQuestionBank(courseId, newBankTitle.trim());
      setBanks(prev => [created, ...prev]);
      setNewBankTitle('');
      setShowCreate(false);
      setMessage('Bank created.');
    } catch (err) {
      setMessage('Error: ' + err.message);
    }
  };

  const handleRename = async (bank) => {
    if (!editBankTitle.trim()) return;
    try {
      const updated = await api.updateQuestionBank(courseId, bank.id, editBankTitle.trim());
      setBanks(prev => prev.map(b => b.id === bank.id ? (updated || { ...b, title: editBankTitle.trim() }) : b));
      setEditingBankId(null);
      setEditBankTitle('');
      setMessage('Bank renamed.');
    } catch (err) {
      setMessage('Error: ' + err.message);
    }
  };

  const handleDelete = async (bank) => {
    if (!window.confirm(`Delete bank "${bank.title}"? This cannot be undone.`)) return;
    try {
      await api.deleteQuestionBank(courseId, bank.id);
      setBanks(prev => prev.filter(b => b.id !== bank.id));
      if (selectedBank?.id === bank.id) setSelectedBank(null);
      setMessage('Bank deleted.');
    } catch (err) {
      setMessage('Error: ' + err.message);
    }
  };

  const openPush = (mode, bank) => {
    setPushDialog({ mode, bank });
    setTargetQuizId('');
    setSelectedItemIds([]);
    setRandomCount(5);
  };

  const handlePushSelected = async () => {
    if (!targetQuizId || selectedItemIds.length === 0) return;
    setPushing(true);
    try {
      // Prefer the Wave A from-bank-item endpoint; fall back to the legacy
      // pull_to_quiz endpoint when the new one isn't deployed yet.
      for (const itemId of selectedItemIds) {
        try {
          await api.addBankItemToQuiz(courseId, targetQuizId, itemId);
        } catch {
          await api.pullBankQuestionsToQuiz(courseId, pushDialog.bank.id, targetQuizId, [itemId]);
        }
      }
      setMessage(`Added ${selectedItemIds.length} item(s) to quiz.`);
      setPushDialog(null);
    } catch (err) {
      setMessage('Error: ' + err.message);
    } finally {
      setPushing(false);
    }
  };

  const handlePushRandom = async () => {
    if (!targetQuizId || !randomCount) return;
    setPushing(true);
    try {
      try {
        await api.randomDrawFromBank(courseId, targetQuizId, pushDialog.bank.id, Number(randomCount));
      } catch {
        // Fallback: pick N random item ids client-side and push them.
        const pool = [...items];
        const picks = [];
        for (let i = 0; i < Math.min(Number(randomCount), pool.length); i++) {
          const idx = Math.floor(Math.random() * pool.length);
          picks.push(pool.splice(idx, 1)[0]);
        }
        await api.pullBankQuestionsToQuiz(courseId, pushDialog.bank.id, targetQuizId, picks.map(p => p.id));
      }
      setMessage(`Random draw of ${randomCount} added.`);
      setPushDialog(null);
    } catch (err) {
      setMessage('Error: ' + err.message);
    } finally {
      setPushing(false);
    }
  };

  if (isTeacher === false) return <Navigate to={`/courses/${courseId}/quizzes`} replace />;

  return (
    <Layout>
      <CourseNav />
      <QuizzesSubNav />

      <header className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-text-primary flex items-center gap-2">
            <Layers className="w-6 h-6 text-brand-600" />
            Item Banks
          </h1>
          <p className="text-sm text-text-tertiary mt-1">
            Reusable question pools. Add items to any quiz, or use random draw for variety.
          </p>
        </div>
        {!showCreate && (
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1 px-3 py-1.5 bg-brand-600 text-white rounded hover:bg-brand-700 text-sm font-medium"
          >
            <Plus className="w-4 h-4" /> New Bank
          </button>
        )}
      </header>

      {message && (
        <div className={`mb-4 px-4 py-2 rounded text-sm ${message.startsWith('Error') ? 'bg-accent-danger/10 text-accent-danger' : 'bg-accent-success/10 text-accent-success'}`}>
          {message}
        </div>
      )}
      {error && (
        <div className="mb-4 px-4 py-2 rounded text-sm bg-accent-danger/10 text-accent-danger">{error}</div>
      )}

      {showCreate && (
        <div className="mb-4 bg-surface-0 border border-border-default rounded p-4 flex items-center gap-2">
          <input
            type="text"
            value={newBankTitle}
            onChange={(e) => setNewBankTitle(e.target.value)}
            placeholder="Bank title..."
            className="flex-1 border border-border-strong rounded px-3 py-1.5 text-sm bg-surface-0 text-text-primary"
            autoFocus
          />
          <button onClick={handleCreate} className="inline-flex items-center gap-1 px-3 py-1.5 bg-accent-success text-white rounded hover:bg-accent-success/90 text-sm font-medium">
            <Check className="w-4 h-4" /> Create
          </button>
          <button onClick={() => { setShowCreate(false); setNewBankTitle(''); }} className="inline-flex items-center gap-1 px-3 py-1.5 bg-border-default text-text-secondary rounded hover:bg-border-strong text-sm font-medium">
            <X className="w-4 h-4" /> Cancel
          </button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Banks list */}
        <section className="bg-surface-0 rounded-lg shadow border border-border-default overflow-hidden">
          <div className="px-4 py-3 border-b border-border-default flex items-center justify-between">
            <h2 className="font-semibold text-sm text-text-primary">All Banks <span className="text-text-tertiary font-normal">({banks.length})</span></h2>
          </div>
          {loading ? (
            <div className="p-6 text-center text-text-tertiary text-sm">Loading…</div>
          ) : banks.length === 0 ? (
            <div className="p-6 text-center text-text-tertiary text-sm">No banks yet. Create your first bank above.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-surface-1 text-text-tertiary text-xs uppercase tracking-wide">
                <tr>
                  <th className="text-left px-4 py-2 font-medium">Title</th>
                  <th className="text-right px-4 py-2 font-medium">Items</th>
                  <th className="text-right px-4 py-2 font-medium">Updated</th>
                  <th className="px-2 py-2 w-32"></th>
                </tr>
              </thead>
              <tbody>
                {banks.map(bank => (
                  <tr key={bank.id}
                      className={`border-t border-border-default cursor-pointer hover:bg-surface-1 ${selectedBank?.id === bank.id ? 'bg-brand-50/40 dark:bg-brand-500/10' : ''}`}
                      onClick={() => setSelectedBank(bank)}>
                    <td className="px-4 py-2">
                      {editingBankId === bank.id ? (
                        <input
                          type="text"
                          value={editBankTitle}
                          onClick={(e) => e.stopPropagation()}
                          onChange={(e) => setEditBankTitle(e.target.value)}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter') handleRename(bank);
                            if (e.key === 'Escape') { setEditingBankId(null); setEditBankTitle(''); }
                          }}
                          className="border border-border-strong rounded px-2 py-1 text-sm bg-surface-0 text-text-primary w-full"
                          autoFocus
                        />
                      ) : (
                        <span className="font-medium text-text-primary">{bank.title}</span>
                      )}
                    </td>
                    <td className="px-4 py-2 text-right text-text-secondary">{bank.question_count ?? bank.item_count ?? '—'}</td>
                    <td className="px-4 py-2 text-right text-text-tertiary text-xs">
                      {bank.updated_at ? new Date(bank.updated_at).toLocaleDateString() : '—'}
                    </td>
                    <td className="px-2 py-2 text-right">
                      <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                        {editingBankId === bank.id ? (
                          <>
                            <button onClick={() => handleRename(bank)} className="p-1 text-accent-success hover:text-accent-success/80" aria-label="Save rename">
                              <Check className="w-3.5 h-3.5" />
                            </button>
                            <button onClick={() => { setEditingBankId(null); setEditBankTitle(''); }} className="p-1 text-text-disabled hover:text-text-secondary" aria-label="Cancel rename">
                              <X className="w-3.5 h-3.5" />
                            </button>
                          </>
                        ) : (
                          <>
                            <button onClick={() => { setEditingBankId(bank.id); setEditBankTitle(bank.title); }}
                                    className="p-1 text-text-disabled hover:text-brand-600" aria-label="Rename bank">
                              <Pencil className="w-3.5 h-3.5" />
                            </button>
                            <button onClick={() => handleDelete(bank)}
                                    className="p-1 text-text-disabled hover:text-accent-danger" aria-label="Delete bank">
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          </>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        {/* Selected bank items */}
        <section className="bg-surface-0 rounded-lg shadow border border-border-default overflow-hidden">
          <div className="px-4 py-3 border-b border-border-default flex items-center justify-between">
            <h2 className="font-semibold text-sm text-text-primary">
              {selectedBank ? selectedBank.title : 'Select a bank to view items'}
              {selectedBank && (
                <span className="text-text-tertiary font-normal ml-1">({items.length} item{items.length !== 1 ? 's' : ''})</span>
              )}
            </h2>
            {selectedBank && (
              <div className="flex items-center gap-2">
                <button
                  onClick={() => openPush('select', selectedBank)}
                  className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded border border-border-strong hover:bg-surface-1 text-text-secondary"
                >
                  <Send className="w-3.5 h-3.5" /> Add to quiz
                </button>
                <button
                  onClick={() => openPush('random', selectedBank)}
                  className="inline-flex items-center gap-1 px-2 py-1 text-xs rounded border border-border-strong hover:bg-surface-1 text-text-secondary"
                >
                  <Shuffle className="w-3.5 h-3.5" /> Random draw
                </button>
              </div>
            )}
          </div>
          {!selectedBank ? (
            <div className="p-6 text-center text-text-tertiary text-sm">
              <FileQuestion className="w-8 h-8 mx-auto mb-2 text-text-disabled" />
              Pick a bank on the left to inspect its items.
            </div>
          ) : items.length === 0 ? (
            <div className="p-6 text-center text-text-tertiary text-sm">No items yet in this bank.</div>
          ) : (
            <ul className="divide-y divide-border-default max-h-[600px] overflow-y-auto">
              {items.map(item => (
                <li key={item.id} className="px-4 py-3 hover:bg-surface-1">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="text-xs text-text-tertiary uppercase tracking-wide">
                        {TYPE_LABELS[item.question_type] || item.question_type}
                      </div>
                      <div className="text-sm text-text-primary mt-1 line-clamp-2"
                           dangerouslySetInnerHTML={{ __html: String(item.question_text || '').slice(0, 240) }} />
                    </div>
                    <span className="text-xs text-text-tertiary whitespace-nowrap">{item.points_possible ?? 1} pts</span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {/* Push to quiz dialog */}
      {pushDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true">
          <div className="bg-surface-0 rounded-lg shadow-xl w-full max-w-lg p-6">
            <h3 className="text-lg font-semibold text-text-primary mb-3">
              {pushDialog.mode === 'random' ? 'Random Draw' : 'Add Items to Quiz'}
            </h3>
            <p className="text-xs text-text-tertiary mb-4">
              From bank: <strong>{pushDialog.bank.title}</strong>
            </p>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">Target quiz</label>
                <select
                  value={targetQuizId}
                  onChange={(e) => setTargetQuizId(e.target.value)}
                  className="w-full border border-border-strong rounded px-3 py-2 text-sm bg-surface-0 text-text-primary"
                >
                  <option value="">— Pick a quiz —</option>
                  {quizzes.map(q => (
                    <option key={q.id} value={q.id}>{q.title}</option>
                  ))}
                </select>
              </div>
              {pushDialog.mode === 'random' ? (
                <div>
                  <label className="block text-xs font-medium text-text-secondary mb-1">How many random items?</label>
                  <input type="number" min="1" max={items.length || 50}
                    value={randomCount}
                    onChange={(e) => setRandomCount(e.target.value)}
                    className="w-32 border border-border-strong rounded px-3 py-2 text-sm bg-surface-0 text-text-primary" />
                </div>
              ) : (
                <div>
                  <label className="block text-xs font-medium text-text-secondary mb-1">
                    Pick items ({selectedItemIds.length} selected)
                  </label>
                  <div className="border border-border-default rounded max-h-[200px] overflow-y-auto divide-y divide-border-default">
                    {items.map(it => (
                      <label key={it.id} className="flex items-center gap-2 px-3 py-1.5 text-xs text-text-secondary cursor-pointer hover:bg-surface-1">
                        <input
                          type="checkbox"
                          checked={selectedItemIds.includes(it.id)}
                          onChange={(e) => {
                            setSelectedItemIds(prev => e.target.checked
                              ? [...prev, it.id]
                              : prev.filter(id => id !== it.id));
                          }}
                        />
                        <span className="truncate flex-1">
                          [{TYPE_LABELS[it.question_type] || it.question_type}]{' '}
                          <span className="text-text-primary"
                                dangerouslySetInnerHTML={{ __html: String(it.question_text || '').slice(0, 120) }} />
                        </span>
                      </label>
                    ))}
                  </div>
                </div>
              )}
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <button onClick={() => setPushDialog(null)} className="px-3 py-1.5 text-sm bg-border-default text-text-secondary rounded hover:bg-border-strong">
                Cancel
              </button>
              {pushDialog.mode === 'random' ? (
                <button onClick={handlePushRandom} disabled={!targetQuizId || pushing}
                  className="px-3 py-1.5 text-sm bg-brand-600 text-white rounded hover:bg-brand-700 disabled:opacity-50">
                  {pushing ? 'Adding…' : `Draw ${randomCount}`}
                </button>
              ) : (
                <button onClick={handlePushSelected} disabled={!targetQuizId || selectedItemIds.length === 0 || pushing}
                  className="px-3 py-1.5 text-sm bg-brand-600 text-white rounded hover:bg-brand-700 disabled:opacity-50">
                  {pushing ? 'Adding…' : `Add ${selectedItemIds.length}`}
                </button>
              )}
            </div>
          </div>
        </div>
      )}
    </Layout>
  );
};

export default ItemBankManagerPage;
