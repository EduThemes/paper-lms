import React, { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { Trophy, AlertTriangle, Pencil } from 'lucide-react';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import LeaderboardTable from '../components/gamification/LeaderboardTable';
import NextToBeatBanner from '../components/gamification/NextToBeatBanner';
import WindowModeToggle from '../components/gamification/WindowModeToggle';
import PseudonymPicker from '../components/gamification/PseudonymPicker';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';

// CourseLeaderboardPage renders the per-course ranking. The page is
// intentionally thin: it asks the server for data and renders whatever
// comes back. Tenant-mode / pseudonym / top-N gating is server-side
// policy (W3-B's leaderboard_render_policy.go).
//
// Sprint 8 additions on top of W3:
//   * NextToBeatBanner — motivational callout when data.next_to_beat
//     is present (relative-window responses).
//   * WindowModeToggle — "This week" (live) vs "Last week" (snapshot).
//   * PseudonymPicker — opens when the learner clicks "Change my name."
//     The button is only rendered when the server's /pseudonym_pools
//     endpoint returns 200 (i.e., the tenant policy allows switching).
//     K-5 / M68 tenants will 403 and the button stays hidden.
export default function CourseLeaderboardPage() {
  const { courseId } = useParams();
  const { user } = useAuth();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [currencies, setCurrencies] = useState([]);
  const [currencyCode, setCurrencyCode] = useState('xp');
  const [windowMode, setWindowMode] = useState('current'); // 'current' | 'last'
  const [pickerOpen, setPickerOpen] = useState(false);
  const [canSwitchName, setCanSwitchName] = useState(false);

  const loadCurrencies = useCallback(async () => {
    try {
      const list = await api.gamification.listCurrencies({});
      const visible = (list?.currencies || list || []).filter((c) => c.visible_to_student !== false);
      setCurrencies(visible);
    } catch (err) {
      console.error('CourseLeaderboardPage: failed to load currencies', err);
    }
  }, []);

  // Probe the pseudonym-pools endpoint to learn whether this tenant
  // allows learner switching. The server gates 403 when policy says
  // LearnerCanSwitch=false, which is exactly the condition where we
  // should hide the "Change my name" affordance.
  const probeSwitchAllowed = useCallback(async () => {
    try {
      await api.gamification.getPseudonymPools(courseId);
      setCanSwitchName(true);
    } catch (err) {
      // 403 / 404 → policy denies. Hide the affordance.
      setCanSwitchName(false);
    }
  }, [courseId]);

  const loadBoard = useCallback(async () => {
    if (!courseId) return;
    setLoading(true);
    setError(null);
    try {
      const offsetWeeks = windowMode === 'last' ? 1 : 0;
      const board = await api.gamification.getCourseLeaderboard(courseId, {
        currency: currencyCode,
        limit: 100,
        offsetWeeks,
      });
      setData(board);
    } catch (err) {
      console.error('CourseLeaderboardPage: failed to load leaderboard', err);
      // Distinguish "no snapshot yet" from real errors so the UI can
      // explain (and offer a way back to the live window).
      if (err.status === 404 && windowMode === 'last') {
        setError("Last week's leaderboard isn't published yet. Try again later, or switch back to This week.");
      } else {
        setError(err.message || 'Could not load the leaderboard.');
      }
      setData(null);
    } finally {
      setLoading(false);
    }
  }, [courseId, currencyCode, windowMode]);

  useEffect(() => {
    loadCurrencies();
    probeSwitchAllowed();
  }, [loadCurrencies, probeSwitchAllowed]);

  useEffect(() => {
    loadBoard();
  }, [loadBoard]);

  return (
    <Layout>
      <CourseNav />
      <div className="max-w-3xl mx-auto py-6 space-y-4">
        <header className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <Trophy className="w-6 h-6 text-text-primary" aria-hidden="true" />
            <h1 className="text-2xl font-semibold text-text-primary">Leaderboard</h1>
          </div>
          {canSwitchName && (
            <button
              type="button"
              onClick={() => setPickerOpen(true)}
              className="inline-flex items-center gap-1.5 rounded-md border border-surface-raised bg-surface-1 hover:bg-surface-2 px-3 py-1.5 text-sm text-text-primary"
            >
              <Pencil className="w-4 h-4" aria-hidden="true" />
              Change my name
            </button>
          )}
        </header>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <WindowModeToggle mode={windowMode} onChange={setWindowMode} disabled={loading} />
          {currencies.length > 1 && (
            <label className="flex items-center gap-2 text-sm text-text-secondary">
              <span>Rank by</span>
              <select
                value={currencyCode}
                onChange={(e) => setCurrencyCode(e.target.value)}
                className="rounded border border-surface-raised bg-surface-0 px-2 py-1 text-text-primary"
              >
                {currencies.map((c) => (
                  <option key={c.id || c.code} value={c.code}>
                    {c.display_label || c.code}
                  </option>
                ))}
              </select>
            </label>
          )}
        </div>

        {error && (
          <div className="flex items-start gap-2 rounded-lg border border-accent-warning bg-accent-warning/10 p-3 text-sm text-accent-warning">
            <AlertTriangle className="w-4 h-4 mt-0.5" aria-hidden="true" />
            <span>{error}</span>
          </div>
        )}

        <NextToBeatBanner nextToBeat={data?.next_to_beat} />

        <LeaderboardTable
          rows={data?.rows || []}
          currencyLabel={data?.currency_label || 'XP'}
          viewerId={user?.id || null}
          loading={loading}
          emptyHint="No learners have earned any points yet."
        />

        {data && data.total_candidates > (data.rows?.length || 0) && data.window_kind !== 'relative' && (
          <p className="text-xs text-text-secondary">
            Showing {data.rows.length} of {data.total_candidates} ranked learners.
          </p>
        )}
      </div>

      <PseudonymPicker
        courseId={courseId}
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
        onSaved={() => {
          // Refresh the leaderboard so the new name appears.
          loadBoard();
        }}
      />
    </Layout>
  );
}
