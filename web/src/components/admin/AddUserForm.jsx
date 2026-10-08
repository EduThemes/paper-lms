import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useCreateAdminUser } from '../../services/adminAccountsQueries';
import { Field, Button, ErrorNote, inputClass } from './AdminUI';

// Add-user form for the admin People page (G114). Without a password the
// user gets a random one and must choose their own at first sign-in.
export default function AddUserForm({ onClose }) {
  const navigate = useNavigate();
  const [form, setForm] = useState({ name: '', email: '', login_id: '', sis_user_id: '', password: '' });
  const create = useCreateAdminUser({
    onSuccess: (user) => {
      onClose?.();
      if (user?.id) navigate(`/admin/users/${user.id}`);
    },
  });
  const set = (k) => (e) => setForm({ ...form, [k]: e.target.value });

  const submit = (e) => {
    e.preventDefault();
    const user = { name: form.name.trim(), email: form.email.trim() };
    if (form.login_id.trim()) user.login_id = form.login_id.trim();
    if (form.sis_user_id.trim()) user.sis_user_id = form.sis_user_id.trim();
    if (form.password) user.password = form.password;
    create.mutate(user);
  };

  return (
    <form onSubmit={submit} className="rounded-lg border border-border-default bg-surface-0 p-4 mb-6" aria-label="Add a user">
      <ErrorNote error={create.error} />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <Field label="Full name" id="add-user-name">
          <input id="add-user-name" required className={inputClass} value={form.name} onChange={set('name')} />
        </Field>
        <Field label="Email" id="add-user-email">
          <input id="add-user-email" type="email" required className={inputClass} value={form.email} onChange={set('email')} />
        </Field>
        <Field label="Login" id="add-user-login" hint="Defaults to the email.">
          <input id="add-user-login" className={inputClass} value={form.login_id} onChange={set('login_id')} />
        </Field>
        <Field label="SIS user ID" id="add-user-sis">
          <input id="add-user-sis" className={inputClass} value={form.sis_user_id} onChange={set('sis_user_id')} />
        </Field>
        <Field label="Password" id="add-user-password" hint="Blank: the user sets one at first sign-in.">
          <input id="add-user-password" type="password" autoComplete="new-password" className={inputClass} value={form.password} onChange={set('password')} />
        </Field>
      </div>
      <div className="mt-4 flex gap-2">
        <Button type="submit" disabled={create.isPending}>Create user</Button>
        <Button variant="secondary" onClick={onClose}>Cancel</Button>
      </div>
    </form>
  );
}
