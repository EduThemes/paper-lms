import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link2, Copy, Check, Trash2, MessageSquare, Lock } from 'lucide-react';
import { api } from '../../services/api';
import { portfolioApi } from '../../services/api_learner_outcomes';
import { sharedPortfolioUrl } from '../../utils/portfolioSharing';

/**
 * Owner controls for sharing and feedback (G155): a private link that
 * works without publishing, turning viewer comments on/off, and the
 * feedback left on the portfolio (deletable).
 *
 * Props: portfolio (normalized, with has_share_link / comments_disabled;
 * share_token only right after a link is created — the server stores a
 * digest and cannot return it later),
 * onPortfolioChange(patch) to merge server changes into the editor state.
 */
export default function PortfolioSharingPanel({ portfolio, onPortfolioChange }) {
  const qc = useQueryClient();
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState(null);
  const commentsKey = ['portfolio', String(portfolio.id), 'comments'];

  const comments = useQuery({
    queryKey: commentsKey,
    queryFn: async () => {
      const res = await api.getPortfolioComments(portfolio.id, 1, 100);
      return Array.isArray(res?.data) ? res.data : [];
    },
  });

  const createLink = useMutation({
    mutationFn: () => portfolioApi.createShareLink(portfolio.id),
    onSuccess: (res) => onPortfolioChange({ share_token: res.share_token, has_share_link: true }),
    onError: (err) => setError(err.message),
  });
  const revokeLink = useMutation({
    mutationFn: () => portfolioApi.revokeShareLink(portfolio.id),
    onSuccess: () => onPortfolioChange({ share_token: undefined, has_share_link: false }),
    onError: (err) => setError(err.message),
  });
  const toggleComments = useMutation({
    mutationFn: (disabled) => api.updatePortfolio(portfolio.id, { comments_disabled: disabled }),
    onSuccess: (_res, disabled) => onPortfolioChange({ comments_disabled: disabled }),
    onError: (err) => setError(err.message),
  });
  const deleteComment = useMutation({
    mutationFn: (commentId) => portfolioApi.deleteComment(portfolio.id, commentId),
    onSuccess: () => qc.invalidateQueries({ queryKey: commentsKey }),
    onError: (err) => setError(err.message),
  });

  const link = portfolio.share_token ? sharedPortfolioUrl(portfolio.share_token, window.location.origin) : null;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(link);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setError('Could not copy the link.');
    }
  };

  return (
    <section aria-labelledby="sharing-heading" className="rounded-2xl border border-border-default bg-surface-0 p-5 space-y-4">
      <h2 id="sharing-heading" className="text-sm font-semibold text-text-primary flex items-center gap-2">
        <Lock className="w-4 h-4" aria-hidden="true" /> Sharing & feedback
      </h2>
      {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}

      <div className="space-y-2">
        <p className="text-sm text-text-secondary">
          A private link lets anyone you send it to view this portfolio, even while it isn&apos;t published.
        </p>
        {link ? (
          <div className="flex flex-wrap items-center gap-2">
            <input readOnly value={link} aria-label="Private sharing link" className="flex-1 min-w-0 px-3 py-1.5 text-sm border border-border-default rounded-lg bg-surface-1" />
            <button type="button" onClick={copy} className="inline-flex items-center gap-1 px-3 py-1.5 text-sm rounded-lg border border-border-strong hover:bg-surface-1">
              {copied ? <Check className="w-4 h-4" aria-hidden="true" /> : <Copy className="w-4 h-4" aria-hidden="true" />} {copied ? 'Copied' : 'Copy'}
            </button>
            <button type="button" onClick={() => createLink.mutate()} className="px-3 py-1.5 text-sm rounded-lg border border-border-strong hover:bg-surface-1">New link</button>
            <button type="button" onClick={() => revokeLink.mutate()} className="px-3 py-1.5 text-sm rounded-lg text-accent-danger hover:bg-accent-danger/10">Turn off</button>
          </div>
        ) : portfolio.has_share_link ? (
          // The server keeps only a digest of the link's secret, so an
          // existing link can't be shown again after the page reloads.
          <div className="space-y-2">
            <p className="text-sm text-text-secondary">
              A private link is active. For security it is only shown when it is created: make a new link to copy it (the old one stops working), or turn it off.
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <button type="button" onClick={() => createLink.mutate()} disabled={createLink.isPending} className="px-3 py-1.5 text-sm rounded-lg border border-border-strong hover:bg-surface-1 disabled:opacity-50">New link</button>
              <button type="button" onClick={() => revokeLink.mutate()} className="px-3 py-1.5 text-sm rounded-lg text-accent-danger hover:bg-accent-danger/10">Turn off</button>
            </div>
          </div>
        ) : (
          <button type="button" onClick={() => createLink.mutate()} disabled={createLink.isPending}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg bg-brand-600 text-white hover:bg-brand-700 disabled:opacity-50">
            <Link2 className="w-4 h-4" aria-hidden="true" /> Create private link
          </button>
        )}
      </div>

      <label className="flex items-center gap-2 text-sm text-text-secondary">
        <input type="checkbox" checked={!portfolio.comments_disabled} onChange={(e) => toggleComments.mutate(!e.target.checked)} />
        Let viewers leave feedback
      </label>

      <div>
        <h3 className="text-sm font-semibold text-text-primary flex items-center gap-2 mb-2">
          <MessageSquare className="w-4 h-4" aria-hidden="true" /> Feedback ({(comments.data || []).length})
        </h3>
        {comments.isLoading ? (
          <p className="text-sm text-text-tertiary">Loading…</p>
        ) : (comments.data || []).length === 0 ? (
          <p className="text-sm text-text-tertiary">No feedback yet.</p>
        ) : (
          <ul className="space-y-2 max-h-64 overflow-y-auto">
            {comments.data.map((c) => (
              <li key={c.id} className="rounded-lg bg-surface-1 px-3 py-2 text-sm flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="font-medium text-text-primary">{c.user_name || 'Viewer'}
                    <span className="ml-2 text-xs font-normal text-text-tertiary">{c.created_at ? new Date(c.created_at).toLocaleString() : ''}</span>
                  </p>
                  <p className="text-text-secondary whitespace-pre-line break-words">{c.content}</p>
                </div>
                <button type="button" aria-label="Delete feedback" onClick={() => deleteComment.mutate(c.id)} className="p-1 text-text-disabled hover:text-accent-danger">
                  <Trash2 className="w-4 h-4" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
