import type { Ref } from 'react';

/** Imperative controls, for the buttons the watch screen draws over the video. */
export type PlayerHandle = { seek: (seconds: number) => void; play: () => void; pause: () => void };

/** Contract of the platform players: Player.web.tsx (hls.js) and Player.tsx (expo-video, phones and TVs). */
export type PlayerProps = {
  ref?: Ref<PlayerHandle>;
  /** Absolute or same-origin stream URL (the server proxy). */
  url: string;
  kind: 'hls' | 'file';
  /** Seconds to resume from, null to start at 0. */
  startAt: number | null;
  muted?: boolean;
  /** Index into the tracks reported by onAudioTracks. */
  audioTrack?: number;
  onReady: () => void;
  /** Fatal playback error: the screen tries the next link. */
  onError: () => void;
  onEnded: () => void;
  /** Called every 10 s while playing, on pause and when unmounting. */
  onProgress: (position: number, duration: number) => void;
  onAudioTracks: (tracks: string[], current: number) => void;
  /** Called about every second while playing, for what depends on the position (skip buttons). */
  onTime?: (position: number, duration: number) => void;
  onPlaying?: (playing: boolean) => void;
};

export const PROGRESS_INTERVAL = 10_000;
