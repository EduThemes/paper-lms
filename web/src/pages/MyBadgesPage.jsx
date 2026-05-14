import React, { useCallback, useEffect, useState } from 'react';
import { Award } from 'lucide-react';
import Layout from '../components/Layout';
import { useAuth } from '../contexts/AuthContext';
import { api } from '../services/api';
import { BadgeIcon } from '../components/gamification/BadgeIcon';

function formatAwarded(iso) {
  try {
    return new Date(iso).toLocaleDateString();
  } catch {
    return iso;
  }
}

// MyBadgesPage — learner-facing grid of earned badges. Each card shows
// the badge medallion, name, description, and the date earned. Empty
// state explains how badges are earned so a fresh learner isn't
// staring at an empty page.
export default function MyBadgesPage() {
  const { user } = useAuth();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const userId = user?.id;

  const load = useCallback(async () => {
    if (!userId) return;
    setLoading(true);
    setError(null);
    try {
      const result = await api.gamification.listUserBadges(userId);
      setItems(result.badges || []);
    } catch (err) {
      console.error('MyBadgesPage: load failed', err);
      setError(err.message || 'Could not load your badges.');
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <Layout>
      <div className="max-w-4xl mx-auto py-6 space-y-6">
        <header>
          <h1 className="text-xl font-semibold text-text-primary flex items-center gap-2">
            <Award className="w-5 h-5" /> My badges
          </h1>
          <p className="text-sm text-text-secondary mt-1">
            Badges your teachers have awarded you, or that you earned automatically.
          </p>
        </header>

        {error && (
          <div className="text-sm text-accent-danger border border-accent-danger rounded-md px-3 py-2">
            {error}
          </div>
        )}

        {loading ? (
          <div className="text-sm text-text-tertiary">Loading…</div>
        ) : items.length === 0 ? (
          <div className="border border-dashed border-surface-raised rounded-lg bg-surface-0 px-6 py-12 text-center">
            <Award className="w-10 h-10 mx-auto text-text-tertiary mb-3" />
            <div className="text-sm text-text-primary font-medium">No badges yet</div>
            <p className="text-xs text-text-tertiary mt-1">
              Keep going! Badges get awarded for completed work, sustained streaks, or by your teacher.
            </p>
          </div>
        ) : (
          <ul className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
            {items.map((it) => (
              <li
                key={it.award_id}
                className="rounded-lg border border-surface-raised bg-surface-0 p-4 flex items-start gap-3"
              >
                <BadgeIcon badge={it} size="lg" />
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-text-primary truncate">
                    {it.name || it.code || 'Badge'}
                  </div>
                  {it.description && (
                    <div className="text-xs text-text-secondary mt-1 line-clamp-3">{it.description}</div>
                  )}
                  <div className="text-[11px] text-text-tertiary mt-2">
                    Earned {formatAwarded(it.awarded_at)}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Layout>
  );
}
