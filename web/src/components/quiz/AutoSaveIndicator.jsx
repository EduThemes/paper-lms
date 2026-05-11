import React, { useEffect, useState } from 'react';
import { Check, AlertCircle, RefreshCw } from 'lucide-react';

const formatRelative = (date) => {
  if (!date) return '';
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));
  if (seconds < 5) return 'just now';
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ago`;
};

const AutoSaveIndicator = ({ status, lastSavedAt, onRetry }) => {
  const [, setTick] = useState(0);
  useEffect(() => {
    if (!lastSavedAt) return;
    const id = setInterval(() => setTick((t) => t + 1), 5000);
    return () => clearInterval(id);
  }, [lastSavedAt]);

  if (status === 'saving') {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-text-secondary">
        <RefreshCw className="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
        Saving…
      </span>
    );
  }
  if (status === 'saved') {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-accent-success">
        <Check className="w-3.5 h-3.5" aria-hidden="true" />
        Saved
      </span>
    );
  }
  if (status === 'error') {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-accent-danger">
        <AlertCircle className="w-3.5 h-3.5" aria-hidden="true" />
        Save failed
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="ml-1 underline font-medium hover:text-red-900 focus:outline-none focus:ring-2 focus:ring-red-400 rounded"
          >
            retry
          </button>
        )}
      </span>
    );
  }
  if (lastSavedAt) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs text-text-tertiary">
        <Check className="w-3.5 h-3.5" aria-hidden="true" />
        Saved {formatRelative(lastSavedAt)}
      </span>
    );
  }
  return null;
};

export default AutoSaveIndicator;
