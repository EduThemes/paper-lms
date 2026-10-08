import React, { useMemo, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  Folder, File, Upload, Trash2, ChevronRight, Plus, X, Settings, Lock, EyeOff, Eye, Clock, HardDrive,
} from 'lucide-react';
import { api } from '../services/api';
import { pagesFilesApi } from '../services/api_pages_files';
import useIsTeacher from '../hooks/useIsTeacher';
import Layout from '../components/Layout';
import CourseNav from '../components/CourseNav';
import ListPager from '../components/ListPager';
import { Skeleton } from '@/components/ui/skeleton';
import FileSettingsDialog from '../components/files/FileSettingsDialog';
import FolderSettingsDialog from '../components/files/FolderSettingsDialog';
import { buildFolderTree, folderPath, formatBytes, quotaSummary, fileStatus } from '../utils/fileTree';

const FILES_PER_PAGE = 50;

const STATUS_STYLE = {
  unpublished: 'text-text-tertiary bg-surface-2',
  scheduled: 'text-accent-warning bg-accent-warning/10',
  closed: 'text-text-tertiary bg-surface-2',
  hidden: 'text-brand-700 bg-brand-50',
  published: 'text-accent-success bg-accent-success/10',
};

const STATUS_ICON = { unpublished: EyeOff, scheduled: Clock, closed: Lock, hidden: EyeOff, published: Eye };

function QuotaBar({ quota }) {
  const s = quotaSummary(quota);
  if (!s) return null;
  return (
    <div className="flex items-center gap-3 text-xs text-text-tertiary" aria-label="Course storage">
      <HardDrive className="w-4 h-4" aria-hidden="true" />
      <div className="w-40 h-2 rounded-full bg-surface-2 overflow-hidden" role="progressbar" aria-valuenow={s.pct} aria-valuemin={0} aria-valuemax={100} aria-label="Storage used">
        <div className={`h-full ${s.nearlyFull ? 'bg-accent-danger' : 'bg-brand-500'}`} style={{ width: `${s.pct}%` }} />
      </div>
      <span>{s.used} of {s.total} used</span>
    </div>
  );
}

