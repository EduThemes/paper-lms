// Quiz lifecycle API (quiz-engine lane): manual grading, attempt event
// log, moderation (extensions), regrade, attempt status and the quiz
// behind an online_quiz assignment. React-query hooks live alongside the
// fetchers so the new pages share one cache-key vocabulary.

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from './api';

const base = (courseId, quizId) => `/courses/${courseId}/quizzes/${quizId}`;

export const quizEngineApi = {
  updateQuizScores: async (courseId, quizId, submissionId, payload) => {
    const { data } = await api.request(`${base(courseId, quizId)}/submissions/${submissionId}/scores`, {
      method: 'PUT', body: JSON.stringify(payload),
    });
    return data;
  },
  getQuizSubmissionEvents: async (courseId, quizId, submissionId) => {
    const { data } = await api.request(`${base(courseId, quizId)}/submissions/${submissionId}/events`);
    return data?.quiz_submission_events || [];
  },
  postQuizSubmissionEvents: async (courseId, quizId, submissionId, events) => {
    const { data } = await api.request(`${base(courseId, quizId)}/submissions/${submissionId}/events`, {
      method: 'POST', body: JSON.stringify({ quiz_submission_events: events }),
    });
    return data;
  },
  getQuizModeration: async (courseId, quizId) => {
    const { data } = await api.request(`${base(courseId, quizId)}/moderation`);
    return data || { students: [] };
  },
  setQuizExtensions: async (courseId, quizId, extensions) => {
    const { data } = await api.request(`${base(courseId, quizId)}/extensions`, {
      method: 'POST', body: JSON.stringify({ quiz_extensions: extensions }),
    });
    return data?.quiz_extensions || [];
  },
  getQuizAttemptStatus: async (courseId, quizId) => {
    const { data } = await api.request(`${base(courseId, quizId)}/attempt_status`);
    return data;
  },
  regradeQuizQuestion: async (courseId, quizId, questionId, regradeOption) => {
    const { data } = await api.request(`${base(courseId, quizId)}/questions/${questionId}/regrade`, {
      method: 'POST', body: JSON.stringify({ regrade_option: regradeOption }),
    });
    return data;
  },
  getQuizRegrades: async (courseId, quizId) => {
    const { data } = await api.request(`${base(courseId, quizId)}/regrades`);
    return Array.isArray(data) ? data : [];
  },
  getQuizForAssignment: async (courseId, assignmentId) => {
    const { data } = await api.request(`/courses/${courseId}/assignments/${assignmentId}/quiz`);
    return data;
  },
};

export const quizEngineKeys = {
  all: ['quizEngine'],
  quiz: (courseId, quizId) => ['quizEngine', String(courseId), String(quizId), 'quiz'],
  attempt: (courseId, quizId, submissionId) => ['quizEngine', String(courseId), String(quizId), 'attempt', String(submissionId)],
  events: (courseId, quizId, submissionId) => ['quizEngine', String(courseId), String(quizId), 'events', String(submissionId)],
  moderation: (courseId, quizId) => ['quizEngine', String(courseId), String(quizId), 'moderation'],
  regrades: (courseId, quizId) => ['quizEngine', String(courseId), String(quizId), 'regrades'],
  questions: (courseId, quizId) => ['quizEngine', String(courseId), String(quizId), 'questions'],
  enrollments: (courseId) => ['quizEngine', String(courseId), 'enrollments'],
  assignmentQuiz: (courseId, assignmentId) => ['quizEngine', String(courseId), 'assignmentQuiz', String(assignmentId)],
};

// --- queries ---------------------------------------------------------------

export function useQuizForEngine(courseId, quizId) {
  return useQuery({
    queryKey: quizEngineKeys.quiz(courseId, quizId),
    queryFn: () => api.getQuiz(courseId, quizId),
    enabled: Boolean(courseId && quizId),
  });
}

