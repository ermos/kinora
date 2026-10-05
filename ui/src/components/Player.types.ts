/** Contract of the platform players: Player.web.tsx (hls.js) and Player.tsx (expo-video, phones and TVs). */
export type PlayerProps = {
  /** Absolute or same-origin stream URL (the server proxy). */
  url: string;
  kind: 'hls' | 'file';
  /** Seconds to resume from, null to start at 0. */
  startAt: number | null;
  /** Index into the tracks reported by onAudioTracks. */
  audioTrack?: number;
  onReady: () => void;
  /** Fatal playback error: the screen tries the next link. */
  onError: () => void;
  onEnded: () => void;
  /** Called every 10 s while playing, on pause and when unmounting. */
  onProgress: (position: number, duration: number) => void;
  onAudioTracks: (tracks: string[], current: number) => void;
};

export const PROGRESS_INTERVAL = 10_000;
