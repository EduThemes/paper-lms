/**
 * Collects every page of a paginated list endpoint.
 *
 * `fetchPage(page)` must resolve to the `{ data, pagination }` shape the
 * api.js `request` helper returns, where `pagination` is the parsed Link
 * header. Pages are requested 1, 2, 3, ... while the server advertises a
 * rel="next" link and the page came back non-empty. `maxPages` bounds the
 * loop so a misbehaving server can't spin it forever.
 *
 * @param {(page: number) => Promise<{data: any[], pagination?: object}>} fetchPage
 * @param {{ maxPages?: number }} [options]
 * @returns {Promise<any[]>}
 */
export async function fetchAllPages(fetchPage, { maxPages = 50 } = {}) {
  const items = [];
  for (let page = 1; page <= maxPages; page++) {
    const result = await fetchPage(page);
    const data = Array.isArray(result?.data) ? result.data : [];
    items.push(...data);
    if (data.length === 0 || !result?.pagination?.next) break;
  }
  return items;
}

export default fetchAllPages;
