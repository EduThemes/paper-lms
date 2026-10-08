import { describe, test, expect } from 'vitest';
import { buildFolderTree, folderPath, moveTargets, formatBytes, quotaSummary, fileStatus } from './fileTree';

const folders = [
  { id: 1, name: 'course files', parent_folder_id: null },
  { id: 2, name: 'Units', parent_folder_id: 1 },
  { id: 3, name: 'Unit 1', parent_folder_id: 2 },
  { id: 4, name: 'Exams', parent_folder_id: 1 },
  { id: 5, name: 'Unit 2', parent_folder_id: 2 },
];

describe('file tree (G071)', () => {
  test('top level is the root folder\'s children, sorted by name', () => {
    const tree = buildFolderTree(folders);
    expect(tree.root.id).toBe(1);
    expect(tree.childrenOf(null).map((f) => f.name)).toEqual(['Exams', 'Units']);
    expect(tree.childrenOf(2).map((f) => f.name)).toEqual(['Unit 1', 'Unit 2']);
  });

  test('breadcrumb path leaves out the root', () => {
    const tree = buildFolderTree(folders);
    expect(folderPath(tree, 3).map((f) => f.name)).toEqual(['Units', 'Unit 1']);
    expect(folderPath(tree, null)).toEqual([]);
  });

  test('a folder cannot be moved into itself or below itself', () => {
    const tree = buildFolderTree(folders);
    const all = moveTargets(tree).map((t) => t.value);
    expect(all).toEqual([null, 4, 2, 3, 5]);
    expect(moveTargets(tree, 2).map((t) => t.value)).toEqual([null, 4]);
  });

  test('a course with no folders yet has an empty top level', () => {
    const tree = buildFolderTree([]);
    expect(tree.childrenOf(null)).toEqual([]);
    expect(moveTargets(tree)).toHaveLength(1);
  });
});

describe('quota and status (G070 / G072)', () => {
  test('formatBytes', () => {
    expect(formatBytes(512)).toBe('512 B');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(500 * 1024 * 1024)).toBe('500 MB');
  });

  test('quotaSummary', () => {
    expect(quotaSummary({ quota: 1000, quota_used: 950 })).toMatchObject({ pct: 95, nearlyFull: true });
    expect(quotaSummary({ quota: 0 })).toBeNull();
  });

  test('fileStatus covers each visibility state', () => {
    const now = new Date('2026-10-08T12:00:00Z');
    expect(fileStatus({ locked: true }, now).kind).toBe('unpublished');
    expect(fileStatus({ unlock_at: '2026-10-10T00:00:00Z' }, now).kind).toBe('scheduled');
    expect(fileStatus({ lock_at: '2026-10-01T00:00:00Z' }, now).kind).toBe('closed');
    expect(fileStatus({ hidden: true }, now).kind).toBe('hidden');
    expect(fileStatus({}, now)).toEqual({ kind: 'published', label: 'Published' });
  });
});
