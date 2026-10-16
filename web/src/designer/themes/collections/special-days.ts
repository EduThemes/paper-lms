import type { ThemeEntry } from '../define';

// School events
import backToSchool from '../themes/special-days/back-to-school';
import newYear from '../themes/special-days/new-year';
import hundredDays from '../themes/special-days/hundred-days';
import testingWeek from '../themes/special-days/testing-week';
import spiritWeek from '../themes/special-days/spirit-week';
import fieldDay from '../themes/special-days/field-day';
import scienceFair from '../themes/special-days/science-fair';
import movingUp from '../themes/special-days/moving-up';
import summerBreak from '../themes/special-days/summer-break';
import teacherAppreciation from '../themes/special-days/teacher-appreciation';
import familyDay from '../themes/special-days/family-day';
// Learning and awareness days
import piDay from '../themes/special-days/pi-day';
import earthDay from '../themes/special-days/earth-day';
import readAloudDay from '../themes/special-days/read-aloud-day';
import hourOfCode from '../themes/special-days/hour-of-code';
import mentalHealthMonth from '../themes/special-days/mental-health-month';
import kindnessWeek from '../themes/special-days/kindness-week';
import stPatricksDay from '../themes/special-days/st-patricks-day';
// Heritage months
import blackHistoryMonth from '../themes/special-days/black-history-month';
import womensHistoryMonth from '../themes/special-days/womens-history-month';
import aanhpiHeritageMonth from '../themes/special-days/aanhpi-heritage-month';
import hispanicHeritageMonth from '../themes/special-days/hispanic-heritage-month';
import nativeAmericanHeritageMonth from '../themes/special-days/native-american-heritage-month';
import prideMonth from '../themes/special-days/pride-month';
// Civic days
import juneteenth from '../themes/special-days/juneteenth';
import mlkDayOfService from '../themes/special-days/mlk-day-of-service';
import remembranceDay from '../themes/special-days/remembrance-day';
import memorialDay from '../themes/special-days/memorial-day';
import citizenshipAndVoting from '../themes/special-days/citizenship-and-voting';

/**
 * 'special-days' collection. Theme files live in ../themes/special-days/. Owned by one design agent; keep the order intentional.
 * Grouped school events → learning/awareness → heritage months → civic. Within the heritage and civic groups,
 * every theme is flagged for human cultural review before its art ships (docs/COLLECTION-GUIDELINES.md §5).
 */
export const ENTRIES: ThemeEntry[] = [
  // School events
  backToSchool, newYear, hundredDays, testingWeek, spiritWeek, fieldDay, scienceFair, movingUp, summerBreak, teacherAppreciation, familyDay,
  // Learning and awareness days
  piDay, earthDay, readAloudDay, hourOfCode, mentalHealthMonth, kindnessWeek, stPatricksDay,
  // Heritage months
  blackHistoryMonth, womensHistoryMonth, aanhpiHeritageMonth, hispanicHeritageMonth, nativeAmericanHeritageMonth, prideMonth,
  // Civic days
  juneteenth, mlkDayOfService, remembranceDay, memorialDay, citizenshipAndVoting,
];
