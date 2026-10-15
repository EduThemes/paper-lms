/** Short random id for sections, links and list items. */
export function newId(prefix = 's') {
  let rnd;
  try {
    rnd = Array.from(crypto.getRandomValues(new Uint8Array(6)), (b) => b.toString(16).padStart(2, '0')).join('');
  } catch {
    rnd = Math.random().toString(16).slice(2, 14);
  }
  return `${prefix}-${rnd}`;
}
