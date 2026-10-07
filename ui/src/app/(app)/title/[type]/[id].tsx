import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Image } from 'expo-image';
import { useLocalSearchParams, type Href } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { BackHandler, ScrollView, StyleSheet, Text, View } from 'react-native';
import { api, img, unwrap, type Details, type MediaType, type Progress } from '../../../../api/client';
import { useLibrary } from '../../../../components/Browse';
import { Focusable } from '../../../../components/Focusable';
import { Button, Chip, formatRuntime, Hero, Icon, icons, PosterGrid, ProgressBar, Spinner, styles as ui, tvFocus } from '../../../../components/ui';
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
          {!!d.badge && d.badge !== 'top10' && <Text style={styles.badgeText}>{t(`badges.${d.badge as 'new' | 'newEpisode' | 'newSeason'}`)}</Text>}
          <Text style={styles.metaText}>
            {type === 'tv' ? t(d.seasons.length > 1 ? 'title.seasons' : 'title.season', { n: d.seasons.length }) : formatRuntime(d.runtime)}
          </Text>
        </View>
        <View style={ui.heroActions}>
          <PlayButton type={type} details={d} last={last} />
          <ListButton type={type} details={d} />
          <RateButtons type={type} details={d} />
          <MoreButton type={type} details={d} />
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
    let s = last?.season ?? details.seasons[0]?.number ?? 1;
    let e = last?.episode ?? 1;
    let verb = resumable ? t('common.resume') : t('common.rewatch');
    // Last episode finished: the next one, if the show has it.
    if (last && !resumable) {
      const next =
        e < (details.seasons.find((x) => x.number === s)?.episodes ?? 0)
          ? { s, e: e + 1 }
          : details.seasons.some((x) => x.number === s + 1)
            ? { s: s + 1, e: 1 }
            : null;
      if (next) [s, e, verb] = [next.s, next.e, t('common.play')];
    }
    href += `?s=${s}&e=${e}`;
    if (last) label = `${verb} ${t('common.episodeShort', { season: s, episode: e })}`;
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

function RoundButton({ icon, fill, label, onPress }: { icon: string; fill?: boolean; label: string; onPress: () => void }) {
  return (
    <Focusable accessibilityLabel={label} onPress={onPress} style={(active) => [styles.round, active && styles.roundActive, active && tvFocus]}>
      <Icon d={icon} fill={fill} size={22} />
    </Focusable>
  );
}

/** Thumbs up and down, pressing the current one again removes it. */
function RateButtons({ type, details }: { type: MediaType; details: Details }) {
  const queryClient = useQueryClient();
  const ratings = useQuery({ queryKey: ['library', 'ratings'], queryFn: () => unwrap(api.GET('/library/ratings')) });
  const current = ratings.data?.find((r) => r.type === type && r.id === details.id)?.rating ?? 0;
  const rate = async (rating: -1 | 1) => {
    await api.PUT('/library/ratings', { body: { type, id: details.id, title: details.title, poster: details.poster, rating: current === rating ? 0 : rating } });
    queryClient.invalidateQueries({ queryKey: ['library', 'ratings'] });
    queryClient.invalidateQueries({ queryKey: ['library', 'family'] }); // a thumbs down hides it there
  };
  return (
    <>
      <RoundButton icon={icons.thumbUp} fill={current === 1} label={t('title.like')} onPress={() => rate(1)} />
      <RoundButton icon={icons.thumbDown} fill={current === -1} label={t('title.dislike')} onPress={() => rate(-1)} />
    </>
  );
}

/** "⋮" next to "My list": family list and mark as watched. */
function MoreButton({ type, details }: { type: MediaType; details: Details }) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const family = useQuery({ queryKey: ['library', 'family'], queryFn: () => unwrap(api.GET('/library/family')) });
  const inFamily = family.data?.some((i) => i.type === type && i.id === details.id);
  const path = { type, id: details.id };

  // Back on the remote closes the menu instead of leaving the page.
  useEffect(() => {
    if (!open) return;
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      setOpen(false);
      return true;
    });
    return () => sub.remove();
  }, [open]);

  const toggleFamily = async () => {
    if (inFamily) await api.DELETE('/library/family/{type}/{id}', { params: { path } });
    else await api.PUT('/library/family', { body: { type, id: details.id, title: details.title, poster: details.poster } });
    queryClient.invalidateQueries({ queryKey: ['library', 'family'] });
    setOpen(false);
  };
  const markWatched = async () => {
    setBusy(true);
    await api.POST('/library/watched/{type}/{id}', { params: { path } });
    setBusy(false);
    setOpen(false);
    queryClient.invalidateQueries({ queryKey: ['library'] }); // progress, history, stats
    queryClient.invalidateQueries({ queryKey: ['foryou'] });
  };

  return (
    <View style={{ zIndex: 10 }}>
      <RoundButton icon={icons.more} fill label={t('title.more')} onPress={() => setOpen(!open)} />
      {open && (
        <View style={styles.menu}>
          <Focusable hasTVPreferredFocus onPress={toggleFamily} style={(active) => [styles.menuItem, active && styles.menuItemActive]}>
            <Icon d={inFamily ? icons.check : icons.plus} size={18} />
            <Text style={ui.text}>{t(inFamily ? 'title.removeFamily' : 'title.addFamily')}</Text>
          </Focusable>
          <Focusable disabled={busy} onPress={markWatched} style={(active) => [styles.menuItem, active && styles.menuItemActive]}>
            <Icon d={icons.check} size={18} />
            <Text style={ui.text}>{busy ? '…' : t('title.markWatched')}</Text>
          </Focusable>
        </View>
      )}
    </View>
  );
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
  badgeText: { color: '#fff', fontSize: 13, fontWeight: '800', textTransform: 'uppercase', backgroundColor: colors.red, paddingHorizontal: 6, paddingVertical: 2, borderRadius: 2 },
  info: { flexDirection: 'row', gap: 32 },
  round: { width: 44, height: 44, borderRadius: 22, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(109,109,110,0.7)', alignSelf: 'center' },
  roundActive: { backgroundColor: 'rgba(150,150,150,0.9)' },
  menu: { position: 'absolute', top: 52, left: 0, minWidth: 260, paddingVertical: 6, backgroundColor: 'rgba(20,20,20,0.97)', borderWidth: 1, borderColor: '#404040', borderRadius: 6 },
  menuItem: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 10, paddingHorizontal: 16 },
  menuItemActive: { backgroundColor: colors.bg3 },
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
