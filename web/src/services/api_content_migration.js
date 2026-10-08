// API calls + react-query hooks for the content-migration lane: course
// copy / package import (content_migrations, content_imports), course
// content exports (content_exports) and blueprint locking.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { request, uploadFile } from './api';
import { anyJobActive, anyMigrationActive } from '../lib/contentMigration';

const enc = encodeURIComponent;

export const contentMigrationApi = {
  // Content migrations (course copy + package import history)
  listMigrations: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/content_migrations?per_page=50`);
    return Array.isArray(data) ? data : [];
  },
  createMigration: async (courseId, body) => {
    const { data } = await request(`/courses/${courseId}/content_migrations`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
    return data;
  },
  copySources: async (courseId, search = '') => {
    const q = search ? `?search=${enc(search)}` : '';
    const { data } = await request(`/courses/${courseId}/content_migrations/copy_sources${q}`);
    return Array.isArray(data) ? data : [];
  },
  selectableContent: async (courseId, sourceCourseId) => {
    const { data } = await request(`/courses/${courseId}/content_migrations/selectable?source_course_id=${enc(sourceCourseId)}`);
    return Array.isArray(data) ? data : [];
  },
  importPackage: async (courseId, file, settings) => {
    const form = new FormData();
    form.append('file', file);
    if (settings) form.append('settings', JSON.stringify(settings));
    return uploadFile(`/courses/${courseId}/content_imports`, form);
  },

  // Content exports
  listExports: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/content_exports?per_page=50`);
    return Array.isArray(data) ? data : [];
  },
  createExport: async (courseId, body = {}) => {
    const { data } = await request(`/courses/${courseId}/content_exports`, {
      method: 'POST',
      body: JSON.stringify({ export_type: 'common_cartridge', ...body }),
    });
    return data;
  },
  exportDownloadUrl: (courseId, exportId) => {
    const base = import.meta.env.VITE_API_URL || '/api/v1';
    return `${base}/courses/${courseId}/content_exports/${exportId}/download`;
  },

  // Blueprint locking
  blueprintContent: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default/content`);
    return data;
  },
  restrictItem: async (courseId, body) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default/restrict_item`, {
      method: 'PUT',
      body: JSON.stringify(body),
    });
    return data;
  },
  unsyncedChanges: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_templates/default/unsynced_changes`);
    return Array.isArray(data) ? data : [];
  },
  blueprintSubscriptions: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_subscriptions?per_page=10`);
    return Array.isArray(data) ? data : [];
  },
  childLockedContent: async (courseId) => {
    const { data } = await request(`/courses/${courseId}/blueprint_subscriptions/locked_content`);
    return Array.isArray(data) ? data : [];
  },
};

export const contentMigrationKeys = {
  migrations: (courseId) => ['contentMigrations', String(courseId)],
  copySources: (courseId, search) => ['contentMigrations', String(courseId), 'copySources', search || ''],
  selectable: (courseId, sourceId) => ['contentMigrations', String(courseId), 'selectable', String(sourceId)],
  exports: (courseId) => ['contentExports', String(courseId)],
  blueprintContent: (courseId) => ['blueprintLocks', String(courseId), 'content'],
  blueprintSubscriptions: (courseId) => ['blueprintLocks', String(courseId), 'subscriptions'],
  childLocked: (courseId) => ['blueprintLocks', String(courseId), 'childLocked'],
  unsynced: (courseId) => ['blueprintLocks', String(courseId), 'unsynced'],
};

export function useUnsyncedChanges(courseId) {
  return useQuery({
    queryKey: contentMigrationKeys.unsynced(courseId),
    queryFn: () => contentMigrationApi.unsyncedChanges(courseId),
    enabled: !!courseId,
  });
}

const POLL_MS = 3000;

// Migrations poll while any is still queued/running.
export function useContentMigrations(courseId) {
  return useQuery({
    queryKey: contentMigrationKeys.migrations(courseId),
    queryFn: () => contentMigrationApi.listMigrations(courseId),
    enabled: !!courseId,
    refetchInterval: (query) => (anyMigrationActive(query.state.data) ? POLL_MS : false),
  });
}

export function useCopySources(courseId, search) {
  return useQuery({
    queryKey: contentMigrationKeys.copySources(courseId, search),
    queryFn: () => contentMigrationApi.copySources(courseId, search),
    enabled: !!courseId,
    staleTime: 30_000,
  });
}

export function useSelectableContent(courseId, sourceCourseId) {
  return useQuery({
    queryKey: contentMigrationKeys.selectable(courseId, sourceCourseId),
    queryFn: () => contentMigrationApi.selectableContent(courseId, sourceCourseId),
    enabled: !!courseId && !!sourceCourseId,
    staleTime: 30_000,
  });
}

export function useCreateMigration(courseId, options = {}) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body) => contentMigrationApi.createMigration(courseId, body),
    ...options,
    onSuccess: (...args) => {
      qc.invalidateQueries({ queryKey: contentMigrationKeys.migrations(courseId) });
      options.onSuccess?.(...args);
    },
  });
}

export function useImportPackage(courseId, options = {}) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ file, settings }) => contentMigrationApi.importPackage(courseId, file, settings),
    ...options,
    onSettled: (...args) => {
      qc.invalidateQueries({ queryKey: contentMigrationKeys.migrations(courseId) });
      options.onSettled?.(...args);
    },
  });
}

export function useContentExports(courseId) {
  return useQuery({
    queryKey: contentMigrationKeys.exports(courseId),
    queryFn: () => contentMigrationApi.listExports(courseId),
    enabled: !!courseId,
    refetchInterval: (query) => (anyJobActive(query.state.data) ? POLL_MS : false),
  });
}

export function useCreateExport(courseId, options = {}) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body) => contentMigrationApi.createExport(courseId, body),
    ...options,
    onSuccess: (...args) => {
      qc.invalidateQueries({ queryKey: contentMigrationKeys.exports(courseId) });
      options.onSuccess?.(...args);
    },
  });
}

export function useBlueprintContent(courseId, enabled = true) {
  return useQuery({
    queryKey: contentMigrationKeys.blueprintContent(courseId),
    queryFn: () => contentMigrationApi.blueprintContent(courseId),
    enabled: !!courseId && enabled,
  });
}

export function useRestrictItem(courseId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body) => contentMigrationApi.restrictItem(courseId, body),
    onSuccess: () => qc.invalidateQueries({ queryKey: contentMigrationKeys.blueprintContent(courseId) }),
  });
}

export function useBlueprintSubscriptions(courseId) {
  return useQuery({
    queryKey: contentMigrationKeys.blueprintSubscriptions(courseId),
    queryFn: () => contentMigrationApi.blueprintSubscriptions(courseId),
    enabled: !!courseId,
  });
}

export function useChildLockedContent(courseId, enabled = true) {
  return useQuery({
    queryKey: contentMigrationKeys.childLocked(courseId),
    queryFn: () => contentMigrationApi.childLockedContent(courseId),
    enabled: !!courseId && enabled,
  });
}