// One attempt for grading: the attempt, its question set (with keys, for
// staff) and its answers.
export function useQuizAttemptForGrading(courseId, quizId, submissionId) {
  return useQuery({
    queryKey: quizEngineKeys.attempt(courseId, quizId, submissionId),
    queryFn: async () => {
      const [quiz, submission, questions, answers] = await Promise.all([
        api.getQuiz(courseId, quizId),
        api.getQuizSubmission(courseId, quizId, submissionId),
        api.getQuizSubmissionQuestions(courseId, quizId, submissionId),
        api.getQuizSubmissionAnswers(courseId, quizId, submissionId),
      ]);
      return { quiz, submission, questions, answers };
    },
    enabled: Boolean(courseId && quizId && submissionId),
  });
}

export function useQuizSubmissionEvents(courseId, quizId, submissionId) {
  return useQuery({
    queryKey: quizEngineKeys.events(courseId, quizId, submissionId),
    queryFn: () => quizEngineApi.getQuizSubmissionEvents(courseId, quizId, submissionId),
    enabled: Boolean(courseId && quizId && submissionId),
  });
}

export function useQuizModeration(courseId, quizId) {
  return useQuery({
    queryKey: quizEngineKeys.moderation(courseId, quizId),
    queryFn: () => quizEngineApi.getQuizModeration(courseId, quizId),
    enabled: Boolean(courseId && quizId),
  });
}

export function useCourseRosterNames(courseId) {
  return useQuery({
    queryKey: quizEngineKeys.enrollments(courseId),
    queryFn: async () => {
      const res = await api.getAllEnrollments(courseId);
      const names = {};
      for (const e of (res?.data || res || [])) {
        const uid = e.user_id || e.user?.id;
        const name = e.user?.name || e.user?.sortable_name || e.user?.email;
        if (uid && name) names[uid] = name;
      }
      return names;
    },
    enabled: Boolean(courseId),
    staleTime: 60_000,
  });
}

export function useQuizRegradeData(courseId, quizId) {
  return useQuery({
    queryKey: quizEngineKeys.regrades(courseId, quizId),
    queryFn: async () => {
      const [quiz, questionRes, regrades] = await Promise.all([
        api.getQuiz(courseId, quizId),
        api.getQuizQuestions(courseId, quizId, 1, 500),
        quizEngineApi.getQuizRegrades(courseId, quizId),
      ]);
      const raw = questionRes?.data || questionRes || [];
      const questions = (Array.isArray(raw) ? raw : [])
        .filter((q) => q.question_bank_entry_id == null)
        .sort((a, b) => (a.position || 0) - (b.position || 0));
      return { quiz, questions, regrades };
    },
    enabled: Boolean(courseId && quizId),
  });
}

export function useQuizForAssignment(courseId, assignmentId, enabled = true) {
  return useQuery({
    queryKey: quizEngineKeys.assignmentQuiz(courseId, assignmentId),
    queryFn: () => quizEngineApi.getQuizForAssignment(courseId, assignmentId),
    enabled: Boolean(enabled && courseId && assignmentId),
    retry: false,
  });
}

// --- mutations -------------------------------------------------------------

export function useUpdateQuizScores(courseId, quizId, submissionId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload) => quizEngineApi.updateQuizScores(courseId, quizId, submissionId, payload),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: quizEngineKeys.attempt(courseId, quizId, submissionId) });
      qc.invalidateQueries({ queryKey: quizEngineKeys.events(courseId, quizId, submissionId) });
    },
  });
}

export function useSetQuizExtensions(courseId, quizId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (extensions) => quizEngineApi.setQuizExtensions(courseId, quizId, extensions),
    onSuccess: () => qc.invalidateQueries({ queryKey: quizEngineKeys.moderation(courseId, quizId) }),
  });
}

export function useRegradeQuizQuestion(courseId, quizId) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ questionId, regradeOption }) =>
      quizEngineApi.regradeQuizQuestion(courseId, quizId, questionId, regradeOption),
    onSuccess: () => qc.invalidateQueries({ queryKey: quizEngineKeys.regrades(courseId, quizId) }),
  });
}
