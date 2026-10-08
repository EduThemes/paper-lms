import { describe, it, expect } from 'vitest';
import {
  parseCSV,
  curveGrades,
  assignmentGradeWeight,
  enteredScore,
  normalizeRubricScores,
  isAssignmentPublished,
} from '../grading';

describe('parseCSV (B118)', () => {
  it('keeps quoted commas inside one field so columns stay aligned', () => {
    const csv = [
      'Student,Student ID,"Essay 1, Draft",Quiz Prep,Total Points',
      '"Doe, Jane",42,8,9,17',
    ].join('\n');
    const rows = parseCSV(csv);
    expect(rows).toEqual([
      ['Student', 'Student ID', 'Essay 1, Draft', 'Quiz Prep', 'Total Points'],
      ['Doe, Jane', '42', '8', '9', '17'],
    ]);
  });

  it('handles escaped quotes, CRLF and embedded newlines', () => {
    const rows = parseCSV('a,"say ""hi""","line1\nline2"\r\n1,2,3\r\n\r\n');
    expect(rows).toEqual([
      ['a', 'say "hi"', 'line1\nline2'],
      ['1', '2', '3'],
    ]);
  });
});

describe('curveGrades (B123)', () => {
  it('treats the target as a percentage of points possible', () => {
    // 10-point assignment, average 6/10; a 70% target means 7 points,
    // not 70 points (which used to clamp everyone to 10/10).
    const { grades } = curveGrades({
      entries: [
        { userId: 1, entered: 5 },
        { userId: 2, entered: 7 },
      ],
      pointsPossible: 10,
      targetPercent: 70,
    });
    expect(grades).toEqual([
      { user_id: 1, posted_grade: '6' },
      { user_id: 2, posted_grade: '8' },
    ]);
  });

  it('clamps to 0..max', () => {
    const { grades } = curveGrades({
      entries: [{ userId: 1, entered: 9.5 }, { userId: 2, entered: 0.5 }],
      pointsPossible: 10,
      targetPercent: 100,
    });
    expect(grades.map((g) => g.posted_grade)).toEqual(['10', '5.5']);
  });
});

describe('enteredScore (B021)', () => {
  it('prefers the entered (pre-deduction) score', () => {
    expect(enteredScore({ score: 7, entered_score: 8 })).toBe(8);
    expect(enteredScore({ score: 7 })).toBe(7);
    expect(enteredScore(null)).toBeNull();
  });
});

describe('assignmentGradeWeight (B115)', () => {
  const published = { workflow_state: 'published', points_possible: 10 };
  it('drops excused work and unpublished assignments', () => {
    expect(assignmentGradeWeight(published, { score: 4, excused: true }).counts).toBe(false);
    expect(assignmentGradeWeight({ ...published, workflow_state: 'unpublished' }, { score: 4 }).counts).toBe(false);
    expect(assignmentGradeWeight({ ...published, grading_type: 'not_graded' }, { score: 4 }).counts).toBe(false);
  });
  it('counts 0-point assignments as extra credit', () => {
    expect(assignmentGradeWeight({ workflow_state: 'published', points_possible: 0 }, { score: 2 })).toEqual({
      counts: true,
      possible: 0,
    });
    expect(assignmentGradeWeight(published, { score: 4 })).toEqual({ counts: true, possible: 10 });
  });
  it('falls back to the published flag', () => {
    expect(isAssignmentPublished({ published: true })).toBe(true);
    expect(isAssignmentPublished({ published: false })).toBe(false);
  });
});

describe('normalizeRubricScores (B120)', () => {
  it('turns typed string points into numbers', () => {
    expect(normalizeRubricScores({ c1: { points: '4', comments: 'ok' }, c2: { points: '' }, c3: { points: 2 } })).toEqual({
      c1: { points: 4, comments: 'ok' },
      c2: { points: null },
      c3: { points: 2 },
    });
  });
});
