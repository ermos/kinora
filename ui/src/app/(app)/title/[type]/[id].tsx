import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Image } from 'expo-image';
import { useLocalSearchParams, type Href } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { api, img, unwrap, type Details, type MediaType, type Progress } from '../../../../api/client';
import { useLibrary } from '../../../../components/Browse';
import { Focusable } from '../../../../components/Focusable';
import { Button, Chip, formatRuntime, Hero, Icon, icons, PosterGrid, ProgressBar, Spinner, styles as ui } from '../../../../components/ui';
import { colors, useLayout } from '../../../../theme';
import { t } from '../../../../i18n';

export default function Title() {
  const { type, id: rawId } = useLocalSearchParams<{ type: MediaType; id: string }>();
  const id = Number(rawId);
  const { gutter, height, phone } = useLayout();
  const scroll = useRef<ScrollView>(null);
  const details = useQuery({
    queryKey: ['title', type, id],
    queryFn: () => unwrap(api.GET('/titles/{type}/{id}', { params: { path: { type, id } } })),
  });
  const progress = useQuery({
    queryKey: ['library', 'progress', type, id],
    queryFn: () => unwrap(api.GET('/library/progress/{type}/{id}', { params: { path: { type, id } } })),
  });

  useEffect(() => {
    scroll.current?.scrollTo({ y: 0, animated: false });
  }, [type, id]);

  if (details.isPending) return <Spinner full />;
  if (details.isError) return <Text style={[ui.error, { padding: gutter, paddingTop: 80 }]}>{details.error.message}</Text>;
  const d = details.data;
  const last = progress.data?.[0];

  return (
    <ScrollView ref={scroll} style={ui.page}>
      <Hero item={{ ...d, overview: '' }} height={Math.max(420, height * 0.7)}>
        <View style={styles.meta}>
          {d.rating > 0 && <Text style={styles.match}>{t('title.positive', { n: Math.round(d.rating * 10) })}</Text>}
          {d.year > 0 && <Text style={styles.metaText}>{d.year}</Text>}
          <Text style={styles.metaText}>
            {type === 'tv' ? t(d.seasons.length > 1 ? 'title.seasons' : 'title.season', { n: d.seasons.length }) : formatRuntime(d.runtime)}
          </Text>
        </View>
        <View style={ui.heroActions}>
          <PlayButton type={type} details={d} last={last} />
          <ListButton type={type} details={d} />
        </View>
      </Hero>

      <View style={{ paddingHorizontal: gutter, paddingBottom: 60, gap: 40 }}>
        <View style={[styles.info, phone && { flexDirection: 'column' }]}>
          <Text style={[ui.text, { flex: 2, fontSize: 17 }]}>{d.overview || t('title.noOverview')}</Text>
          <View style={{ flex: 1, gap: 10 }}>
            {d.cast.length > 0 && (
              <Text style={styles.fact}>
                <Text style={ui.muted}>{t('title.cast')}</Text>
                {d.cast.join(', ')}
              </Text>
            )}
            {d.genres.length > 0 && (
              <Text style={styles.fact}>
                <Text style={ui.muted}>{t('title.genres')}</Text>
                {d.genres.map((g) => g.name).join(', ')}
              </Text>
            )}
          </View>
        </View>

        {type === 'tv' && d.seasons.length > 0 && <Episodes details={d} progress={progress.data ?? []} />}

        {d.similar.length > 0 && (
          <View style={{ gap: 12 }}>
            <Text style={ui.h2}>{t('title.similar')}</Text>
            <PosterGrid items={d.similar} />
          </View>
        )}
      </View>
    </ScrollView>
  );
}

function PlayButton({ type, details, last }: { type: MediaType; details: Details; last?: Progress }) {
  const resumable = last && last.position < last.duration * 0.95;
  let href = `/watch/${type}/${details.id}`;
  let label = resumable ? t('common.resume') : t('common.play');
  if (type === 'tv') {
    const s = last?.season ?? details.seasons[0]?.number ?? 1;
    const e = last?.episode ?? 1;
    href += `?s=${s}&e=${e}`;
    if (last) label = `${resumable ? t('common.resume') : t('common.rewatch')} ${t('common.episodeShort', { season: s, episode: e })}`;
  }
  return <Button icon={icons.play} label={label} href={href as Href} hasTVPreferredFocus />;
}

