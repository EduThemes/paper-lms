import React, { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Award, Settings } from 'lucide-react';
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogClose,
} from '@/components/ui/dialog';
import { api } from '../../services/api';
import { CurrencyIcon } from './currencyIcon';

function formatDelta(n) {
  const sign = n > 0 ? '+' : '';
  return `${sign}${n.toLocaleString()}`;
}

function formatWhen(iso) {
  try {
    const d = new Date(iso);
    return d.toLocaleString();
  } catch {
    return iso;
  }
}

// Human-readable label for a transaction row. The backend stores
// "rule:<id>" / "manual:<actor>" / "seed:<source>" / "spend:<sku>" patterns;
// strip the colon prefix for display, keep the suffix for the operator.
function describeReason(reason) {
  if (!reason) return 'Adjustment';
  const [kind, detail] = reason.split(':', 2);
  switch (kind) {
    case 'rule':
      return detail ? `Rule #${detail}` : 'Rule';
    case 'manual':
      return 'Manual award';
    case 'seed':
      return 'Initial grant';
    case 'spend':
      return detail ? `Spent: ${detail}` : 'Spent';
    default:
      return reason;
  }
}

// WalletDrawer slides in from the right and shows the transaction history
// for a single currency. The drawer always renders to keep Radix focus-trap
// behavior consistent; an `open` prop drives visibility.
export default function WalletDrawer({ userId, balance, open, onOpenChange }) {
  const [transactions, setTransactions] = useState([]);
  const [page, setPage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const currencyTypeId = balance?.currency_type_id;

  const loadPage = useCallback(
    async (nextPage) => {
      if (!userId || !currencyTypeId) return;
      setLoading(true);
      setError(null);
      try {
        const result = await api.gamification.listUserWalletTransactions(userId, currencyTypeId, {
          page: nextPage,
          perPage: 20,
        });
        setPage(result.page);
        setTotalCount(result.total_count || 0);
        setTransactions((prev) =>
          nextPage === 1 ? result.transactions || [] : [...prev, ...(result.transactions || [])],
        );
      } catch (err) {
        console.error('WalletDrawer: failed to load transactions', err);
        setError('Could not load transaction history.');
      } finally {
        setLoading(false);
      }
    },
    [userId, currencyTypeId],
  );

  useEffect(() => {
    if (open && currencyTypeId) {
      setTransactions([]);
      setPage(1);
      setTotalCount(0);
      loadPage(1);
    }
  }, [open, currencyTypeId, loadPage]);

  const canLoadMore = transactions.length < totalCount;

  // Side-drawer variant of the shared <Dialog>. We override the centering
  // classes baked into DialogContent (left-[50%] top-[50%] translate-x/y)
  // so the panel slides in from the right edge of the viewport.
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="left-auto right-0 top-0 translate-x-0 translate-y-0 h-full w-full max-w-md max-w-none rounded-none border-0 border-l border-surface-raised bg-surface-0 shadow-xl flex flex-col p-0 gap-0 data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right sm:rounded-none"
        aria-describedby={undefined}
      >
          <header className="flex items-center gap-3 px-5 py-4 border-b border-surface-raised">
            {balance && (
              <CurrencyIcon
                icon={balance.icon}
                color={balance.color}
                className="w-6 h-6 flex-shrink-0"
                title={balance.display_label}
              />
            )}
            <div className="flex-1 min-w-0">
              <DialogTitle className="text-base font-semibold text-text-primary truncate">
                {balance?.display_label || 'Wallet'}
              </DialogTitle>
              <div className="text-sm text-text-secondary tabular-nums">
                {typeof balance?.balance === 'number'
                  ? `${balance.balance.toLocaleString()} ${balance.display_label_plural || balance.display_label}`
                  : ''}
                {typeof balance?.lifetime_earned === 'number' && balance.lifetime_earned !== balance.balance && (
                  <span className="ml-2 text-text-tertiary">
                    · {balance.lifetime_earned.toLocaleString()} lifetime
                  </span>
                )}
              </div>
            </div>
          </header>

          <div className="flex-1 overflow-y-auto px-5 py-3">
            {error && (
              <div className="text-sm text-accent-danger border border-accent-danger rounded-md px-3 py-2 mb-3">
                {error}
              </div>
            )}

            {!error && transactions.length === 0 && !loading && (
              <div className="text-sm text-text-tertiary text-center py-8">
                No transactions yet.
              </div>
            )}

            <ul className="divide-y divide-surface-raised">
              {transactions.map((tx) => (
                <li key={tx.id} className="py-3 flex items-start gap-3">
                  <span
                    className={
                      'tabular-nums text-sm font-semibold w-16 text-right flex-shrink-0 ' +
                      (tx.delta >= 0 ? 'text-accent-success' : 'text-accent-danger')
                    }
                  >
                    {formatDelta(tx.delta)}
                  </span>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm text-text-primary truncate">{describeReason(tx.reason)}</div>
                    <div className="text-xs text-text-tertiary">{formatWhen(tx.occurred_at)}</div>
                  </div>
                </li>
              ))}
            </ul>

            {loading && (
              <div className="text-sm text-text-tertiary text-center py-4">Loading…</div>
            )}

            {!loading && canLoadMore && (
              <button
                type="button"
                onClick={() => loadPage(page + 1)}
                className="mt-2 w-full py-2 text-sm rounded-md border border-surface-raised text-text-secondary hover:bg-surface-2 focus:outline-none focus:ring-2 focus:ring-brand-400/60"
              >
                Load more
              </button>
            )}
          </div>

          <footer className="px-5 py-3 border-t border-surface-raised flex items-center gap-4">
            <DialogClose asChild>
              <Link
                to="/profile/badges"
                className="inline-flex items-center gap-1.5 text-xs text-text-secondary hover:text-text-primary"
              >
                <Award className="w-3.5 h-3.5" />
                My badges
              </Link>
            </DialogClose>
            <DialogClose asChild>
              <Link
                to="/profile/gamification"
                className="inline-flex items-center gap-1.5 text-xs text-text-secondary hover:text-text-primary"
              >
                <Settings className="w-3.5 h-3.5" />
                Privacy settings
              </Link>
            </DialogClose>
          </footer>
      </DialogContent>
    </Dialog>
  );
}
