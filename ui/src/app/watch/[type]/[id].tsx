import { useQuery, useQueryClient } from '@tanstack/react-query';
import { router, useLocalSearchParams, type Href } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';
import { absolute, api, unwrap, type Link, type MediaType } from '../../../api/client';
import { Focusable } from '../../../components/Focusable';
import { Player } from '../../../components/Player';
import { Button, Chip, Icon, icons, styles as ui } from '../../../components/ui';
import { Gate } from '../../../lib/auth';
import { colors, useLayout } from '../../../theme';
import { t } from '../../../i18n';

type Status = 'searching' | 'loading' | 'playing' | 'none' | 'failed';

type Params = {
  type: MediaType;
  id: string;
  s?: string;
  e?: string;
  /** Link of the previous episode (source, hoster, lang), so the next one starts on the same site. */
  src?: string;
  host?: string;
  lang?: string;
};

export default function WatchScreen() {
  return (
    <Gate>
      <Watch />
    </Gate>
  );
}

function Watch() {
  const params = useLocalSearchParams<Params>();
  const type = params.type;
  const id = Number(params.id);
  const season = type === 'tv' ? Number(params.s ?? 1) : 0;
  const episode = type === 'tv' ? Number(params.e ?? 1) : 0;
  const queryClient = useQueryClient();
  const { gutter } = useLayout();

  const details = useQuery({
    queryKey: ['title', type, id],
    queryFn: () => unwrap(api.GET('/titles/{type}/{id}', { params: { path: { type, id } } })),
  });
  const links = useQuery({
    queryKey: ['links', type, id, season, episode],
    queryFn: () =>
      unwrap(api.GET('/titles/{type}/{id}/links', { params: { path: { type, id }, query: type === 'tv' ? { season, episode } : {} } })),
    staleTime: 0,
    gcTime: 0,
  });
  const episodes = useQuery({
    queryKey: ['season', id, season],
    queryFn: () => unwrap(api.GET('/titles/tv/{id}/seasons/{season}', { params: { path: { id, season } } })),
    enabled: type === 'tv',
  });
  const progress = useQuery({
    queryKey: ['library', 'progress', type, id],
    queryFn: () => unwrap(api.GET('/library/progress/{type}/{id}', { params: { path: { type, id } } })),
  });

  // Same site (then hoster, then language) as the previous episode first, the server order otherwise.
  const list = useMemo(() => {
    const ls = links.data ?? [];
    if (!params.src) return ls;
    const score = (l: Link) => (l.source === params.src ? 4 : 0) + (l.hoster === params.host ? 2 : 0) + (l.lang === params.lang ? 1 : 0);
    return [...ls].sort((a, b) => score(b) - score(a));
  }, [links.data, params.src, params.host, params.lang]);

  const [index, setIndex] = useState(0);
  const [status, setStatus] = useState<Status>('searching');
  const [stream, setStream] = useState<{ url: string; kind: 'hls' | 'file' } | null>(null);
  const [audio, setAudio] = useState<{ tracks: string[]; current: number } | null>(null);
  const [menu, setMenu] = useState(false);
  const [chrome, setChrome] = useState(true);
  const resumeAt = useRef<number | null>(null);
  const position = useRef(0);
  const hideTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Resume where this profile stopped this movie/episode.
  useEffect(() => {
    const p = progress.data?.find((x) => x.season === season && x.episode === episode);
    resumeAt.current = p && p.position > 30 && p.position < p.duration * 0.95 ? p.position : null;
    position.current = 0;
    setIndex(0);
  }, [progress.data, season, episode]);

  const next = useCallback(() => setIndex((i) => i + 1), []);

  // Resolve the current link; a failure moves on to the next one.
  useEffect(() => {
    const done = links.isPending || progress.isPending ? 'searching' : !list.length ? 'none' : index >= list.length ? 'failed' : null;
    setStream(null);
    setAudio(null);
    if (done) {
      setStatus(done);
      return;
    }
    let cancelled = false;
    setStatus('loading');
    unwrap(api.POST('/play', { body: { token: list[index].token } }))
      .then((res) => !cancelled && setStream({ url: absolute(res.url), kind: res.kind === 'hls' ? 'hls' : 'file' }))
      .catch(() => !cancelled && next());
    return () => {
      cancelled = true;
    };
  }, [links.isPending, progress.isPending, list, index, next]);

  // Watchdog: a link that resolved but never starts (stalled CDN) gives way to the next one.
  useEffect(() => {
    if (!stream || status === 'playing') return;
    const timer = setTimeout(next, 15_000);
    return () => clearTimeout(timer);
  }, [stream, status, next]);

  useEffect(
    () => () => {
      queryClient.invalidateQueries({ queryKey: ['library', 'progress'] });
    },
    [queryClient],
  );

  const saveProgress = (pos: number, duration: number) => {
    position.current = pos;
    const d = details.data;
    if (!d) return;
    api.PUT('/library/progress', {
      body: { type, id, season, episode, title: d.title, poster: d.poster, backdrop: d.backdrop, position: pos, duration },
      keepalive: true,
    });
  };

  const nextEpisode = (() => {
    if (type !== 'tv' || !episodes.data || !details.data) return null;
    if (episode < episodes.data.length) return { s: season, e: episode + 1 };
    return details.data.seasons.some((s) => s.number === season + 1) ? { s: season + 1, e: 1 } : null;
  })();

  const goNext = () => {
    if (!nextEpisode) return;
    const cur = list[index];
    const pref = cur ? `&src=${encodeURIComponent(cur.source)}&host=${encodeURIComponent(cur.hoster)}&lang=${encodeURIComponent(cur.lang)}` : '';
    router.replace(`/watch/tv/${id}?s=${nextEpisode.s}&e=${nextEpisode.e}${pref}` as Href);
  };

  const switchTo = (i: number) => {
    if (position.current) resumeAt.current = position.current;
    setMenu(false);
    setIndex(i);
  };

  const poke = () => {
    setChrome(true);
    clearTimeout(hideTimer.current);
    hideTimer.current = setTimeout(() => setChrome(false), 3000);
  };

  const epName = episodes.data?.find((e) => e.number === episode)?.name;
  const backTo = `/title/${type}/${id}` as Href;
  const current = list[Math.min(index, list.length - 1)];
  const showChrome = chrome || menu || status !== 'playing';

  return (
    <View style={styles.watch} onPointerMove={poke}>
      {stream && (
        <Player
          key={stream.url}
          url={stream.url}
          kind={stream.kind}
          startAt={resumeAt.current}
          audioTrack={audio?.current}
          onReady={() => setStatus('playing')}
          onError={next}
          onEnded={goNext}
          onProgress={saveProgress}
          onAudioTracks={(tracks, cur) => setAudio({ tracks, current: cur })}
        />
      )}

      {showChrome && (
        <View style={[styles.top, { paddingHorizontal: gutter }]} pointerEvents="box-none">
          <Focusable href={backTo} accessibilityLabel={t('common.back')} onFocus={poke} style={(active) => [styles.iconBtn, active && styles.iconBtnActive]}>
            <Icon d={icons.back} size={30} />
          </Focusable>
          <View style={{ flex: 1, minWidth: 0 }}>
            <Text style={styles.title} numberOfLines={1}>
              {details.data?.title}
            </Text>
            {type === 'tv' && (
              <Text style={ui.muted} numberOfLines={1}>
                {t('common.episodeShort', { season, episode })}
                {epName ? ` « ${epName} »` : ''}
              </Text>
            )}
          </View>
          {list.length > 0 && (
            <Button kind="grey" small label={current ? [current.source, current.quality].filter(Boolean).join(' · ') : t('watch.sources')} onPress={() => setMenu(!menu)} />
          )}
          {nextEpisode && <Button small icon={icons.next} label={t('watch.nextEpisode')} onPress={goNext} />}
        </View>
      )}

      {menu && (
        <View style={[styles.menu, { right: gutter }]}>
          <ScrollView contentContainerStyle={{ gap: 16, padding: 20 }}>
            {audio && (
              <View style={{ gap: 8 }}>
                <Text style={ui.muted}>{t('watch.audio')}</Text>
                <View style={styles.chips}>
                  {audio.tracks.map((t, i) => (
                    <Chip key={i} label={t} selected={i === audio.current} onPress={() => setAudio({ ...audio, current: i })} />
                  ))}
                </View>
              </View>
            )}
            <View style={{ gap: 8 }}>
              <Text style={ui.muted}>{t('watch.sources')}</Text>
              {list.map((l, i) => (
                <Chip key={i} label={linkLabel(l)} selected={i === index} onPress={() => switchTo(i)} />
              ))}
            </View>
          </ScrollView>
        </View>
      )}

      {status !== 'playing' && (
        <View style={styles.status} pointerEvents="box-none">
          {(status === 'searching' || status === 'loading') && <ActivityIndicator size="large" color={colors.red} />}
          {status === 'searching' && <Text style={ui.text}>{t('watch.searching')}</Text>}
          {status === 'loading' && (
            <Text style={ui.text}>
              {t('watch.loadingFrom', { source: current?.source ?? '', hoster: current?.hoster ?? '' })}
              {index > 0 && <Text style={ui.muted}>{t('watch.attempt', { n: index + 1, total: list.length })}</Text>}
            </Text>
          )}
          {(status === 'none' || status === 'failed') && (
            <>
              <Text style={ui.text}>{status === 'none' ? t('watch.noSource') : t('watch.allFailed')}</Text>
              <View style={ui.heroActions}>
                {status === 'failed' && <Button label={t('common.retry')} onPress={() => setIndex(0)} hasTVPreferredFocus />}
                <Button kind="grey" label={t('common.back')} href={backTo} hasTVPreferredFocus={status === 'none'} />
              </View>
            </>
          )}
        </View>
      )}
    </View>
  );
}

/** "Purstream · 1080p · MULTI", with the hoster when the site has several, and a mark on dead links. */
function linkLabel(l: Link) {
  const parts = [l.source, l.quality, l.lang, l.hoster !== 'Direct' ? l.hoster : ''].filter(Boolean);
  return parts.join(' · ') + (l.dead ? t('watch.offline') : '');
}

const styles = StyleSheet.create({
  watch: { flex: 1, backgroundColor: '#000' },
  top: { position: 'absolute', top: 0, left: 0, right: 0, flexDirection: 'row', alignItems: 'center', gap: 16, paddingVertical: 16, backgroundColor: 'rgba(0,0,0,0.55)' },
  iconBtn: { padding: 6, borderRadius: 30, borderWidth: 2, borderColor: 'transparent' },
  iconBtnActive: { borderColor: '#fff' },
  title: { color: '#fff', fontSize: 18, fontWeight: '700' },
  menu: { position: 'absolute', top: 80, maxHeight: '70%', width: 360, maxWidth: '90%', backgroundColor: 'rgba(20,20,20,0.95)', borderRadius: 6, borderWidth: 1, borderColor: '#333' },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  status: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', gap: 16, padding: 16 },
});
