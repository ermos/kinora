import { Image } from 'expo-image';
import { type Href } from 'expo-router';
import { LinearGradient } from 'expo-linear-gradient';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { ActivityIndicator, Animated, FlatList, Platform, Pressable, StyleSheet, Text, TextInput, View, type TextInputProps, type ViewStyle } from 'react-native';
import Svg, { Circle, Path, Rect } from 'react-native-svg';
import { img, type Item, type Progress } from '../api/client';
import { AVATARS, colors, useLayout } from '../theme';
import { Focusable } from './Focusable';
import { t } from '../i18n';

export function Avatar({ color, size = 32 }: { color: string; size?: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 40 40">
      <Rect width="40" height="40" rx="4" fill={AVATARS[color] ?? AVATARS.red} />
      <Circle cx="13" cy="15" r="2.6" fill="#fff" />
      <Circle cx="27" cy="15" r="2.6" fill="#fff" />
      <Path d="M10 24c3 5.5 17 5.5 20 0" stroke="#fff" strokeWidth="2.6" fill="none" strokeLinecap="round" />
    </Svg>
  );
}

export function Icon({ d, size = 26, color = '#fff', fill }: { d: string; size?: number; color?: string; fill?: boolean }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill={fill ? color : 'none'} stroke={fill ? 'none' : color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <Path d={d} />
    </Svg>
  );
}

export const icons = {
  play: 'M6 4v16l14-8z',
  search: 'M10.5 4a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM15.5 15.5 21 21',
  home: 'M3 10.5 12 3l9 7.5M5 9v11h5v-6h4v6h5V9',
  tv: 'M3 6h18v12H3zM8 21h8M12 18v3',
  movie: 'M4 4h16v16H4zM4 9h16M4 15h16M9 4v5M15 4v5M9 15v5M15 15v5',
  plus: 'M12 5v14M5 12h14',
  check: 'M5 12.5 10 17.5 19 7',
  gear: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6ZM19.4 13a7.6 7.6 0 0 0 0-2l2-1.6-2-3.4-2.4 1a7.7 7.7 0 0 0-1.7-1L15 3h-4l-.3 2.6a7.7 7.7 0 0 0-1.7 1l-2.4-1-2 3.4 2 1.6a7.6 7.6 0 0 0 0 2l-2 1.6 2 3.4 2.4-1a7.7 7.7 0 0 0 1.7 1L11 21h4l.3-2.6a7.7 7.7 0 0 0 1.7-1l2.4 1 2-3.4Z',
  logout: 'M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10',
  back: 'M19 12H5M11 6l-6 6 6 6',
  info: 'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20ZM12 16v-5M12 8h.01',
  next: 'M5 4v16l10-8zM19 5v14',
  edit: 'M4 20h4L19 9l-4-4L4 16zM13.5 6.5l4 4',
};

export function Spinner({ full }: { full?: boolean }) {
  return (
    <View style={full ? styles.spinnerFull : styles.spinnerWrap}>
      <ActivityIndicator size="large" color={colors.red} accessibilityLabel={t('common.loading')} />
    </View>
  );
}

type ButtonKind = 'white' | 'grey' | 'red' | 'outline' | 'danger';

export function Button({
  label,
  icon,
  kind = 'white',
  small,
  onPress,
  href,
  disabled,
  hasTVPreferredFocus,
  onFocus,
  onBlur,
}: {
  label: string;
  icon?: string;
  kind?: ButtonKind;
  small?: boolean;
  onPress?: () => void;
  href?: Href;
  disabled?: boolean;
  hasTVPreferredFocus?: boolean;
  onFocus?: () => void;
  onBlur?: () => void;
}) {
  const fg = kind === 'white' ? '#000' : kind === 'outline' || kind === 'danger' ? colors.muted : '#fff';
  return (
    <Focusable
      href={href}
      onPress={onPress}
      disabled={disabled}
      hasTVPreferredFocus={hasTVPreferredFocus}
      onFocus={onFocus}
      onBlur={onBlur}
      style={(active) => [
        styles.button,
        small && styles.buttonSmall,
        buttonBg[kind],
        active && buttonActive[kind],
        disabled && { opacity: 0.6 },
      ]}
    >
      {(active) => {
        const color = active && (kind === 'outline' || kind === 'danger') ? (kind === 'danger' ? colors.red : '#fff') : fg;
        return (
          <>
            {icon && <Icon d={icon} size={small ? 18 : 22} color={color} fill={icon === icons.play} />}
            <Text style={[styles.buttonText, small && styles.buttonTextSmall, { color }, (kind === 'outline' || kind === 'danger') && styles.buttonTextOutline]}>
              {label}
            </Text>
          </>
        );
      }}
    </Focusable>
  );
}

