import type { Release } from './UpdatePrompt';

/** The web app updates with the server, nothing to install. */
export function UpdatePrompt(_: { onShow?: (shown: boolean) => void }) {
  return null;
}

export const offerUpdate = (_: Release) => {};
