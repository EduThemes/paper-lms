import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { api } from '../../services/api';
import { pagesFilesApi } from '../../services/api_pages_files';
import { useAuth } from '../../contexts/AuthContext';
import { quotaSummary } from '../../utils/fileTree';

// CourseStorageCard shows the course's file storage use (G072) and lets an
// admin set the course's own quota; teachers see the numbers only (the
// server refuses quota changes from non-admins too).
export default function CourseStorageCard({ courseId, course, onSaved }) {
  const { user } = useAuth();
  const isAdmin = user?.role === 'admin' || user?.role === 'super_admin';
  const queryClient = useQueryClient();
  const quotaKey = ['courses', String(courseId), 'files', 'quota'];
  const quota = useQuery({ queryKey: quotaKey, queryFn: () => pagesFilesApi.getCourseQuota(courseId) });
  const [value, setValue] = useState(course?.storage_quota_mb != null ? String(course.storage_quota_mb) : '');

  const save = useMutation({
    mutationFn: (mb) => api.updateCourse(courseId, { storage_quota_mb: mb }),
    onSuccess: (updated) => {
      queryClient.invalidateQueries({ queryKey: quotaKey });
      if (updated) onSaved?.(updated);
    },
  });

  const summary = quotaSummary(quota.data);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">File storage</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {summary ? (
          <p className="text-sm text-text-secondary">
            {summary.used} of {summary.total} used ({summary.pct}%).
          </p>
        ) : (
          <p className="text-sm text-text-tertiary">{quota.isError ? quota.error.message : 'Loading…'}</p>
        )}
        {isAdmin && (
          <form
            className="flex flex-wrap items-end gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              save.mutate(value.trim() === '' ? null : Number(value));
            }}
          >
            <div>
              <label htmlFor="storage-quota-mb" className="block text-sm font-medium text-text-secondary mb-1">Course quota (MB)</label>
              <input
                id="storage-quota-mb"
                type="number"
                min="1"
                value={value}
                placeholder="Account default"
                onChange={(e) => setValue(e.target.value)}
                className="w-40 border border-border-strong rounded-md px-3 py-2 text-sm"
              />
            </div>
            <button type="submit" disabled={save.isPending} className="px-3 py-2 bg-brand-600 text-on-brand rounded-md text-sm font-medium disabled:opacity-50">
              {save.isPending ? 'Saving…' : 'Save quota'}
            </button>
            {save.isError && <p role="alert" className="w-full text-sm text-accent-danger">{save.error.message}</p>}
            {save.isSuccess && <p role="status" className="w-full text-sm text-accent-success">Quota saved.</p>}
            <p className="w-full text-xs text-text-tertiary">Leave empty to use the account default.</p>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
