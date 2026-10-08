// Take-page side of the quiz attempt log (G057): reports when the student
// leaves / returns to the quiz page and which question they look at.
// Answer saves, start and submit are logged by the server itself.

import { useEffect, useRef } from 'react';
import { quizEngineApi } from '../../services/api_quiz_engine';

const MAX_BATCH = 50;

// createQuizEventQueue batches events and hands them to `send`. A failed
// send keeps the events for the next flush (bounded, oldest dropped) so a
// flaky connection does not lose the log.
export function createQuizEventQueue(send, { maxQueued = 200, now = () => new Date() } = {}) {
  let queue = [];
  let inFlight = false;
  return {
    push(eventType, eventData) {
      const e = { event_type: eventType, client_timestamp: now().toISOString() };
      if (eventData && Object.keys(eventData).length) e.event_data = eventData;
      queue.push(e);
      if (queue.length > maxQueued) queue = queue.slice(queue.length - maxQueued);
    },
    size() {
      return queue.length;
    },
    async flush() {
      if (inFlight || queue.length === 0) return 0;
      inFlight = true;
      const batch = queue.slice(0, MAX_BATCH);
      try {
        await send(batch);
        queue = queue.slice(batch.length);
        return batch.length;
      } catch {
        return 0;
      } finally {
        inFlight = false;
      }
    },
  };
}

/**
 * useQuizEventLog reports page focus / blur and question views for the
 * student's in-progress attempt. `active` turns it on (attempt started
 * and not yet submitted); `questionId` is the question on screen.
 */
export default function useQuizEventLog({ courseId, quizId, submissionId, active, questionId }) {
  const queueRef = useRef(null);

  useEffect(() => {
    if (!active || !submissionId) return undefined;
    const queue = createQuizEventQueue((events) =>
      quizEngineApi.postQuizSubmissionEvents(courseId, quizId, submissionId, events));
    queueRef.current = queue;

    const onVisibility = () => {
      queue.push(document.visibilityState === 'hidden' ? 'page_blurred' : 'page_focused');
      if (document.visibilityState === 'hidden') queue.flush();
    };
    const onBlur = () => queue.push('page_blurred');
    const onFocus = () => queue.push('page_focused');
    document.addEventListener('visibilitychange', onVisibility);
    window.addEventListener('blur', onBlur);
    window.addEventListener('focus', onFocus);
    const timer = setInterval(() => { queue.flush(); }, 5000);

    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      window.removeEventListener('blur', onBlur);
      window.removeEventListener('focus', onFocus);
      clearInterval(timer);
      queue.flush();
      queueRef.current = null;
    };
  }, [courseId, quizId, submissionId, active]);

  useEffect(() => {
    if (!active || !questionId || !queueRef.current) return;
    queueRef.current.push('question_viewed', { question_id: questionId });
  }, [active, questionId]);
}