const buttonBg: Record<ButtonKind, ViewStyle> = {
  white: { backgroundColor: '#fff' },
  grey: { backgroundColor: 'rgba(109,109,110,0.7)' },
  red: { backgroundColor: colors.red },
  outline: { borderWidth: 1, borderColor: colors.muted },
  danger: { borderWidth: 1, borderColor: colors.muted },
};
const buttonActive: Record<ButtonKind, ViewStyle> = {
  white: { backgroundColor: 'rgba(255,255,255,0.75)' },
  grey: { backgroundColor: 'rgba(109,109,110,0.4)' },
  red: { backgroundColor: '#c11119' },
  outline: { borderColor: '#fff' },
  danger: { borderColor: colors.red },
};

/** Small selectable pill: seasons, sources, audio tracks. Replaces <select>, which does not exist on TV. */
export function Chip({ label, selected, onPress }: { label: string; selected?: boolean; onPress: () => void }) {
  return (
    <Focusable onPress={onPress} style={(active) => [styles.chip, selected && styles.chipSelected, active && styles.chipActive]}>
      <Text style={[styles.chipText, selected && styles.chipTextSelected]}>{label}</Text>
    </Focusable>
  );
}

export function Field(props: TextInputProps & { label?: string }) {
  const { label, style, ...rest } = props;
  const input = <TextInput placeholderTextColor="#777" accessibilityLabel={label ?? rest.placeholder} {...rest} style={[styles.input, style]} />;
  return label ? (
    <View style={{ gap: 6 }}>
      <Text style={styles.label}>{label}</Text>
      {input}
    </View>
  ) : (
    input
  );
}

export function Hero({
  item,
  children,
  height,
  overlay,
}: {
  item: Pick<Item, 'title' | 'overview' | 'backdrop' | 'logo'>;
  children?: ReactNode;
  height?: number;
  /** Absolutely positioned extras (carousel indicators). */
  overlay?: ReactNode;
}) {
  const { height: screenH, rail, gutter, phone } = useLayout();
  // The text fades in whenever the item changes (carousel); the image crossfades through expo-image.
  const fade = useRef(new Animated.Value(1)).current;
  const [logoFailed, setLogoFailed] = useState(false);
  useEffect(() => {
    setLogoFailed(false);
    fade.setValue(0);
    Animated.timing(fade, { toValue: 1, duration: 600, useNativeDriver: true }).start();
  }, [item.title, fade]);
  const logo = !logoFailed && img(item.logo, 'w500');
  return (
    <View style={{ height: height ?? Math.max(460, screenH * 0.8), marginLeft: -rail }}>
      <Image source={img(item.backdrop, 'w1280')} style={StyleSheet.absoluteFill} contentFit="cover" contentPosition="top" transition={700} />
      <LinearGradient colors={['rgba(0,0,0,0.8)', 'transparent']} start={{ x: 0, y: 0.5 }} end={{ x: 0.75, y: 0.5 }} style={StyleSheet.absoluteFill} />
      <LinearGradient colors={['transparent', colors.bg]} locations={[0.6, 1]} style={StyleSheet.absoluteFill} />
      <Animated.View style={[styles.heroContent, { opacity: fade, paddingLeft: rail + gutter, paddingRight: gutter, maxWidth: phone ? undefined : 760 + rail }]}>
        {logo ? (
          // Title artwork, like Netflix banners; the text title comes back if it fails to load.
          <Image
            source={logo}
            style={phone ? styles.heroLogoPhone : styles.heroLogo}
            contentFit="contain"
            contentPosition="left"
            accessibilityLabel={item.title}
            onError={() => setLogoFailed(true)}
          />
        ) : (
          <Text style={[styles.heroTitle, phone && { fontSize: 34 }]}>{item.title}</Text>
        )}
        {!!item.overview && (
          <Text style={styles.heroOverview} numberOfLines={3}>
            {item.overview}
          </Text>
        )}
        {children}
      </Animated.View>
      {/* after the content: focus reaches the hero buttons before the indicators */}
      {overlay}
    </View>
  );
}

