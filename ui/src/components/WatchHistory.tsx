import { useQuery } from '@tanstack/react-query';
import { Image } from 'expo-image';
import { type Href } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { api, img, unwrap } from '../api/client';
import type { components } from '../api/schema';
import { Focusable } from './Focusable';
import { Button, Spinner, styles as ui, tvFocus } from './ui';
import { colors } from '../theme';
import { t, useLanguage } from '../i18n';

type Year = components['schemas']['store.YearStats'];
type Entry = components['schemas']['store.HistoryEntry'];

const PAGE = 30;

const hours = (sec: number) => (sec < 3600 ? t('common.minutes', { n: Math.round(sec / 60) }) : t('history.hours', { n: Math.round(sec / 3600) }));

/** One line of three tiles: time on movies, time on shows, show of the year. */
export function WatchStats({ stats: s }: { stats: Year }) {
  return (
    <View style={styles.tiles}>
      <View style={styles.tile}>
        <Text style={styles.label}>{t('nav.movies')}</Text>
        <Text style={styles.value}>{hours(s.movieSeconds)}</Text>
        <Text style={ui.muted}>{t('history.movies', { n: s.movies })}</Text>
      </View>
      <View style={styles.tile}>
        <Text style={styles.label}>{t('nav.shows')}</Text>
        <Text style={styles.value}>{hours(s.showSeconds)}</Text>
        <Text style={ui.muted}>{t('history.episodes', { n: s.episodes })}</Text>
      </View>
      <View style={styles.tile}>
        <Text style={styles.label}>{t('history.topShow')}</Text>
        <Text style={styles.value} numberOfLines={2}>
          {s.topShow || '–'}
        </Text>
      </View>
    </View>
  );
}

/** Movies and episodes the current profile played, most recent first. */
export function HistoryList() {
  const history = useQuery({ queryKey: ['library', 'history'], queryFn: () => unwrap(api.GET('/library/history')) });
  const [shown, setShown] = useState(PAGE);
  const language = useLanguage();

  if (!history.data) return <Spinner />;
  if (!history.data.length) return <Text style={ui.muted}>{t('history.empty')}</Text>;
  return (
    <View style={{ gap: 16 }}>
      <View>
        {history.data.slice(0, shown).map((e) => (
          <Row key={`${e.type}-${e.id}-${e.season}-${e.episode}`} entry={e} language={language} />
        ))}
      </View>
      {shown < history.data.length && <Button kind="outline" small label={t('history.more')} onPress={() => setShown(shown + PAGE)} />}
    </View>
  );
}

function Row({ entry: e, language }: { entry: Entry; language: string }) {
  const date = new Date(e.watchedAt * 1000).toLocaleDateString(language, { day: 'numeric', month: 'short', year: 'numeric' });
  const sub = e.type === 'tv' ? `${t('common.episodeShort', { season: e.season, episode: e.episode })} · ${date}` : date;
  return (
    <Focusable href={`/title/${e.type}/${e.id}` as Href} style={(active) => [styles.row, active && styles.rowActive]}>
      <View style={styles.poster}>{!!e.poster && <Image source={img(e.poster, 'w300')} style={StyleSheet.absoluteFill} contentFit="cover" />}</View>
      <View style={{ flex: 1, gap: 4 }}>
        <Text style={ui.text} numberOfLines={1}>
          {e.title}
        </Text>
        <Text style={ui.muted}>{sub}</Text>
      </View>
      <View style={styles.bar}>
        <View style={{ width: `${Math.min(100, (e.position / e.duration) * 100)}%`, height: '100%', backgroundColor: colors.red }} />
      </View>
    </Focusable>
  );
}

const styles = StyleSheet.create({
  tiles: { flexDirection: 'row', gap: 12 },
  tile: { flex: 1, minWidth: 0, padding: 16, backgroundColor: colors.bg3, borderRadius: 8, gap: 4 },
  label: { color: colors.muted, fontSize: 13, fontWeight: '600', textTransform: 'uppercase', letterSpacing: 0.5 },
  value: { color: colors.text, fontSize: 24, fontWeight: '800' },
  row: { flexDirection: 'row', alignItems: 'center', gap: 14, paddingVertical: 8, paddingHorizontal: 8, borderRadius: 6 },
  rowActive: { backgroundColor: colors.bg3, ...tvFocus },
  poster: { width: 40, height: 60, borderRadius: 3, overflow: 'hidden', backgroundColor: colors.bg3 },
  bar: { width: 80, height: 4, borderRadius: 2, overflow: 'hidden', backgroundColor: '#404040' },
});
