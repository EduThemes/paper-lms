import type { ThemeEntry } from '../define';
import storybookForest from '../themes/cartoons/storybook-forest';
import monsterPals from '../themes/cartoons/monster-pals';
import dinoDig from '../themes/cartoons/dino-dig';
import jungleSafari from '../themes/cartoons/jungle-safari';
import fairground from '../themes/cartoons/fairground';
import robotBuddies from '../themes/cartoons/robot-buddies';
import doodleNotebook from '../themes/cartoons/doodle-notebook';
import saturdayMorning from '../themes/cartoons/saturday-morning';

/** 'cartoons' collection. Theme files live in ../themes/cartoons/. Owned by one design agent; keep the order intentional. */
// Order: K-2 picture-book worlds first, then upper elementary, then the two that suit secondary.
export const ENTRIES: ThemeEntry[] = [storybookForest, monsterPals, dinoDig, jungleSafari, fairground, robotBuddies, doodleNotebook, saturdayMorning];