export function Row<T>({ title, data, render, keyOf }: { title: string; data: T[]; render: (it: T, width: number) => ReactNode; keyOf: (it: T) => string }) {
  const { width, rail, gutter, cardsPerRow } = useLayout();
  const cardWidth = (width - rail - 2 * gutter) / cardsPerRow - 6;
  const list = useRef<FlatList<T>>(null);
  const offset = useRef(0);
  const [hovered, setHovered] = useState(false);
  const [edges, setEdges] = useState({ start: true, end: false });

  // Mouse users get Netflix web arrows (a wheel does not scroll sideways). TVs scroll by moving the focus.
  const arrows = Platform.OS === 'web' && hovered;
  const page = (dir: number) => list.current?.scrollToOffset({ offset: Math.max(0, offset.current + dir * (width - rail - 2 * gutter) * 0.9), animated: true });

  return (
    <View style={styles.row} onPointerEnter={() => setHovered(true)} onPointerLeave={() => setHovered(false)}>
      <Text style={[styles.rowTitle, { paddingHorizontal: gutter }]}>{title}</Text>
      <View>
        <FlatList
          ref={list}
          horizontal
          data={data}
          keyExtractor={keyOf}
          renderItem={({ item }) => <View style={{ width: cardWidth }}>{render(item, cardWidth)}</View>}
          ItemSeparatorComponent={() => <View style={{ width: 6 }} />}
          // vertical padding leaves room for the focused card, which grows (overflow must stay scrollable on web)
          contentContainerStyle={{ paddingHorizontal: gutter, paddingVertical: 12 }}
          showsHorizontalScrollIndicator={false}
          scrollEventThrottle={100}
          onScroll={(e) => {
            const { contentOffset, contentSize, layoutMeasurement } = e.nativeEvent;
            offset.current = contentOffset.x;
            const next = { start: contentOffset.x <= 4, end: contentOffset.x + layoutMeasurement.width >= contentSize.width - 4 };
            if (next.start !== edges.start || next.end !== edges.end) setEdges(next);
          }}
        />
        {arrows && !edges.start && <RowArrow side="left" width={gutter} onPress={() => page(-1)} />}
        {arrows && !edges.end && <RowArrow side="right" width={gutter} onPress={() => page(1)} />}
      </View>
    </View>
  );
}

function RowArrow({ side, width, onPress }: { side: 'left' | 'right'; width: number; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      focusable={false}
      accessibilityLabel={side === 'left' ? t('common.previous') : t('common.next')}
      style={[styles.rowArrow, { [side]: 0, width: Math.max(width, 40) }]}
    >
      <Icon d={side === 'left' ? 'M15 5l-7 7 7 7' : 'M9 5l7 7-7 7'} size={36} />
    </Pressable>
  );
}

/** Landscape card, Netflix style: grows and gets a white frame when focused. */
export function Card({ href, title, image, progress, subtitle }: { href: Href; title: string; image?: string; progress?: number; subtitle?: string }) {
  return (
    <Focusable href={href} style={(active) => [styles.card, active && styles.cardActive]}>
      {image ? <Image source={image} style={StyleSheet.absoluteFill} contentFit="cover" transition={200} recyclingKey={image} /> : null}
      <LinearGradient colors={['transparent', 'rgba(0,0,0,0.85)']} locations={[0.45, 1]} style={StyleSheet.absoluteFill} />
      <Text style={styles.cardTitle} numberOfLines={2}>
        {title}
        {subtitle ? <Text style={styles.cardSubtitle}> {subtitle}</Text> : null}
      </Text>
      {progress !== undefined && <ProgressBar value={progress} style={styles.cardProgress} />}
    </Focusable>
  );
}

export function itemCard(it: Pick<Item, 'id' | 'type' | 'title' | 'backdrop' | 'poster'>) {
  return <Card href={`/title/${it.type}/${it.id}`} title={it.title} image={img(it.backdrop || it.poster, 'w780')} />;
}

export function progressCard(p: Progress) {
  const href = p.type === 'tv' ? `/watch/tv/${p.id}?s=${p.season}&e=${p.episode}` : `/watch/movie/${p.id}`;
  return (
    <Card
      href={href as Href}
      title={p.title}
      subtitle={p.type === 'tv' ? t('common.episodeShort', { season: p.season, episode: p.episode }) : undefined}
      image={img(p.backdrop || p.poster, 'w780')}
      progress={p.position / p.duration}
    />
  );
}

export function ProgressBar({ value, style }: { value: number; style?: ViewStyle }) {
  return (
    <View style={[styles.progress, style]}>
      <View style={[styles.progressFill, { width: `${Math.min(100, Math.max(0, value * 100))}%` }]} />
    </View>
  );
}

export function PosterGrid({ items }: { items: Pick<Item, 'id' | 'type' | 'title' | 'poster'>[] }) {
  const { width, rail, gutter } = useLayout();
  const inner = width - rail - 2 * gutter;
  const columns = Math.max(2, Math.floor(inner / 160));
  const w = (inner - (columns - 1) * 12) / columns;
  return (
    <View style={styles.grid}>
      {items.map((it) => (
        <Focusable key={`${it.type}-${it.id}`} href={`/title/${it.type}/${it.id}`} style={(active) => [styles.poster, { width: w }, active && styles.cardActive]}>
          {it.poster ? (
            <Image source={img(it.poster, 'w342')} style={StyleSheet.absoluteFill} contentFit="cover" transition={200} accessibilityLabel={it.title} />
          ) : (
            <Text style={styles.posterFallback}>{it.title}</Text>
          )}
        </Focusable>
      ))}
    </View>
  );
}

