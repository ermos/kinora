import { LinearGradient } from 'expo-linear-gradient';
import { useEffect, useState, type ReactNode } from 'react';
import { Platform, Pressable, StyleSheet, Text, View } from 'react-native';
import Svg, { Path, Text as SvgText } from 'react-native-svg';
import { colors, useLayout } from '../theme';
import { Focusable } from './Focusable';
import { Icon } from './ui';
import { t } from '../i18n';

export const SEEK_STEP = 10;

const paths = {
  play: 'M6 4v16l14-8z',
  pause: 'M6 4h4v16H6zM14 4h4v16h-4z',
  volume: 'M4 9v6h4l5 4V5L8 9zM16 9a4 4 0 0 1 0 6M18.5 6.5a8 8 0 0 1 0 11',
  muted: 'M4 9v6h4l5 4V5L8 9zM17 9l5 6M22 9l-5 6',
  fullscreen: 'M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5',
  exitFullscreen: 'M9 4v5H4M20 9h-5V4M15 20v-5h5M4 15h5v5',
};

/** Netflix's bottom bar: progress with the time left, playback buttons, the title, and the screen's own actions. */
export function PlayerControls({
  position,
  duration,
  playing,
  muted,
  title,
  subtitle,
  actions,
  onToggle,
  onSeek,
  onMute,
  onPoke,
}: {
  position: number;
  duration: number;
  playing: boolean;
  muted: boolean;
  title: string;
  subtitle?: string;
  /** Right side: next episode, sources... */
  actions?: ReactNode;
  onToggle: () => void;
  onSeek: (seconds: number) => void;
  onMute: () => void;
  /** Keeps the controls shown while the remote moves through them. */
  onPoke: () => void;
}) {
  const { gutter, phone } = useLayout();
  const fullscreen = useFullscreen();
  const web = Platform.OS === 'web';

  return (
    <View style={styles.wrap} pointerEvents="box-none">
      <LinearGradient colors={['transparent', 'rgba(0,0,0,0.85)']} style={StyleSheet.absoluteFill} pointerEvents="none" />
      <View style={[styles.inner, { paddingHorizontal: gutter }]}>
        <View style={styles.progressRow}>
          <Scrubber position={position} duration={duration} onSeek={onSeek} />
          <Text style={styles.time}>{duration ? `-${clock(Math.max(0, duration - position))}` : ''}</Text>
        </View>
        <View style={styles.buttons}>
          <ControlButton label={playing ? t('watch.pause') : t('common.play')} onPress={onToggle} onFocus={onPoke} hasTVPreferredFocus>
            <Icon d={playing ? paths.pause : paths.play} size={34} fill />
          </ControlButton>
          <ControlButton label={t('watch.back10')} onPress={() => onSeek(Math.max(0, position - SEEK_STEP))} onFocus={onPoke}>
            <SeekIcon forward={false} />
          </ControlButton>
          <ControlButton label={t('watch.forward10')} onPress={() => onSeek(Math.min(duration, position + SEEK_STEP))} onFocus={onPoke}>
            <SeekIcon forward />
          </ControlButton>
          {web && (
            <ControlButton label={muted ? t('watch.unmute') : t('watch.mute')} onPress={onMute} onFocus={onPoke}>
              <Icon d={muted ? paths.muted : paths.volume} size={30} />
            </ControlButton>
          )}
          <View style={styles.titleBox}>
            {!phone && (
              <Text style={styles.title} numberOfLines={1}>
                {title}
                {subtitle ? <Text style={styles.subtitle}> {subtitle}</Text> : null}
              </Text>
            )}
          </View>
          {actions}
          {web && fullscreen.supported && (
            <ControlButton label={t('watch.fullscreen')} onPress={fullscreen.toggle} onFocus={onPoke}>
              <Icon d={fullscreen.on ? paths.exitFullscreen : paths.fullscreen} size={30} />
            </ControlButton>
          )}
        </View>
      </View>
    </View>
  );
}

