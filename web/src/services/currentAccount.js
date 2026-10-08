// The signed-in user's tenant (account) id.
//
// Account-scoped admin routes (/api/v1/accounts/:account_id/...) must name
// the caller's own account: the backend answers 404 for any other account
// unless the caller is a platform super_admin. Admin pages therefore read
// the id from here instead of hard-coding account 1. AuthContext keeps it in
// step with the loaded user; until then it is the root account (1).
//
// Kept in its own module (not api.js) so page tests that mock the API
// client don't also have to mock this.

let currentAccountIdValue = 1;

export function setCurrentAccountId(id) {
  const n = Number(id);
  currentAccountIdValue = Number.isInteger(n) && n > 0 ? n : 1;
}

export function getCurrentAccountId() {
  return currentAccountIdValue;
}

// For module-level constants: stringifies to the CURRENT account id each
// time it is interpolated into a URL, so `const ACCOUNT_ID = currentAccount`
// keeps working after the user signs in.
export const currentAccount = {
  toString() {
    return String(currentAccountIdValue);
  },
  valueOf() {
    return currentAccountIdValue;
  },
};