export function formatRuntime(min: number) {
  if (!min) return '';
  const h = Math.floor(min / 60);
  return h ? t('common.hours', { h, m: String(min % 60).padStart(2, '0') }) : t('common.minutes', { n: min });
}

export const styles = StyleSheet.create({
  spinnerFull: { flex: 1, minHeight: 300, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.bg },
  spinnerWrap: { padding: 40, alignItems: 'center' },
  button: { flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 8, paddingVertical: 10, paddingHorizontal: 24, borderRadius: 4 },
  buttonSmall: { paddingVertical: 6, paddingHorizontal: 14 },
  buttonText: { fontSize: 17, fontWeight: '700' },
  buttonTextSmall: { fontSize: 14 },
  buttonTextOutline: { fontWeight: '400', textTransform: 'uppercase', letterSpacing: 1.5 },
  chip: { paddingVertical: 8, paddingHorizontal: 16, borderRadius: 4, backgroundColor: '#242424', borderWidth: 2, borderColor: 'transparent' },
  chipSelected: { backgroundColor: '#fff' },
  chipActive: { borderColor: '#fff', transform: [{ scale: 1.05 }] },
  chipText: { color: '#ddd', fontWeight: '600' },
  chipTextSelected: { color: '#000' },
  input: { backgroundColor: 'rgba(22,22,22,0.7)', borderWidth: 1, borderColor: 'rgba(128,128,128,0.7)', borderRadius: 4, paddingVertical: 12, paddingHorizontal: 14, color: '#fff', fontSize: 16 },
  label: { color: colors.muted, fontSize: 13 },
  heroContent: { flex: 1, justifyContent: 'center', gap: 16 },
  heroTitle: { color: '#fff', fontSize: 60, fontWeight: '900', lineHeight: 64, textShadowColor: 'rgba(0,0,0,0.5)', textShadowRadius: 6, textShadowOffset: { width: 2, height: 2 } },
  heroLogo: { width: 480, maxWidth: '100%', height: 170 },
  heroLogoPhone: { width: 260, maxWidth: '100%', height: 100 },
  heroOverview: { color: '#fff', fontSize: 18, lineHeight: 25, textShadowColor: 'rgba(0,0,0,0.6)', textShadowRadius: 3, textShadowOffset: { width: 1, height: 1 } },
  heroActions: { flexDirection: 'row', gap: 12, flexWrap: 'wrap', marginTop: 4 },
  row: { marginBottom: 12 },
  rowTitle: { color: '#e5e5e5', fontSize: 20, fontWeight: '700' },
  rowArrow: { position: 'absolute', top: 12, bottom: 12, zIndex: 5, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(20,20,20,0.5)' },
  card: { aspectRatio: 16 / 9, borderRadius: 4, overflow: 'hidden', backgroundColor: colors.bg2, justifyContent: 'flex-end', borderWidth: 3, borderColor: 'transparent' },
  cardActive: { borderColor: '#fff', transform: [{ scale: 1.06 }], zIndex: 2 },
  cardTitle: { color: '#fff', fontWeight: '700', fontSize: 15, padding: 10, paddingBottom: 12, textShadowColor: '#000', textShadowRadius: 2 },
  cardSubtitle: { fontWeight: '400', fontSize: 13 },
  cardProgress: { position: 'absolute', left: 8, right: 8, bottom: 5 },
  progress: { height: 3, backgroundColor: 'rgba(255,255,255,0.3)' },
  progressFill: { height: '100%', backgroundColor: colors.red },
  grid: { flexDirection: 'row', flexWrap: 'wrap', gap: 12 },
  poster: { aspectRatio: 2 / 3, borderRadius: 4, overflow: 'hidden', backgroundColor: colors.bg3, justifyContent: 'center', borderWidth: 3, borderColor: 'transparent' },
  posterFallback: { color: '#fff', textAlign: 'center', padding: 8 },
  page: { flex: 1, backgroundColor: colors.bg },
  h1: { color: '#fff', fontSize: 32, fontWeight: '700', marginBottom: 20 },
  h2: { color: '#fff', fontSize: 22, fontWeight: '700' },
  text: { color: '#fff', fontSize: 16, lineHeight: 23 },
  muted: { color: colors.muted },
  error: { color: colors.error },
  success: { color: colors.green },
});
