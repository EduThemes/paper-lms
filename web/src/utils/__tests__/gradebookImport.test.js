import { mapGradebookCSV, columnAssignment } from '../gradebookImport';
import { parseCSV } from '../grading';

const assignments = [
  { id: 101, name: 'Essay 1, Draft' },
  { id: 102, name: 'Quiz Prep' },
];

describe('mapGradebookCSV', () => {
  it("maps Paper's own export", () => {
    const rows = parseCSV('Student,Student ID,"Essay 1, Draft",Quiz Prep,Total Points\n"Doe, Jane",7,8,9,17');
    const { entries, skipped } = mapGradebookCSV(rows, assignments);
    expect(skipped).toBe(0);
    expect(entries).toEqual([
      { user_id: 7, assignment_id: 101, score: 8 },
      { user_id: 7, assignment_id: 102, score: 9 },
    ]);
  });

  it("maps Canvas's export: ID column, name (id) headers, Points Possible row", () => {
    const csv = [
      'Student,ID,SIS User ID,SIS Login ID,Section,"Essay 1, Draft (101)",Renamed Quiz (102),Current Score,Final Score',
      '    Points Possible,,,,,10,10,(read only),(read only)',
      '"Doe, Jane",7,,jdoe,Section A,6,,60,30',
      '"Roe, Rick",8,,rroe,Section A,EX,4,40,20',
    ].join('\n');
    const { entries, skipped, unmatched } = mapGradebookCSV(parseCSV(csv), assignments);
    expect(skipped).toBe(0);
    expect(unmatched).toEqual([]);
    expect(entries).toEqual([
      { user_id: 7, assignment_id: 101, score: 6 },
      { user_id: 8, assignment_id: 102, score: 4 },
    ]);
  });

  it('requires a student id column', () => {
    expect(() => mapGradebookCSV(parseCSV('Student,Quiz Prep\nJane,4'), assignments)).toThrow(/Student ID/);
  });

  it('ignores total and SIS columns', () => {
    expect(columnAssignment('Final Score', assignments)).toBeNull();
    expect(columnAssignment('SIS User ID', assignments)).toBeNull();
    expect(columnAssignment('quiz prep', assignments)).toEqual(assignments[1]);
  });
});
