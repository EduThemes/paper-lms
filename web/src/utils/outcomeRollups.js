/**
 * Converts the GET /courses/:id/outcome_rollups response
 *   { rollups: [{ links: { user }, scores: [{ outcome_id, score, mastery }] }],
 *     linked: { outcomes: [...] } }
 * into the shape the outcomes page renders:
 *   { outcomes, students: [{ user_id, user_name, scores: { [outcomeId]: score },
 *                            mastery: { [outcomeId]: bool } }] }
 *
 * `enrollments` (optional) is a list of course enrollments used to resolve
 * student names, since rollups only carry user ids.
 */
/**
 * Fetches every page of a paginated list. `fetchPage(page, perPage)` returns
 * one page and `itemsOf(result)` its items; paging stops at the first empty
 * page (not a short one: the server may cap per_page below what was asked).
 * The outcome_rollups endpoint pages its students (default 10 per page) and
 * sends no Link header, so reading only the first response showed at most
 * 10 students.
 */
export async function fetchAllPages(fetchPage, itemsOf, perPage = 100, maxPages = 100) {
  const pages = [];
  for (let page = 1; page <= maxPages; page += 1) {
    const result = await fetchPage(page, perPage);
    if (page > 1 && (itemsOf(result) || []).length === 0) break;
    pages.push(result);
    if ((itemsOf(result) || []).length === 0) break;
  }
  return pages;
}

/**
 * Loads all outcome rollups of a course (every page) and merges them into a
 * single { rollups, linked } response.
 */
export async function fetchAllOutcomeRollups(getPage) {
  const pages = await fetchAllPages(getPage, (r) => r?.rollups);
  return {
    rollups: pages.flatMap((p) => p?.rollups || []),
    linked: pages[0]?.linked || { outcomes: [] },
  };
}

export function rollupResponseToView(response, enrollments = []) {
  const outcomes = response?.linked?.outcomes || [];
  const names = {};
  for (const e of enrollments || []) {
    const id = e.user_id ?? e.user?.id;
    const name = e.user?.name || e.user_name;
    if (id != null && name) names[id] = name;
  }
  const students = (response?.rollups || []).map((r) => {
    const userId = r?.links?.user;
    const scores = {};
    const mastery = {};
    for (const s of r?.scores || []) {
      if (s.score !== null && s.score !== undefined) scores[s.outcome_id] = s.score;
      if (s.mastery !== null && s.mastery !== undefined) mastery[s.outcome_id] = s.mastery;
    }
    return { user_id: userId, user_name: names[userId], scores, mastery };
  });
  return { outcomes, students };
}
