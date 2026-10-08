// Course publish state (G077). Students can't open a course until it is
// published ("available"); the server's state machine only moves one step
// at a time, so publishing a freshly created course walks the chain.

const PUBLISH_CHAIN = ['claimed', 'created', 'unpublished', 'available'];

export function isCoursePublished(state) {
  return state === 'available' || state === 'completed';
}

// publishSteps lists the workflow_state updates that publish a course in
// `state` (empty when already published or not publishable from here).
export function publishSteps(state) {
  const i = PUBLISH_CHAIN.indexOf(state || 'unpublished');
  if (i < 0 || i === PUBLISH_CHAIN.length - 1) return [];
  return PUBLISH_CHAIN.slice(i + 1);
}

// unpublishSteps: only a published (available) course can go back.
export function unpublishSteps(state) {
  return state === 'available' ? ['unpublished'] : [];
}

export function courseStateLabel(state) {
  switch (state) {
    case 'available': return 'Published';
    case 'completed': return 'Concluded';
    case 'deleted': return 'Deleted';
    default: return 'Unpublished';
  }
}
