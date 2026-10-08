import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Trash2, Edit2, Calendar, ChevronDown, ChevronRight } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { api } from '../services/api';
import Layout from '../components/Layout';
import useAccountId from '../hooks/useAccountId';
import AccountGradingSchemesPanel from '../components/grading/AccountGradingSchemesPanel';

const GradingPeriodsPage = () => {
  const { t } = useTranslation();
  const [groups, setGroups] = useState([]);
  const [expandedGroups, setExpandedGroups] = useState({});
  const [periods, setPeriods] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreateGroup, setShowCreateGroup] = useState(false);
  const [newGroupTitle, setNewGroupTitle] = useState('');
  const [showCreatePeriod, setShowCreatePeriod] = useState(null);
  const [newPeriod, setNewPeriod] = useState({ title: '', start_date: '', end_date: '', close_date: '', weight: '' });

  const accountId = useAccountId();

  const fetchGroups = async () => {
    try {
      const { data } = await api.getGradingPeriodGroups(accountId, 1, 100);
      setGroups(data?.grading_period_groups || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const fetchPeriods = async (groupId) => {
    try {
      const data = await api.getGradingPeriods(accountId, groupId);
      setPeriods(prev => ({ ...prev, [groupId]: data?.grading_periods || [] }));
    } catch (err) {
      setError(err.message);
    }
  };

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (accountId) fetchGroups(); }, [accountId]);

  const toggleGroup = (groupId) => {
    setExpandedGroups(prev => {
      const isExpanded = !prev[groupId];
      if (isExpanded && !periods[groupId]) {
        fetchPeriods(groupId);
      }
      return { ...prev, [groupId]: isExpanded };
    });
  };

  const handleCreateGroup = async (e) => {
    e.preventDefault();
    try {
      await api.createGradingPeriodGroup(accountId, { title: newGroupTitle });
      setNewGroupTitle('');
      setShowCreateGroup(false);
      fetchGroups();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDeleteGroup = async (groupId) => {
    if (!window.confirm(t('gradingPeriods.deleteGroupConfirm'))) return;
    try {
      await api.deleteGradingPeriodGroup(accountId, groupId);
      fetchGroups();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleCreatePeriod = async (e, groupId) => {
    e.preventDefault();
    try {
      await api.createGradingPeriod(accountId, groupId, {
        title: newPeriod.title,
        start_date: new Date(newPeriod.start_date).toISOString(),
        end_date: new Date(newPeriod.end_date).toISOString(),
        ...(newPeriod.close_date ? { close_date: new Date(newPeriod.close_date).toISOString() } : {}),
        ...(newPeriod.weight !== '' ? { weight: Number(newPeriod.weight) } : {}),
      });
      setNewPeriod({ title: '', start_date: '', end_date: '', close_date: '', weight: '' });
      setShowCreatePeriod(null);
      fetchPeriods(groupId);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDeletePeriod = async (groupId, periodId) => {
    if (!window.confirm(t('gradingPeriods.deletePeriodConfirm'))) return;
    try {
      await api.deleteGradingPeriod(accountId, groupId, periodId);
      fetchPeriods(groupId);
    } catch (err) {
      setError(err.message);
    }
  };

  // G025: weighted grading periods and per-period weights / close dates
  // drive the course grade calculation.
  const handleGroupFlag = async (group, field, value) => {
    try {
      await api.updateGradingPeriodGroup(accountId, group.id, { [field]: value });
      setGroups((prev) => prev.map((g) => (g.id === group.id ? { ...g, [field]: value } : g)));
    } catch (err) {
      setError(err.message);
    }
  };

  const handlePeriodField = async (groupId, period, field, raw) => {
    let value = raw;
    if (field === 'weight') value = raw === '' ? null : Number(raw);
    if (field === 'close_date') value = raw ? new Date(raw).toISOString() : null;
    if (field === 'is_closed') value = !!raw;
    try {
      await api.updateGradingPeriod(accountId, groupId, period.id, { [field]: value });
      fetchPeriods(groupId);
    } catch (err) {
      setError(err.message);
    }
  };

  const formatDate = (dateStr) => {
    if (!dateStr) return '';
    return new Date(dateStr).toLocaleDateString();
  };

  if (loading) {
    return <Layout><div className="flex items-center justify-center py-12 gap-2 text-text-tertiary">
  <svg className="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" /><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" /></svg>
  {t('gradingPeriods.loading')}
</div></Layout>;
  }

  return (
    <Layout>
      <div className="mb-6">
        <Link to="/" className="text-brand-600 hover:underline text-sm">{t('gradingPeriods.backToDashboard')}</Link>
        <div className="flex items-center justify-between mt-2">
          <h2 className="text-2xl font-bold">{t('gradingPeriods.title')}</h2>
          <button
            onClick={() => setShowCreateGroup(!showCreateGroup)}
            className="flex items-center space-x-2 bg-brand-600 text-white px-4 py-2 rounded-md hover:bg-brand-700 text-sm"
          >
            <Plus className="w-4 h-4" />
            <span>{t('gradingPeriods.newGroup')}</span>
          </button>
        </div>
      </div>

      {error && <div className="bg-accent-danger/10 text-accent-danger p-3 rounded mb-4">{error}</div>}

      {showCreateGroup && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
          <form onSubmit={handleCreateGroup} className="flex items-center space-x-3">
            <input
              type="text"
              placeholder={t('gradingPeriods.groupTitlePlaceholder')}
              value={newGroupTitle}
              onChange={e => setNewGroupTitle(e.target.value)}
              className="flex-1 border border-border-strong rounded px-3 py-2 text-sm"
              required
            />
            <button type="submit" className="bg-brand-600 text-white px-4 py-2 rounded text-sm hover:bg-brand-700">{t('common.create')}</button>
            <button type="button" onClick={() => setShowCreateGroup(false)} className="text-text-tertiary text-sm">{t('common.cancel')}</button>
          </form>
        </div>
      )}

      <div className="space-y-3">
        {groups.length === 0 ? (
          <div className="bg-surface-0 rounded-lg shadow p-8 text-center text-text-tertiary">{t('gradingPeriods.noGroups')}</div>
        ) : (
          groups.map(group => (
            <div key={group.id} className="bg-surface-0 rounded-lg shadow">
              <div className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-surface-1" onClick={() => toggleGroup(group.id)}>
                <div className="flex items-center space-x-3">
                  {expandedGroups[group.id] ? <ChevronDown className="w-5 h-5 text-text-disabled" /> : <ChevronRight className="w-5 h-5 text-text-disabled" />}
                  <Calendar className="w-5 h-5 text-indigo-500" />
                  <span className="font-medium">{group.title}</span>
                </div>
                <button
                  onClick={(e) => { e.stopPropagation(); handleDeleteGroup(group.id); }}
                  className="text-text-disabled hover:text-accent-danger"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>

              {expandedGroups[group.id] && (
                <div className="border-t px-4 py-3">
                  <div className="flex flex-wrap gap-4 mb-3 text-sm">
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={!!group.weighted}
                        onChange={(e) => handleGroupFlag(group, 'weighted', e.target.checked)}
                      />
                      Weight final grade by grading period
                    </label>
                    <label className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={!!group.display_totals_for_all_grading_periods}
                        onChange={(e) => handleGroupFlag(group, 'display_totals_for_all_grading_periods', e.target.checked)}
                      />
                      Show totals for &ldquo;All grading periods&rdquo;
                    </label>
                  </div>
                  {periods[group.id]?.length === 0 && (
                    <p className="text-sm text-text-tertiary mb-3">{t('gradingPeriods.noPeriodsInGroup')}</p>
                  )}
                  {periods[group.id]?.map(period => (
                    <div key={period.id} className="flex items-center justify-between py-2 border-b last:border-0">
                      <div>
                        <p className="text-sm font-medium">{period.title}</p>
                        <p className="text-xs text-text-tertiary">
                          {formatDate(period.start_date)} - {formatDate(period.end_date)}
                          {period.close_date ? ` · closes ${formatDate(period.close_date)}` : ''}
                        </p>
                        <div className="mt-1 flex flex-wrap items-center gap-3 text-xs">
                          {group.weighted && (
                            <label className="flex items-center gap-1">
                              Weight %
                              <input
                                type="number"
                                min="0"
                                step="0.01"
                                defaultValue={period.weight ?? ''}
                                aria-label={`Weight for ${period.title}`}
                                onBlur={(e) => {
                                  if (String(period.weight ?? '') !== e.target.value) handlePeriodField(group.id, period, 'weight', e.target.value);
                                }}
                                className="w-20 border border-border-strong rounded px-1 py-0.5"
                              />
                            </label>
                          )}
                          <label className="flex items-center gap-1">
                            Close date
                            <input
                              type="date"
                              defaultValue={period.close_date ? String(period.close_date).slice(0, 10) : ''}
                              aria-label={`Close date for ${period.title}`}
                              onBlur={(e) => {
                                const was = period.close_date ? String(period.close_date).slice(0, 10) : '';
                                if (was !== e.target.value) handlePeriodField(group.id, period, 'close_date', e.target.value);
                              }}
                              className="border border-border-strong rounded px-1 py-0.5"
                            />
                          </label>
                          <label className="flex items-center gap-1">
                            <input
                              type="checkbox"
                              checked={!!period.is_closed}
                              onChange={(e) => handlePeriodField(group.id, period, 'is_closed', e.target.checked)}
                            />
                            Closed
                          </label>
                        </div>
                      </div>
                      <button
                        onClick={() => handleDeletePeriod(group.id, period.id)}
                        className="text-text-disabled hover:text-accent-danger"
                      >
                        <Trash2 className="w-3 h-3" />
                      </button>
                    </div>
                  ))}

                  {showCreatePeriod === group.id ? (
                    <form onSubmit={(e) => handleCreatePeriod(e, group.id)} className="mt-3 space-y-2">
                      <input
                        type="text"
                        placeholder={t('gradingPeriods.periodTitlePlaceholder')}
                        value={newPeriod.title}
                        onChange={e => setNewPeriod({ ...newPeriod, title: e.target.value })}
                        className="w-full border border-border-strong rounded px-3 py-2 text-sm"
                        required
                      />
                      <div className="grid grid-cols-2 gap-2">
                        <input
                          type="date"
                          value={newPeriod.start_date}
                          onChange={e => setNewPeriod({ ...newPeriod, start_date: e.target.value })}
                          className="border border-border-strong rounded px-3 py-2 text-sm"
                          required
                        />
                        <input
                          type="date"
                          value={newPeriod.end_date}
                          onChange={e => setNewPeriod({ ...newPeriod, end_date: e.target.value })}
                          className="border border-border-strong rounded px-3 py-2 text-sm"
                          required
                        />
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <label className="text-xs text-text-tertiary">
                          Close date (grades lock)
                          <input
                            type="date"
                            value={newPeriod.close_date}
                            onChange={e => setNewPeriod({ ...newPeriod, close_date: e.target.value })}
                            className="mt-1 w-full border border-border-strong rounded px-3 py-2 text-sm"
                          />
                        </label>
                        {group.weighted && (
                          <label className="text-xs text-text-tertiary">
                            Weight %
                            <input
                              type="number"
                              min="0"
                              step="0.01"
                              value={newPeriod.weight}
                              onChange={e => setNewPeriod({ ...newPeriod, weight: e.target.value })}
                              className="mt-1 w-full border border-border-strong rounded px-3 py-2 text-sm"
                            />
                          </label>
                        )}
                      </div>
                      <div className="flex space-x-2">
                        <button type="submit" className="bg-brand-600 text-white px-3 py-1 rounded text-sm hover:bg-brand-700">{t('common.add')}</button>
                        <button type="button" onClick={() => setShowCreatePeriod(null)} className="text-text-tertiary text-sm">{t('common.cancel')}</button>
                      </div>
                    </form>
                  ) : (
                    <button
                      onClick={() => setShowCreatePeriod(group.id)}
                      className="mt-2 text-brand-600 hover:underline text-sm"
                    >
                      {t('gradingPeriods.addPeriod')}
                    </button>
                  )}
                </div>
              )}
            </div>
          ))
        )}
      </div>
      {accountId && <AccountGradingSchemesPanel accountId={accountId} />}
    </Layout>
  );
};

export default GradingPeriodsPage;
