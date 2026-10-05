import type Hls from 'hls.js';
import { useEffect, useImperativeHandle, useRef } from 'react';
import { PROGRESS_INTERVAL, type PlayerProps } from './Player.types';
import { t } from '../i18n';

/** Web player: native <video>, with hls.js where the browser cannot play HLS itself (all but Safari). */
export function Player({ ref, url, kind, startAt, muted, audioTrack, onReady, onError, onEnded, onProgress, onAudioTracks, onTime, onPlaying }: PlayerProps) {
  const video = useRef<HTMLVideoElement>(null);
  const hls = useRef<Hls | null>(null);
  // Callbacks change every render; the effects below must only restart when the stream changes.
  const cb = useRef({ onReady, onError, onEnded, onProgress, onAudioTracks, onTime, onPlaying });
  cb.current = { onReady, onError, onEnded, onProgress, onAudioTracks, onTime, onPlaying };

  useImperativeHandle(ref, () => ({
    seek: (seconds) => {
      if (video.current) video.current.currentTime = seconds;
    },
    play: () => {
      video.current?.play().catch(() => {});
    },
    pause: () => video.current?.pause(),
  }));

  useEffect(() => {
    if (video.current) video.current.muted = !!muted;
  }, [muted]);

  useEffect(() => {
    const el = video.current!;
    const start = () => {
      if (startAt) el.currentTime = startAt;
      el.play().catch(() => {}); // autoplay may be blocked: the play button stays shown
      cb.current.onReady();
    };
    el.addEventListener('loadedmetadata', start, { once: true });
    el.onerror = () => cb.current.onError();

    let cancelled = false;
    // hls.js wherever Media Source Extensions exist (it exposes audio tracks), native HLS otherwise (older iOS).
    const useHls = kind === 'hls' && ('MediaSource' in window || 'ManagedMediaSource' in window);
    if (useHls) {
      // hls.js is loaded on demand: only the player needs it.
      import('hls.js').then(({ default: HlsLib }) => {
        if (cancelled) return;
        // Fail fast on a dead CDN: the screen moves on to the next link.
        const h = new HlsLib({ manifestLoadPolicy: { default: { maxTimeToFirstByteMs: 8000, maxLoadTimeMs: 10000, timeoutRetry: null, errorRetry: null } } });
        hls.current = h;
        h.on(HlsLib.Events.AUDIO_TRACKS_UPDATED, () => {
          if (h.audioTracks.length > 1) {
            cb.current.onAudioTracks(h.audioTracks.map((tr) => tr.name || tr.lang || t('watch.track', { n: tr.id + 1 })), h.audioTrack);
          }
        });
        h.on(HlsLib.Events.ERROR, (_, data) => data.fatal && cb.current.onError());
        h.loadSource(url);
        h.attachMedia(el);
      }, () => cb.current.onError());
    } else {
      el.src = url;
    }

    const report = () => el.duration && isFinite(el.duration) && el.currentTime > 5 && cb.current.onProgress(el.currentTime, el.duration);
    const timer = setInterval(() => !el.paused && report(), PROGRESS_INTERVAL);
    el.addEventListener('pause', report);
    return () => {
      cancelled = true;
      report();
      clearInterval(timer);
      el.removeEventListener('pause', report);
      el.removeEventListener('loadedmetadata', start);
      el.onerror = null;
      hls.current?.destroy();
      hls.current = null;
      el.removeAttribute('src');
      el.load();
    };
  }, [url, kind, startAt]);

  useEffect(() => {
    if (hls.current && audioTrack !== undefined && hls.current.audioTrack !== audioTrack) hls.current.audioTrack = audioTrack;
  }, [audioTrack]);

  return (
    <video
      ref={video}
      playsInline
      onEnded={() => cb.current.onEnded()}
      onPlay={() => cb.current.onPlaying?.(true)}
      onPause={() => cb.current.onPlaying?.(false)}
      onTimeUpdate={(e) => {
        const el = e.currentTarget;
        if (el.duration && isFinite(el.duration)) cb.current.onTime?.(el.currentTime, el.duration);
      }}
      style={{ width: '100%', height: '100%', backgroundColor: '#000' }}
    />
  );
}
