import { useEvent } from 'expo';
import { useVideoPlayer, VideoView } from 'expo-video';
import { useEffect, useImperativeHandle, useMemo, useRef } from 'react';
import { StyleSheet } from 'react-native';
import { PROGRESS_INTERVAL, type PlayerProps } from './Player.types';
import { t } from '../i18n';

/**
 * Phones, Android TV and tvOS: expo-video (ExoPlayer / AVPlayer) plays HLS natively. The watch screen draws its own
 * controls over it, like on the web. The proxied URL carries its own credential, no cookie needed.
 * ponytail: not exercised yet, the web build is the only target for now.
 */
export function Player({ ref, url, kind, startAt, muted, audioTrack, onReady, onError, onEnded, onProgress, onAudioTracks, onTime, onPlaying }: PlayerProps) {
  const cb = useRef({ onReady, onError, onEnded, onProgress, onAudioTracks, onTime, onPlaying });
  cb.current = { onReady, onError, onEnded, onProgress, onAudioTracks, onTime, onPlaying };
  const lastReport = useRef(0);
  const last = useRef({ position: 0, duration: 0 });

  // The proxy URL has no .m3u8 extension: without the content type, ExoPlayer reads HLS as a plain file and fails.
  const source = useMemo(() => ({ uri: url, contentType: kind === 'hls' ? ('hls' as const) : ('auto' as const) }), [url, kind]);
  const player = useVideoPlayer(source, (p) => {
    p.timeUpdateEventInterval = 1;
    if (startAt) p.currentTime = startAt;
    p.play();
  });

  const { status } = useEvent(player, 'statusChange', { status: player.status });
  useEffect(() => {
    if (status === 'readyToPlay') cb.current.onReady();
    if (status === 'error') cb.current.onError();
  }, [status]);

  useEffect(() => {
    const subs = [
      player.addListener('playToEnd', () => cb.current.onEnded()),
      player.addListener('playingChange', ({ isPlaying }) => cb.current.onPlaying?.(isPlaying)),
      player.addListener('timeUpdate', ({ currentTime }) => {
        if (player.duration > 0) {
          last.current = { position: currentTime, duration: player.duration };
          cb.current.onTime?.(currentTime, player.duration);
        }
        if (player.playing && player.duration > 0 && currentTime > 5 && Date.now() - lastReport.current >= PROGRESS_INTERVAL) {
          lastReport.current = Date.now();
          cb.current.onProgress(currentTime, player.duration);
        }
      }),
      player.addListener('availableAudioTracksChange', ({ availableAudioTracks }) => {
        if (availableAudioTracks.length > 1) {
          const current = availableAudioTracks.findIndex((tr) => tr.id === player.audioTrack?.id);
          cb.current.onAudioTracks(availableAudioTracks.map((tr, i) => tr.label || tr.language || t('watch.track', { n: i + 1 })), Math.max(0, current));
        }
      }),
    ];
    return () => {
      // useVideoPlayer releases the native player before this cleanup runs, and touching a released player crashes
      // the app: the last position comes from the time updates instead.
      const { position, duration } = last.current;
      if (duration > 0 && position > 5) cb.current.onProgress(position, duration);
      subs.forEach((s) => {
        try {
          s.remove();
        } catch {
          // already gone with the released player
        }
      });
    };
  }, [player]);

  useImperativeHandle(ref, () => ({
    seek: (seconds) => {
      player.currentTime = seconds;
    },
    play: () => player.play(),
    pause: () => player.pause(),
  }));

  useEffect(() => {
    player.muted = !!muted;
  }, [muted, player]);

  useEffect(() => {
    const track = audioTrack !== undefined ? player.availableAudioTracks[audioTrack] : undefined;
    if (track) player.audioTrack = track;
  }, [audioTrack, player]);

  return <VideoView player={player} style={StyleSheet.absoluteFill} nativeControls={false} contentFit="contain" allowsPictureInPicture />;
}
