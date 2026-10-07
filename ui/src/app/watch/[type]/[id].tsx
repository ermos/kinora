import { useQuery, useQueryClient } from '@tanstack/react-query';
import { router, useLocalSearchParams, type Href } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, BackHandler, Platform, Pressable, ScrollView, StyleSheet, Text, TVEventHandler, View } from 'react-native';
import { absolute, api, unwrap, type Link, type MediaType, type Segment } from '../../../api/client';
import { Focusable } from '../../../components/Focusable';
import { Player } from '../../../components/Player';
import type { PlayerHandle } from '../../../components/Player.types';
import { PlayerControls, SEEK_STEP, toggleFullscreen } from '../../../components/PlayerControls';
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

  // Same site (then hoster, then language) as the previous episode, or as the one this episode was stopped on, first.
  // The server order otherwise.
  const saved = progress.data?.find((x) => x.season === season && x.episode === episode && x.position < x.duration * 0.95);
  const [src, host, lang] = params.src ? [params.src, params.host, params.lang] : [saved?.source, saved?.hoster, saved?.lang];
  const list = useMemo(() => {
    const ls = links.data ?? [];
    if (!src) return ls;
    const score = (l: Link) => (l.source === src ? 4 : 0) + (l.hoster === host ? 2 : 0) + (l.lang === lang ? 1 : 0);
    return [...ls].sort((a, b) => score(b) - score(a));
  }, [links.data, src, host, lang]);

  const [index, setIndex] = useState(0);
  const [status, setStatus] = useState<Status>('searching');
  const [stream, setStream] = useState<{ url: string; kind: 'hls' | 'file' } | null>(null);
  const [audio, setAudio] = useState<{ tracks: string[]; current: number } | null>(null);
  const [menu, setMenu] = useState(false);
  const [chrome, setChrome] = useState(true);
  const [duration, setDuration] = useState(0);
  const [time, setTime] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(false);
  const [skip, setSkip] = useState<Segment | null>(null);
  const player = useRef<PlayerHandle>(null);
  const resumeAt = useRef<number | null>(null);
  const position = useRef(0);
  const leaving = useRef(false);
  const hideTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const scrubbing = useRef(false);

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
    setDuration(0);
    setTime(0);
    setPlaying(false);
    setSkip(null);
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
      queryClient.invalidateQueries({ queryKey: ['library'] }); // progress, watched titles
      queryClient.invalidateQueries({ queryKey: ['foryou'] }); // recommendations follow what was just watched
    },
    [queryClient],
  );

  // Opening and ending, timed for the length of this file (empty when unknown or turned off in the profile).
  const segments = useQuery({
    queryKey: ['segments', type, id, season, episode, duration],
    queryFn: () =>
      unwrap(
        api.GET('/titles/{type}/{id}/segments', {
          params: { path: { type, id }, query: { duration, ...(type === 'tv' ? { season, episode } : {}) } },
        }),
      ),
    enabled: duration > 0,
    staleTime: Infinity,
  });

  const onTime = (pos: number, dur: number) => {
    setTime(pos);
    const d = Math.round(dur);
    if (d !== duration) setDuration(d);
    // Hidden during the last second, so the button doesn't flash once the segment is over.
    const seg = segments.data?.find((s) => pos >= s.start && pos < s.end - 1) ?? null;
    if (seg !== skip) setSkip(seg);
  };

  const saveProgress = (pos: number, duration: number) => {
    if (leaving.current) return; // the player closing after "next episode" would undo the episode marked as watched
    position.current = pos;
    const d = details.data;
    if (!d) return;
    const link = list[index];
    api.PUT('/library/progress', {
      body: {
        type,
        id,
        season,
        episode,
        title: d.title,
        poster: d.poster,
        backdrop: d.backdrop,
        position: pos,
        duration,
        source: link?.source,
        hoster: link?.hoster,
        lang: link?.lang,
      },
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
    // Moving on counts the current episode as watched, wherever it was stopped (credits skipped...).
    if (duration > 0) saveProgress(duration, duration);
    leaving.current = true;
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

  const toggle = () => (playing ? player.current?.pause() : player.current?.play());
  const seek = (s: number) => {
    player.current?.seek(s);
    setTime(s);
  };

  // Keyboard, like Netflix on the web: space or K plays/pauses, arrows seek, F fullscreen, M mute.
  const epName = episodes.data?.find((e) => e.number === episode)?.name;
  const backTo = `/title/${type}/${id}` as Href;
  const current = list[Math.min(index, list.length - 1)];
  const showChrome = chrome || menu || status !== 'playing' || !playing;

  const keys = useRef({ toggle, seek, time, duration, poke, showChrome, skip, status });
  keys.current = { toggle, seek, time, duration, poke, showChrome, skip, status };
  useEffect(() => {
    if (Platform.OS !== 'web') return;
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.metaKey || e.ctrlKey || e.altKey) return;
      const k = keys.current;
      const actions: Record<string, () => void> = {
        ' ': k.toggle,
        k: k.toggle,
        ArrowLeft: () => k.seek(Math.max(0, k.time - SEEK_STEP)),
        ArrowRight: () => k.seek(Math.min(k.duration, k.time + SEEK_STEP)),
        f: toggleFullscreen,
        m: () => setMuted((m) => !m),
      };
      const action = actions[e.key];
      if (!action) return;
      e.preventDefault();
      action();
      k.poke();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  // TV remote. Controls hidden: the full screen catcher below has the focus, center toggles, left and right seek, any
  // key shows the controls with the progress bar focused. There left and right keep seeking, center toggles, down goes
  // to the buttons. Android sends the release of each key (and repeated long* events while held).
  useEffect(() => {
    if (!Platform.isTV) return;
    const sub = TVEventHandler.addListener((e) => {
      const k = keys.current;
      if (k.status !== 'playing' || e.eventKeyAction === 0) return;
      const free = !k.showChrome || scrubbing.current;
      const seekBy = (d: number) => k.seek(Math.min(k.duration, Math.max(0, k.time + d)));
      const type = e.eventType;
      if (type === 'playPause' || (type === 'select' && scrubbing.current)) k.toggle();
      else if (type === 'rewind' || type === 'longLeft' || (type === 'left' && free)) seekBy(-SEEK_STEP);
      else if (type === 'fastForward' || type === 'longRight' || (type === 'right' && free)) seekBy(SEEK_STEP);
      k.poke();
    });
    return () => sub?.remove();
  }, []);

  // Back closes the controls first, then leaves the player.
  const overlay = chrome || menu;
  useEffect(() => {
    if (!overlay) return;
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      clearTimeout(hideTimer.current);
      setMenu(false);
      setChrome(false);
      return true;
    });
    return () => sub.remove();
  }, [overlay]);

  return (
    <View style={styles.watch} onPointerMove={poke}>
      {stream && (
        <Player
          ref={player}
          key={stream.url}
          url={stream.url}
          kind={stream.kind}
          startAt={resumeAt.current}
          muted={muted}
          audioTrack={audio?.current}
          onReady={() => {
            leaving.current = false;
            setStatus('playing');
          }}
          onError={next}
          onEnded={goNext}
          onProgress={saveProgress}
          onAudioTracks={(tracks, cur) => setAudio({ tracks, current: cur })}
          onTime={onTime}
          onPlaying={setPlaying}
        />
      )}

      {/* Click or tap on the picture: shows the controls, then plays/pauses. Below every control. */}
      {status === 'playing' && (
        <Pressable
          // TV: keys only reach the app when something has the focus, this takes it while the controls are hidden
          focusable={Platform.isTV && !showChrome}
          hasTVPreferredFocus={Platform.isTV && !showChrome}
          style={StyleSheet.absoluteFill}
          onPress={() => {
            if (showChrome || Platform.isTV) toggle();
            poke();
          }}
        />
      )}

      {skip && status === 'playing' && (
        <View style={[styles.skip, { right: gutter, bottom: showChrome ? 150 : 60 }]}>
          <Button
            kind="grey"
            label={skip.kind === 'intro' ? t('watch.skipIntro') : nextEpisode ? t('watch.nextEpisode') : t('watch.skipCredits')}
            icon={skip.kind === 'credits' && nextEpisode ? icons.next : undefined}
            onPress={() => (skip.kind === 'credits' && nextEpisode ? goNext() : player.current?.seek(skip.end))}
            hasTVPreferredFocus
          />
        </View>
      )}

      {showChrome && (
        <View style={[styles.top, { paddingHorizontal: gutter }]} pointerEvents="box-none">
          <Focusable href={backTo} accessibilityLabel={t('common.back')} onFocus={poke} style={(active) => [styles.iconBtn, active && styles.iconBtnActive]}>
            <Icon d={icons.back} size={30} />
          </Focusable>
        </View>
      )}

      {showChrome && status === 'playing' && (
        <PlayerControls
          position={time}
          duration={duration}
          playing={playing}
          muted={muted}
          title={details.data?.title ?? ''}
          subtitle={type === 'tv' ? `${t('common.episodeShort', { season, episode })}${epName ? ` « ${epName} »` : ''}` : undefined}
          onToggle={toggle}
          onSeek={seek}
          onMute={() => setMuted(!muted)}
          onPoke={poke}
          onScrubFocus={(f) => (scrubbing.current = f)}
          actions={
            <>
              {nextEpisode && <Button kind="grey" small icon={icons.next} label={t('watch.nextEpisode')} onPress={goNext} onFocus={poke} />}
              {list.length > 0 && (
                <Button kind="grey" small label={current ? [current.source, current.quality].filter(Boolean).join(' · ') : t('watch.sources')} onPress={() => setMenu(!menu)} onFocus={poke} />
              )}
            </>
          }
        />
      )}

      {menu && (
        <View style={[styles.menu, { right: gutter }]} onPointerMove={poke}>
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
  top: { position: 'absolute', top: 0, left: 0, flexDirection: 'row', alignItems: 'center', paddingVertical: 16 },
  iconBtn: { padding: 6, borderRadius: 30, borderWidth: 2, borderColor: 'transparent' },
  iconBtnActive: { borderColor: '#fff' },
  menu: { position: 'absolute', bottom: 120, maxHeight: '65%', width: 360, maxWidth: '90%', backgroundColor: 'rgba(20,20,20,0.95)', borderRadius: 6, borderWidth: 1, borderColor: '#333' },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  skip: { position: 'absolute', zIndex: 3 },
  status: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', gap: 16, padding: 16 },
});