function ControlButton({
  label,
  onPress,
  onFocus,
  hasTVPreferredFocus,
  children,
}: {
  label: string;
  onPress: () => void;
  onFocus: () => void;
  hasTVPreferredFocus?: boolean;
  children: ReactNode;
}) {
  return (
    <Focusable
      onPress={onPress}
      onFocus={onFocus}
      hasTVPreferredFocus={hasTVPreferredFocus}
      accessibilityLabel={label}
      style={(active) => [styles.control, active && styles.controlActive]}
    >
      {children}
    </Focusable>
  );
}

/** Circular arrow with "10" inside, the rewind/forward buttons of Netflix. */
function SeekIcon({ forward }: { forward: boolean }) {
  return (
    <Svg width={32} height={32} viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
      <Path d={forward ? 'M20 12a8 8 0 1 1-2.6-5.9L20 8.5M20 3.5v5h-5' : 'M4 12a8 8 0 1 0 2.6-5.9L4 8.5M4 3.5v5h5'} />
      <SvgText x={12} y={15.2} fontSize={7.5} fontWeight="700" fill="#fff" stroke="none" textAnchor="middle">
        {SEEK_STEP}
      </SvgText>
    </Svg>
  );
}

/** Progress bar: click or tap anywhere to jump there. */
function Scrubber({ position, duration, onSeek }: { position: number; duration: number; onSeek: (seconds: number) => void }) {
  const [width, setWidth] = useState(0);
  const ratio = duration ? Math.min(1, position / duration) : 0;
  return (
    <Pressable
      focusable={false}
      onLayout={(e) => setWidth(e.nativeEvent.layout.width)}
      onPress={(e) => width && duration && onSeek((e.nativeEvent.locationX / width) * duration)}
      style={styles.scrubber}
    >
      <View style={styles.track}>
        <View style={[styles.fill, { width: `${ratio * 100}%` }]} />
      </View>
      <View style={[styles.knob, { left: ratio * width - 7 }]} />
    </Pressable>
  );
}

/** "1:02:05" or "42:07". */
export function clock(seconds: number) {
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`;
}

/** Web fullscreen of the whole page, so the controls stay over the video. */
function useFullscreen() {
  const supported = Platform.OS === 'web' && typeof document !== 'undefined' && !!document.documentElement.requestFullscreen;
  const [on, setOn] = useState(false);
  useEffect(() => {
    if (!supported) return;
    const update = () => setOn(!!document.fullscreenElement);
    document.addEventListener('fullscreenchange', update);
    return () => document.removeEventListener('fullscreenchange', update);
  }, [supported]);
  return { supported, on, toggle: toggleFullscreen };
}

export function toggleFullscreen() {
  if (typeof document === 'undefined' || !document.documentElement.requestFullscreen) return;
  if (document.fullscreenElement) document.exitFullscreen().catch(() => {});
  else document.documentElement.requestFullscreen().catch(() => {});
}

const styles = StyleSheet.create({
  wrap: { position: 'absolute', left: 0, right: 0, bottom: 0, paddingTop: 60 },
  inner: { paddingBottom: 18, gap: 6 },
  progressRow: { flexDirection: 'row', alignItems: 'center', gap: 14 },
  scrubber: { flex: 1, height: 24, justifyContent: 'center' },
  track: { height: 4, borderRadius: 2, backgroundColor: 'rgba(255,255,255,0.3)', overflow: 'hidden' },
  fill: { height: '100%', backgroundColor: colors.red },
  knob: { position: 'absolute', top: 5, width: 14, height: 14, borderRadius: 7, backgroundColor: colors.red },
  time: { color: '#fff', fontSize: 15, fontVariant: ['tabular-nums'], minWidth: 60, textAlign: 'right' },
  buttons: { flexDirection: 'row', alignItems: 'center', gap: 10 },
  control: { padding: 8, borderRadius: 30, borderWidth: 2, borderColor: 'transparent' },
  controlActive: { borderColor: '#fff', transform: [{ scale: 1.15 }] },
  titleBox: { flex: 1, minWidth: 0, alignItems: 'center', paddingHorizontal: 12 },
  title: { color: '#fff', fontSize: 17, fontWeight: '700' },
  subtitle: { fontWeight: '400', color: '#ddd' },
});
