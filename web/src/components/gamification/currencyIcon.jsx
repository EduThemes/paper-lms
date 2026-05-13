import React from 'react';
import {
  Zap,
  Gem,
  Target,
  ShieldCheck,
  Coins,
  Sparkles,
  Award,
  Star,
  Trophy,
  Heart,
  Crown,
  Flame,
  Diamond,
  Medal,
} from 'lucide-react';

const LUCIDE_BY_NAME = {
  zap: Zap,
  gem: Gem,
  target: Target,
  'shield-check': ShieldCheck,
  shieldcheck: ShieldCheck,
  coins: Coins,
  sparkles: Sparkles,
  award: Award,
  star: Star,
  trophy: Trophy,
  heart: Heart,
  crown: Crown,
  flame: Flame,
  diamond: Diamond,
  medal: Medal,
};

const looksLikeEmoji = (str) => {
  if (!str || typeof str !== 'string') return false;
  if (str.length > 4) return false;
  return /\p{Extended_Pictographic}/u.test(str);
};

export function CurrencyIcon({ icon, color, className = 'w-4 h-4', title }) {
  if (looksLikeEmoji(icon)) {
    return (
      <span
        role="img"
        aria-label={title || 'currency'}
        className={className}
        style={{ lineHeight: 1 }}
      >
        {icon}
      </span>
    );
  }

  const key = (icon || '').toLowerCase().trim();
  const LucideIcon = LUCIDE_BY_NAME[key] || Sparkles;
  return <LucideIcon className={className} style={color ? { color } : undefined} aria-hidden="true" />;
}
