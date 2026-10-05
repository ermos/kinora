import { useEvent } from 'expo';
import { useVideoPlayer, VideoView } from 'expo-video';
import { useEffect, useRef } from 'react';
import { StyleSheet } from 'react-native';
import { PROGRESS_INTERVAL, type PlayerProps } from './Player.types';
import { t } from '../i18n';

/**
 * Phones, Android TV and tvOS: expo-video (ExoPlayer / AVPlayer) plays HLS natively and its native controls
 * already handle the remote. The proxied URL carries its own credential, no cookie needed.
 * ponytail: not exercised yet, the web build is the only target for now.
 */
export function Player({ url, startAt, audioTrack, onReady, onError, onEnded, onProgress, onAudioTracks }: PlayerProps) {
  const cb = useRef({ onReady, onError, onEnded, onProgress, onAudioTracks });
  cb.current = { onReady, onError, onEnded, onProgress, onAudioTracks };

  const player = useVideoPlayer(url, (p) => {
    p.timeUpdateEventInterval = PROGRESS_INTERVAL / 1000;
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
      player.addListener('timeUpdate', ({ currentTime }) => {
        if (player.playing && player.duration > 0 && currentTime > 5) cb.current.onProgress(currentTime, player.duration);
      }),
      player.addListener('availableAudioTracksChange', ({ availableAudioTracks }) => {
        if (availableAudioTracks.length > 1) {
          const current = availableAudioTracks.findIndex((tr) => tr.id === player.audioTrack?.id);
          cb.current.onAudioTracks(availableAudioTracks.map((tr, i) => tr.label || tr.language || t('watch.track', { n: i + 1 })), Math.max(0, current));
        }
      }),
    ];
    return () => {
      if (player.duration > 0 && player.currentTime > 5) cb.current.onProgress(player.currentTime, player.duration);
      subs.forEach((s) => s.remove());
    };
  }, [player]);

  useEffect(() => {
    const track = audioTrack !== undefined ? player.availableAudioTracks[audioTrack] : undefined;
    if (track) player.audioTrack = track;
  }, [audioTrack, player]);

  return <VideoView player={player} style={StyleSheet.absoluteFill} nativeControls contentFit="contain" allowsPictureInPicture />;
}
