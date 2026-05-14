import React, { useCallback, useEffect, useState } from 'react';
import { api } from '../../services/api';
import { useAuth } from '../../contexts/AuthContext';
import { CurrencyIcon } from './currencyIcon';
import WalletDrawer from './WalletDrawer';

function formatBalance(n) {
  if (typeof n !== 'number') return '0';
  if (n >= 100_000) return `${Math.floor(n / 1000)}k`;
  if (n >= 10_000) return `${(n / 1000).toFixed(1)}k`;
  return n.toLocaleString();
}

// CurrencyPills renders the signed-in user's topbar currencies as a row of
// small icon+balance buttons. Click a pill → opens WalletDrawer scoped to
// that currency. Filters server-side via topbar_only=true on the currencies
// endpoint, then to balances that exist for that user (no zero pills).
//
// `mode` controls the chrome:
//   - 'strip' (default): renders the full sticky subheader (border, bg,
//     height) so the parent layout can mount us with a single
//     <CurrencyPills /> and the strip vanishes when the user has no
//     topbar balances.
//   - 'inline': renders only the pill row; caller owns the chrome. Used
//     by tests and any future contexts that want pills without a strip.
export default function CurrencyPills({ mode = 'strip' }) {
  const { user } = useAuth();
  const [balances, setBalances] = useState([]);
  const [loading, setLoading] = useState(false);
  const [openCurrency, setOpenCurrency] = useState(null);

  const userId = user?.id;

  const load = useCallback(async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const wallet = await api.gamification.getUserWallet(userId);
      const topbarOnly = (wallet.balances || []).filter((b) => b.visible_in_topbar);
      topbarOnly.sort((a, b) => (a.display_order ?? 99) - (b.display_order ?? 99));
      setBalances(topbarOnly);
    } catch (err) {
      console.error('CurrencyPills: failed to load wallet', err);
      setBalances([]);
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    const handler = () => load();
    window.addEventListener('wallet:refresh', handler);
    return () => window.removeEventListener('wallet:refresh', handler);
  }, [load]);

  if (!userId) return null;
  if (!loading && balances.length === 0) return null;

  const pillRow = (
    <div className="flex items-center gap-2" role="group" aria-label="Currency balances">
      {balances.map((b) => (
        <button
          key={b.currency_type_id}
          type="button"
          onClick={() => setOpenCurrency(b)}
          title={b.display_label}
          className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border border-surface-raised bg-surface-1 hover:bg-surface-2 text-text-primary text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-brand-400/60"
        >
          <CurrencyIcon
            icon={b.icon}
            color={b.color}
            className="w-4 h-4 flex-shrink-0"
            title={b.display_label}
          />
          <span className="tabular-nums">{formatBalance(b.balance)}</span>
          <span className="sr-only"> {b.display_label_plural || b.display_label}</span>
        </button>
      ))}
    </div>
  );

  const content =
    mode === 'strip' ? (
      <div className="flex justify-end items-center h-10 px-6 border-b border-surface-raised bg-surface-0">
        {pillRow}
      </div>
    ) : (
      pillRow
    );

  return (
    <>
      {content}
      <WalletDrawer
        userId={userId}
        balance={openCurrency}
        open={!!openCurrency}
        onOpenChange={(o) => {
          if (!o) setOpenCurrency(null);
        }}
      />
    </>
  );
}
