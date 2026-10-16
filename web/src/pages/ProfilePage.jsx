import React, { useRef, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ExternalLink, Pencil, Plus, Trash2, Upload, X } from 'lucide-react';
import Layout from '../components/Layout';
import Page from '../components/Page';
import useDocumentTitle from '../hooks/useDocumentTitle';
import { useAuth } from '../contexts/AuthContext';
import { Button } from '@/components/ui/button';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { learnerApi, useProfileCard, useLearnerMutation, learnerKeys } from '../services/api_learner';

// User profile (G161): pronouns, a short bio, links and an avatar. The
// owner edits it at /profile; anyone who shares a course with them (and
// admins) sees it at /users/:userId/profile.

const initials = (name = '') =>
  name.split(' ').filter(Boolean).slice(0, 2).map((p) => p[0].toUpperCase()).join('') || '?';

function ProfileView({ profile }) {
  return (
    <div className="space-y-4">
      {profile.bio ? (
        <p className="whitespace-pre-wrap text-sm text-text-primary">{profile.bio}</p>
      ) : (
        <p className="text-sm text-text-secondary">No bio yet.</p>
      )}
      {profile.links.length > 0 && (
        <div>
          <h2 className="mb-2 text-sm font-semibold uppercase tracking-wider text-text-secondary">Links</h2>
          <ul className="space-y-1">
            {profile.links.map((l) => (
              <li key={l.id}>
                <a href={l.url} target="_blank" rel="noopener noreferrer nofollow" className="inline-flex items-center gap-1 text-sm text-brand-600 hover:underline">
                  {l.title}
                  <ExternalLink className="h-3 w-3" aria-hidden="true" />
                  <span className="sr-only">(opens in a new tab)</span>
                </a>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function ProfileEditor({ profile, onDone }) {
  const [pronouns, setPronouns] = useState(profile.pronouns || '');
  const [bio, setBio] = useState(profile.bio || '');
  const [links, setLinks] = useState(profile.links.map((l) => ({ title: l.title, url: l.url })));
  const [error, setError] = useState('');
  const { refreshUser } = useAuth();
  const save = useLearnerMutation(
    () => learnerApi.updateProfile({ pronouns, bio, links: links.filter((l) => l.title.trim() || l.url.trim()) }),
    { invalidate: [learnerKeys.all], onSuccess: () => { refreshUser?.(); onDone(); }, onError: (e) => setError(e.message) },
  );
  const setLink = (i, patch) => setLinks((ls) => ls.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  return (
    <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); setError(''); save.mutate(); }}>
      {error && <p role="alert" className="text-sm text-accent-danger">{error}</p>}
      <div>
        <label htmlFor="profile-pronouns" className="mb-1 block text-sm font-medium">Pronouns</label>
        <input id="profile-pronouns" value={pronouns} maxLength={50} onChange={(e) => setPronouns(e.target.value)}
          placeholder="e.g. she/her, they/them"
          className="w-full max-w-xs rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm" />
      </div>
      <div>
        <label htmlFor="profile-bio" className="mb-1 block text-sm font-medium">Bio</label>
        <textarea id="profile-bio" value={bio} maxLength={2000} rows={4} onChange={(e) => setBio(e.target.value)}
          className="w-full rounded-md border border-border-default bg-surface-0 px-3 py-2 text-sm" />
      </div>
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium">Links</legend>
        {links.map((l, i) => (
          <div key={i} className="flex flex-wrap items-center gap-2">
            <label className="sr-only" htmlFor={`link-title-${i}`}>Link {i + 1} title</label>
            <input id={`link-title-${i}`} value={l.title} placeholder="Title" onChange={(e) => setLink(i, { title: e.target.value })}
              className="w-40 rounded-md border border-border-default bg-surface-0 px-3 py-1.5 text-sm" />
            <label className="sr-only" htmlFor={`link-url-${i}`}>Link {i + 1} address</label>
            <input id={`link-url-${i}`} value={l.url} placeholder="https://" onChange={(e) => setLink(i, { url: e.target.value })}
              className="min-w-0 flex-1 rounded-md border border-border-default bg-surface-0 px-3 py-1.5 text-sm" />
            <button type="button" onClick={() => setLinks((ls) => ls.filter((_, j) => j !== i))}
              className="rounded p-1 hover:bg-surface-1" aria-label={`Remove link ${i + 1}`}>
              <Trash2 className="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        ))}
        {links.length < 10 && (
          <Button type="button" variant="outline" size="sm" onClick={() => setLinks((ls) => [...ls, { title: '', url: '' }])}>
            <Plus className="me-1 h-4 w-4" aria-hidden="true" /> Add link
          </Button>
        )}
      </fieldset>
      <div className="flex gap-2">
        <Button type="submit" disabled={save.isPending}>{save.isPending ? 'Saving…' : 'Save profile'}</Button>
        <Button type="button" variant="outline" onClick={onDone}>Cancel</Button>
      </div>
    </form>
  );
}

function AvatarControls({ profile }) {
  const input = useRef(null);
  const [error, setError] = useState('');
  const { refreshUser } = useAuth();
  const upload = useLearnerMutation((file) => learnerApi.uploadAvatar(file), {
    onSuccess: () => refreshUser?.(),
    onError: (e) => setError(e.message),
  });
  const remove = useLearnerMutation(() => learnerApi.deleteAvatar(), { onSuccess: () => refreshUser?.() });
  return (
    <div className="space-y-1">
      <input
        ref={input}
        type="file"
        accept="image/png,image/jpeg,image/gif,image/webp"
        className="sr-only"
        id="avatar-upload"
        tabIndex={-1}
        aria-label="Profile picture file"
        onChange={(e) => {
          const f = e.target.files?.[0];
          setError('');
          if (f && f.size > 2 * 1024 * 1024) {
            setError('Pick an image of 2 MB or less.');
          } else if (f) {
            upload.mutate(f);
          }
          e.target.value = '';
        }}
      />
      <div className="flex gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => input.current?.click()} disabled={upload.isPending}>
          <Upload className="me-1 h-4 w-4" aria-hidden="true" /> {upload.isPending ? 'Uploading…' : 'Change picture'}
        </Button>
        {profile.avatar_url && (
          <Button type="button" variant="ghost" size="sm" onClick={() => remove.mutate()}>
            <X className="me-1 h-4 w-4" aria-hidden="true" /> Remove
          </Button>
        )}
      </div>
      {error && <p role="alert" className="text-xs text-accent-danger">{error}</p>}
    </div>
  );
}

export default function ProfilePage() {
  const { userId } = useParams();
  const { user } = useAuth();
  const isSelf = !userId || String(userId) === String(user?.id);
  const query = useProfileCard(isSelf ? 'self' : userId);
  const [editing, setEditing] = useState(false);
  useDocumentTitle(query.data ? `${query.data.name} profile` : 'Profile');
  return (
    <Page query={query} title="Profile">
      {(profile) => (
        <Layout>
          <div className="mx-auto max-w-2xl space-y-6">
            <div className="flex flex-wrap items-start gap-4">
              <Avatar className="h-20 w-20 text-xl">
                {profile.avatar_url && <AvatarImage src={profile.avatar_url} alt="" />}
                <AvatarFallback>{initials(profile.name)}</AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1">
                <h1 className="text-2xl font-bold text-text-primary">
                  {profile.name}
                  {profile.pronouns && <span className="ms-2 text-base font-normal text-text-secondary">({profile.pronouns})</span>}
                </h1>
                {isSelf && !editing && (
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
                      <Pencil className="me-1 h-4 w-4" aria-hidden="true" /> Edit profile
                    </Button>
                    <Button asChild size="sm" variant="ghost">
                      <Link to="/profile/settings">Account settings</Link>
                    </Button>
                  </div>
                )}
              </div>
            </div>
            {isSelf && <AvatarControls profile={profile} />}
            {isSelf && editing ? (
              <ProfileEditor profile={profile} onDone={() => setEditing(false)} />
            ) : (
              <ProfileView profile={profile} />
            )}
          </div>
        </Layout>
      )}
    </Page>
  );
}
