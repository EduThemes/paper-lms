import React, { useState } from 'react';
import { MoreHorizontal, Check } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '../ui/dropdown-menu';
import { gradingPolicyApi, LATE_STATUSES } from '../../services/api_grading_policy';

// SubmissionStatusMenu is the gradebook cell's status menu (G021 /
// G022): excuse or un-excuse, or set a manual late / missing / extended /
// none status (Automatic goes back to the computed status). A late status
// can carry a "days late" override that drives the late deduction.
const SubmissionStatusMenu = ({ courseId, assignmentId, student, cell, onUpdated, onError }) => {
  const [busy, setBusy] = useState(false);
  const current = cell?.late_policy_status ?? null;
  const excused = !!cell?.excused;

  const run = async (fn) => {
    setBusy(true);
    try {
      const saved = await fn();
      onUpdated?.(student.id, assignmentId, saved);
    } catch (e) {
      onError?.(e.message || 'Could not update the submission.');
    } finally {
      setBusy(false);
    }
  };

  const setStatus = (value) => {
    if (value === 'late') {
      const raw = window.prompt('Days late (used for the late penalty). Leave blank for none.', '');
      if (raw === null) return;
      const days = raw.trim() === '' ? null : Number(raw);
      if (days !== null && (Number.isNaN(days) || days < 0)) {
        onError?.('Days late must be a positive number.');
        return;
      }
      run(() =>
        gradingPolicyApi.setLatePolicyStatus(courseId, assignmentId, student.id, 'late', days === null ? null : Math.round(days * 86400)),
      );
      return;
    }
    run(() => gradingPolicyApi.setLatePolicyStatus(courseId, assignmentId, student.id, value));
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          disabled={busy}
          aria-label={`Status options for ${student.name}`}
          className="absolute bottom-0.5 right-0.5 rounded p-0.5 text-text-tertiary opacity-60 hover:opacity-100 hover:bg-surface-2 focus:opacity-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500"
        >
          <MoreHorizontal className="h-3 w-3" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuLabel className="truncate">{student.name}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => run(() => gradingPolicyApi.setExcused(courseId, assignmentId, student.id, !excused))}>
          {excused ? <Check className="h-4 w-4" /> : <span className="w-4" />}
          Excused
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuLabel className="text-xs text-text-tertiary">Status</DropdownMenuLabel>
        {LATE_STATUSES.map((s) => (
          <DropdownMenuItem key={String(s.value)} onSelect={() => setStatus(s.value)}>
            {current === s.value && !excused ? <Check className="h-4 w-4" /> : <span className="w-4" />}
            {s.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
};

export default SubmissionStatusMenu;
