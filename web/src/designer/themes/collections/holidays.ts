import type { ThemeEntry } from '../define';

import midAutumnFestival from '../themes/holidays/mid-autumn-festival';
import roshHashanah from '../themes/holidays/rosh-hashanah';
import navratriDurgaPuja from '../themes/holidays/navratri-durga-puja';
import diwali from '../themes/holidays/diwali';
import diaDeLosMuertos from '../themes/holidays/dia-de-los-muertos';
import autumnHalloween from '../themes/holidays/autumn-halloween';
import harvestThanksgiving from '../themes/holidays/harvest-thanksgiving';
import hanukkah from '../themes/holidays/hanukkah';
import christmas from '../themes/holidays/christmas';
import santasWorkshop from '../themes/holidays/santas-workshop';
import gingerbreadLane from '../themes/holidays/gingerbread-lane';
import nordicKnit from '../themes/holidays/nordic-knit';
import vintageNoel from '../themes/holidays/vintage-noel';
import sweaterParty from '../themes/holidays/sweater-party';
import navidad from '../themes/holidays/navidad';
import winterSolstice from '../themes/holidays/winter-solstice';
import kwanzaa from '../themes/holidays/kwanzaa';
import winterCelebrations from '../themes/holidays/winter-celebrations';
import polarPals from '../themes/holidays/polar-pals';
import lunarNewYear from '../themes/holidays/lunar-new-year';
import ramadanEid from '../themes/holidays/ramadan-eid';
import holi from '../themes/holidays/holi';
import nowruz from '../themes/holidays/nowruz';
import passover from '../themes/holidays/passover';
import easter from '../themes/holidays/easter';
import springCelebrations from '../themes/holidays/spring-celebrations';
import vaisakhi from '../themes/holidays/vaisakhi';
import vesak from '../themes/holidays/vesak';

/**
 * 'holidays' collection. Theme files live in ../themes/holidays/. Owned by one design agent; keep the order intentional.
 * Order follows the school year (September → June). Each inclusive seasonal theme sits right after the
 * religious holiday it can stand in for (Winter after Christmas/Kwanzaa, Spring after Easter).
 */
export const ENTRIES: ThemeEntry[] = [
  midAutumnFestival, // Sep–Oct
  roshHashanah, // Sep–Oct
  navratriDurgaPuja, // Sep–Oct
  diwali, // Oct–Nov
  diaDeLosMuertos, // Oct 31–Nov 2
  autumnHalloween, // Oct
  harvestThanksgiving, // Sep–Nov
  hanukkah, // Nov–Dec
  sweaterParty, // mid-Dec school sweater day
  navidad, // Las Posadas Dec 16–24 → Día de Reyes Jan 6
  winterSolstice, // Dec 21/22
  christmas, // Dec (Jan 7 Orthodox)
  santasWorkshop, // Dec, cultural Christmas
  gingerbreadLane, // Dec, cultural Christmas
  nordicKnit, // Dec, cultural Christmas (premium)
  vintageNoel, // Dec, cultural Christmas (premium)
  kwanzaa, // Dec 26–Jan 1
  winterCelebrations, // Dec–Feb, inclusive
  polarPals, // Dec–Feb, Arctic animals, no holiday
  lunarNewYear, // Jan–Feb
  ramadanEid, // moves ~11 days earlier each year
  holi, // Feb–Mar
  nowruz, // Mar equinox
  passover, // Mar–Apr
  easter, // Mar–May
  springCelebrations, // Mar–May, inclusive
  vaisakhi, // Apr 13/14
  vesak, // Apr–Jun
];