function ListButton({ type, details }: { type: MediaType; details: Details }) {
  const queryClient = useQueryClient();
  const { list } = useLibrary();
  const inList = list.some((i) => i.type === type && i.id === details.id);
  const toggle = async () => {
    if (inList) await api.DELETE('/library/list/{type}/{id}', { params: { path: { type, id: details.id } } });
    else await api.PUT('/library/list', { body: { type, id: details.id, title: details.title, poster: details.poster } });
    queryClient.invalidateQueries({ queryKey: ['library', 'list'] });
  };
  return <Button kind="grey" icon={inList ? icons.check : icons.plus} label={t('common.myList')} onPress={toggle} />;
}

function Episodes({ details, progress }: { details: Details; progress: Progress[] }) {
  const { phone } = useLayout();
  const [season, setSeason] = useState(progress[0]?.season || details.seasons[0].number);
  const episodes = useQuery({
    queryKey: ['season', details.id, season],
    queryFn: () => unwrap(api.GET('/titles/tv/{id}/seasons/{season}', { params: { path: { id: details.id, season } } })),
  });
  const watched = (e: number) => progress.find((p) => p.season === season && p.episode === e);

  return (
    <View style={{ gap: 12 }}>
      <Text style={ui.h2}>{t('title.episodes')}</Text>
      {details.seasons.length > 1 && (
        <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={{ gap: 8, paddingVertical: 4 }}>
          {details.seasons.map((s) => (
            <Chip key={s.number} label={s.name} selected={s.number === season} onPress={() => setSeason(s.number)} />
          ))}
        </ScrollView>
      )}
      {episodes.isPending ? (
        <Spinner />
      ) : (
        episodes.data?.map((ep) => {
          const w = watched(ep.number);
          return (
            <Focusable
              key={ep.number}
              href={`/watch/tv/${details.id}?s=${season}&e=${ep.number}` as Href}
              style={(active) => [styles.episode, active && styles.episodeActive]}
            >
              {(active) => (
                <>
                  {!phone && <Text style={styles.epNum}>{ep.number}</Text>}
                  <View style={[styles.still, { width: phone ? 120 : 160 }]}>
                    {ep.still ? <Image source={img(ep.still, 'w300')} style={StyleSheet.absoluteFill} contentFit="cover" /> : null}
                    {active && (
                      <View style={styles.stillPlay}>
                        <Icon d={icons.play} fill size={30} />
                      </View>
                    )}
                    {w && <ProgressBar value={w.position / w.duration} style={styles.stillProgress} />}
                  </View>
                  <View style={{ flex: 1, gap: 6 }}>
                    <View style={styles.epHead}>
                      <Text style={[ui.text, { fontWeight: '700', flex: 1 }]}>
                        {phone ? `${ep.number}. ` : ''}
                        {ep.name}
                      </Text>
                      <Text style={ui.muted}>{formatRuntime(ep.runtime)}</Text>
                    </View>
                    <Text style={styles.epOverview} numberOfLines={3}>
                      {ep.overview}
                    </Text>
                  </View>
                </>
              )}
            </Focusable>
          );
        })
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  meta: { flexDirection: 'row', gap: 12, alignItems: 'center', flexWrap: 'wrap' },
  match: { color: colors.green, fontWeight: '700', fontSize: 16 },
  metaText: { color: '#ddd', fontSize: 16 },
  info: { flexDirection: 'row', gap: 32 },
  fact: { color: '#fff', fontSize: 14 },
  episode: { flexDirection: 'row', alignItems: 'center', gap: 16, padding: 16, borderRadius: 4, borderBottomWidth: 1, borderBottomColor: '#404040' },
  episodeActive: { backgroundColor: '#333' },
  epNum: { color: '#d2d2d2', fontSize: 24, width: 32, textAlign: 'center' },
  still: { aspectRatio: 16 / 9, borderRadius: 4, overflow: 'hidden', backgroundColor: colors.bg3 },
  stillPlay: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(0,0,0,0.3)' },
  stillProgress: { position: 'absolute', left: 0, right: 0, bottom: 0 },
  epHead: { flexDirection: 'row', gap: 12 },
  epOverview: { color: '#d2d2d2', fontSize: 14, lineHeight: 20 },
});
