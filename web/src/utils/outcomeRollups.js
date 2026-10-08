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
