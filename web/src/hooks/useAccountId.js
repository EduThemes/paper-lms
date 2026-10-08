import { useEffect, useRef } from 'react';
import { useAuth } from '../contexts/AuthContext';

// useAccountId returns the signed-in user's tenant (account) id.
//
// Admin pages used to hardcode `accountId = 1`, so an admin of any other
// tenant read and wrote tenant 1's roles, terms, SIS imports, SSO
// providers and so on — or got 404s once the API started enforcing the
// tenant (B070). /users/self and the login responses carry account_id;
// if the user object predates that (e.g. a login path that doesn't
// return it), refresh it once from /users/self.
//
// Returns null until the account is known; callers must not fetch
// account-scoped data while it is null.
export default function useAccountId() {
  const { user, refreshUser } = useAuth();
  const accountId = user?.account_id ?? null;
  const needsRefresh = Boolean(user) && accountId == null;
  const refreshed = useRef(false);

  useEffect(() => {
    if (needsRefresh && !refreshed.current && typeof refreshUser === 'function') {
      refreshed.current = true;
      refreshUser();
    }
  }, [needsRefresh, refreshUser]);

  return accountId;
}
