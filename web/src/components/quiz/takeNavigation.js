// Navigation rules for taking a quiz (Canvas "show one question at a time"
// and "lock questions after answering"). Pure helpers so the take page and
// its tests share one definition.

// navigationPolicy derives what the take page may offer for a quiz.
//   forceSingle — one question per page; the "all questions" view is off.
//   canGoBack   — Previous, the question palette and number-key jumps.
export function navigationPolicy(quiz) {
  const oneAtATime = Boolean(quiz?.one_question_at_a_time);
  const cantGoBack = oneAtATime && Boolean(quiz?.cant_go_back);
  return { forceSingle: oneAtATime, canGoBack: !cantGoBack, locksOnNext: cantGoBack };
}

// firstUnlockedIndex is where a resumed attempt continues: the first
// question the student has not moved past (locked), else the last one.
export function firstUnlockedIndex(questions) {
  const list = Array.isArray(questions) ? questions : [];
  const idx = list.findIndex(q => !q?.locked);
  if (idx >= 0) return idx;
  return Math.max(0, list.length - 1);
}

// isAccessCodeError recognises the server's "invalid access code" refusal.
export function isAccessCodeError(err) {
  return err?.status === 403 && /access code/i.test(err?.message || '');
}