// FilesPage: the course file manager (G070-G072). Folders form a tree
// under the course's root folder; teachers upload into, create, rename,
// move, publish / hide and delete folders and files, set availability
// dates and usage rights, and replace a file's content in place.
// Students see only what is published and not hidden.
const FilesPage = () => {
  const { courseId } = useParams();
  const isTeacher = useIsTeacher(courseId);
  const queryClient = useQueryClient();
  const [currentFolderId, setCurrentFolderId] = useState(null);
  const [filePage, setFilePage] = useState(1);
  const [showNewFolder, setShowNewFolder] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [editingFile, setEditingFile] = useState(null);
  const [editingFolder, setEditingFolder] = useState(null);
  const [actionError, setActionError] = useState(null);

  const treeKey = ['courses', String(courseId), 'folders', 'tree'];
  const filesKey = ['courses', String(courseId), 'files'];
  const foldersQuery = useQuery({ queryKey: treeKey, queryFn: () => pagesFilesApi.listCourseFolderTree(courseId) });
  const filesQuery = useQuery({
    queryKey: [...filesKey, currentFolderId ?? 'root', filePage],
    queryFn: () => pagesFilesApi.listFolderFiles(courseId, currentFolderId, filePage, FILES_PER_PAGE),
    // Keep the old listing on screen while the next folder loads.
    placeholderData: (previous) => previous,
  });
  const quotaQuery = useQuery({
    queryKey: [...filesKey, 'quota'],
    queryFn: () => pagesFilesApi.getCourseQuota(courseId),
    enabled: isTeacher === true,
  });

  const tree = useMemo(() => buildFolderTree(foldersQuery.data || []), [foldersQuery.data]);
  const breadcrumbs = folderPath(tree, currentFolderId);
  const subfolders = tree.childrenOf(currentFolderId);
  const files = filesQuery.data?.data || [];

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: treeKey });
    queryClient.invalidateQueries({ queryKey: filesKey });
  };
  const onError = (err) => setActionError(err.message);
  const mutationOpts = (extra = {}) => ({ onSuccess: () => { setActionError(null); refresh(); extra.onSuccess?.(); }, onError });

  const upload = useMutation({ mutationFn: (file) => api.uploadCourseFile(courseId, file, currentFolderId), ...mutationOpts() });
  const createFolder = useMutation({
    mutationFn: (name) => api.createCourseFolder(courseId, { name, parent_folder_id: currentFolderId || null }),
    ...mutationOpts({ onSuccess: () => { setNewFolderName(''); setShowNewFolder(false); } }),
  });
  const updateFile = useMutation({
    mutationFn: ({ id, fields }) => pagesFilesApi.updateFile(courseId, id, fields),
    ...mutationOpts({ onSuccess: () => setEditingFile(null) }),
  });
  const replaceFile = useMutation({
    mutationFn: ({ id, file }) => pagesFilesApi.replaceFile(courseId, id, file),
    ...mutationOpts({ onSuccess: () => setEditingFile(null) }),
  });
  const deleteFile = useMutation({ mutationFn: (id) => api.deleteFile(courseId, id), ...mutationOpts() });
  const updateFolder = useMutation({
    mutationFn: ({ id, fields }) => pagesFilesApi.updateFolder(id, fields),
    ...mutationOpts({ onSuccess: () => setEditingFolder(null) }),
  });
  const deleteFolder = useMutation({
    mutationFn: (id) => api.deleteFolder(id),
    ...mutationOpts({ onSuccess: () => setCurrentFolderId(null) }),
  });

  const openFolder = (id) => {
    setFilePage(1);
    setCurrentFolderId(id);
  };

  const loading = foldersQuery.isLoading || filesQuery.isLoading;
  const loadError = foldersQuery.error || filesQuery.error;

  if (loading) {
    return (
      <Layout>
        <CourseNav />
        <div className="space-y-3 p-6">
          <Skeleton className="h-9 w-48" />
          <Skeleton className="h-12 w-full" />
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full" />
          ))}
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <CourseNav />
      <div className="mb-6 flex flex-wrap items-end justify-between gap-3">
        <div>
          <Link to={`/courses/${courseId}`} className="text-brand-600 hover:underline text-sm">&larr; Back to Course</Link>
          <h2 className="text-2xl font-bold text-text-primary mt-2">Files</h2>
        </div>
        {isTeacher && <QuotaBar quota={quotaQuery.data} />}
      </div>

      {(loadError || actionError) && (
        <div role="alert" className="bg-accent-danger/10 border border-accent-danger/30 text-accent-danger px-4 py-3 rounded mb-4">
          {actionError || loadError.message}
        </div>
      )}

      <nav aria-label="Folder path" className="flex items-center flex-wrap gap-1 text-sm text-text-secondary mb-4">
        <button onClick={() => openFolder(null)} className="hover:text-brand-600 font-medium">Files</button>
        {breadcrumbs.map((crumb) => (
          <React.Fragment key={crumb.id}>
            <ChevronRight className="w-4 h-4 text-text-disabled" aria-hidden="true" />
            <button onClick={() => openFolder(crumb.id)} className="hover:text-brand-600 font-medium">{crumb.name}</button>
          </React.Fragment>
        ))}
      </nav>

      {isTeacher && (
        <div className="flex items-center gap-3 mb-4">
          <button
            onClick={() => setShowNewFolder(true)}
            className="inline-flex items-center gap-1 bg-surface-0 border border-border-strong text-text-secondary px-3 py-2 rounded-md hover:bg-surface-1 text-sm font-medium shadow-sm"
          >
            <Plus className="w-4 h-4" aria-hidden="true" />
            <span>New Folder</span>
          </button>
          <label className="inline-flex items-center gap-1 bg-brand-600 text-white px-3 py-2 rounded-md hover:bg-brand-700 text-sm font-medium shadow-sm cursor-pointer">
            <Upload className="w-4 h-4" aria-hidden="true" />
            <span>{upload.isPending ? 'Uploading...' : 'Upload File'}</span>
            <input
              type="file"
              className="hidden"
              disabled={upload.isPending}
              onChange={(e) => { const f = e.target.files?.[0]; if (f) upload.mutate(f); e.target.value = ''; }}
            />
          </label>
        </div>
      )}

      {showNewFolder && (
        <div className="bg-surface-0 rounded-lg shadow p-4 mb-4">
          <form
            onSubmit={(e) => { e.preventDefault(); if (newFolderName.trim()) createFolder.mutate(newFolderName.trim()); }}
            className="flex items-center gap-3"
          >
            <Folder className="w-5 h-5 text-text-disabled" aria-hidden="true" />
            <input
              type="text"
              aria-label="Folder name"
              value={newFolderName}
              onChange={(e) => setNewFolderName(e.target.value)}
              placeholder="Folder name"
              className="flex-1 border border-border-strong rounded-md px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
              autoFocus
            />
            <button type="submit" className="bg-brand-600 text-white px-3 py-1.5 rounded-md text-sm hover:bg-brand-700">Create</button>
            <button type="button" aria-label="Cancel" onClick={() => { setShowNewFolder(false); setNewFolderName(''); }} className="text-text-disabled hover:text-text-secondary">
              <X className="w-5 h-5" />
            </button>
          </form>
        </div>
      )}

      <div className="bg-surface-0 rounded-lg shadow">
        {subfolders.length === 0 && files.length === 0 ? (
          <div className="p-6 text-center text-text-tertiary">No files or folders yet.</div>
        ) : (
          <ul className="divide-y">
            {subfolders.map((folder) => (
              <li key={`folder-${folder.id}`} className="flex items-center justify-between px-4 py-3 hover:bg-surface-1">
                <button onClick={() => openFolder(folder.id)} className="flex items-center gap-3 flex-1 text-left">
                  <Folder className="w-5 h-5 text-brand-500" aria-hidden="true" />
                  <span className="text-sm font-medium text-text-primary">{folder.name}</span>
                  {isTeacher && folder.locked && <span className="text-xs px-2 py-0.5 rounded-full text-text-tertiary bg-surface-2">Unpublished</span>}
                  {isTeacher && folder.hidden && <span className="text-xs px-2 py-0.5 rounded-full text-brand-700 bg-brand-50">Hidden</span>}
                </button>
                {isTeacher && (
                  <div className="flex items-center gap-1">
                    <button onClick={() => setEditingFolder(folder)} className="text-text-disabled hover:text-text-secondary p-1" aria-label={`Settings for folder ${folder.name}`}>
                      <Settings className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => { if (window.confirm(`Delete "${folder.name}" and everything in it?`)) deleteFolder.mutate(folder.id); }}
                      className="text-text-disabled hover:text-accent-danger p-1"
                      aria-label={`Delete folder ${folder.name}`}
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                )}
              </li>
            ))}

            {files.map((file) => {
              const status = fileStatus(file);
              const StatusIcon = STATUS_ICON[status.kind];
              const learnerLocked = !isTeacher && file.locked_for_user;
              return (
                <li key={`file-${file.id}`} className="flex items-center justify-between px-4 py-3 hover:bg-surface-1">
                  <div className="flex items-center gap-3 flex-1 min-w-0">
                    {learnerLocked ? <Lock className="w-5 h-5 text-text-disabled" aria-hidden="true" /> : <File className="w-5 h-5 text-text-disabled" aria-hidden="true" />}
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-text-primary truncate">{file.display_name}</p>
                      <p className="text-xs text-text-tertiary">
                        {formatBytes(file.size)} &middot; {file.content_type}
                        {file.usage_rights && <> &middot; {file.usage_rights.license || file.usage_rights.use_justification.replace(/_/g, ' ')}</>}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {(isTeacher || learnerLocked) && (
                      <span className={`inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full ${STATUS_STYLE[status.kind]}`}>
                        <StatusIcon className="w-3 h-3" aria-hidden="true" /> {status.label}
                      </span>
                    )}
                    {isTeacher && (
                      <button
                        onClick={() => updateFile.mutate({ id: file.id, fields: { locked: !file.locked } })}
                        className="text-xs text-text-secondary hover:text-brand-600"
                        aria-label={`${file.locked ? 'Publish' : 'Unpublish'} ${file.display_name}`}
                      >
                        {file.locked ? 'Publish' : 'Unpublish'}
                      </button>
                    )}
                    {!learnerLocked && (
                      <a href={file.url} className="text-brand-600 hover:text-brand-800 text-sm font-medium" download>Download</a>
                    )}
                    {isTeacher && (
                      <>
                        <button onClick={() => setEditingFile(file)} className="text-text-disabled hover:text-text-secondary p-1" aria-label={`Settings for ${file.display_name}`}>
                          <Settings className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => { if (window.confirm('Are you sure you want to delete this file?')) deleteFile.mutate(file.id); }}
                          className="text-text-disabled hover:text-accent-danger p-1"
                          aria-label={`Delete ${file.display_name}`}
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </div>
      <ListPager page={filePage} pagination={filesQuery.data?.pagination || {}} onPageChange={setFilePage} label="Files pagination" />

      {editingFile && (
        <FileSettingsDialog
          key={editingFile.id}
          file={editingFile}
          tree={tree}
          open
          onOpenChange={(open) => { if (!open) setEditingFile(null); }}
          onSave={(fields) => updateFile.mutate({ id: editingFile.id, fields })}
          onReplace={(file) => replaceFile.mutate({ id: editingFile.id, file })}
          saving={updateFile.isPending || replaceFile.isPending}
          error={actionError}
        />
      )}
      {editingFolder && (
        <FolderSettingsDialog
          key={editingFolder.id}
          folder={editingFolder}
          tree={tree}
          open
          onOpenChange={(open) => { if (!open) setEditingFolder(null); }}
          onSave={(fields) => updateFolder.mutate({ id: editingFolder.id, fields })}
          saving={updateFolder.isPending}
          error={actionError}
        />
      )}
    </Layout>
  );
};

export default FilesPage;
