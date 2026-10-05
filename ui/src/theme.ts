import { useWindowDimensions } from 'react-native';

export const colors = {
  bg: '#141414',
  bg2: '#181818',
  bg3: '#2f2f2f',
  text: '#ffffff',
  muted: '#a3a3a3',
  red: '#e50914',
  green: '#46d369',
  error: '#ff6b6b',
};

export const AVATARS: Record<string, string> = {
  red: '#e50914',
  blue: '#2f80ed',
  green: '#27ae60',
  yellow: '#f2b705',
  purple: '#9b51e0',
  pink: '#eb5ea8',
  teal: '#14a39a',
  orange: '#f2780c',
};

const RAIL = 76;

/**
 * Layout metrics shared by every screen. Phones get a bottom tab bar, everything else (desktop, tablet, TV)
 * the Netflix TV style left rail.
 */
export function useLayout() {
  const { width, height } = useWindowDimensions();
  const phone = width < 700;
  const gutter = phone ? 16 : Math.min(60, Math.max(24, width * 0.04));
  return {
    width,
    height,
    phone,
    gutter,
    rail: phone ? 0 : RAIL,
    tabBar: phone ? 64 : 0,
    // Cards per visible row width, like Netflix: 6 on large screens, fewer on small ones.
    cardsPerRow: width > 1400 ? 6 : width > 1100 ? 5 : width > 700 ? 4 : 2.3,
  };
}
