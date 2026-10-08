// Quiz-items lane API calls: item analysis + CSV reports, the formula
// preview, the can't-go-back question lock, and quiz delivery settings.
// Shares the request helper (CSRF, ApiError) with api.js.

import { api } from './api';

const API_URL = import.meta.env.VITE_API_URL || '/api/v1';

export const quizItemsApi = {
  // Item analysis report (difficulty, discrimination, point-biserial,
  // distributions for every item type).
  getItemAnalysis: async (courseId, quizId) => {
    const { data } = await api.request(`/courses/${courseId}/quizzes/${quizId}/item_analysis`);
    return data;
  },

  // Plain GET downloads (session cookie auth), used as <a href>.
  itemAnalysisCsvUrl: (courseId, quizId) =>
    `${API_URL}/courses/${courseId}/quizzes/${quizId}/item_analysis.csv`,
  studentAnalysisCsvUrl: (courseId, quizId) =>
    `${API_URL}/courses/${courseId}/quizzes/${quizId}/student_analysis.csv`,

  // Sample variable sets + computed answers for a formula config.
  formulaPreview: async (courseId, quizId, answers, count = 5) => {
    const { data } = await api.request(`/courses/${courseId}/quizzes/${quizId}/formula_preview`, {
      method: 'POST',
      body: JSON.stringify({ answers: typeof answers === 'string' ? answers : JSON.stringify(answers), count }),
    });
    return data?.samples || [];
  },

  // Can't go back: the student moved past this question.
  markQuestionRead: async (courseId, quizId, submissionId, questionId) => {
    const { data } = await api.request(
      `/courses/${courseId}/quizzes/${quizId}/submissions/${submissionId}/questions/${questionId}/read`,
      { method: 'POST' },
    );
    return data;
  },

  // Start (or resume) an attempt, sending the access code when the quiz
  // requires one.
  startSubmission: async (courseId, quizId, accessCode) => {
    const { data } = await api.request(`/courses/${courseId}/quizzes/${quizId}/submissions`, {
      method: 'POST',
      body: JSON.stringify(accessCode ? { access_code: accessCode } : {}),
    });
    return data?.quiz_submissions?.[0] || data;
  },

  // Delivery / security settings live on the quiz itself.
  updateDeliverySettings: async (courseId, quizId, settings) => {
    const { data } = await api.request(`/courses/${courseId}/quizzes/${quizId}`, {
      method: 'PUT',
      body: JSON.stringify({ quiz: settings }),
    });
    return data;
  },
};

export const quizItemsKeys = {
  itemAnalysis: (courseId, quizId) => ['quizItems', 'itemAnalysis', String(courseId), String(quizId)],
};
