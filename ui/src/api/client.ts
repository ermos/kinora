import createClient from 'openapi-fetch';
import { useSyncExternalStore } from 'react';
import { Platform } from 'react-native';
import { t, type TKey } from '../i18n';
import { getItem, setItem } from '../lib/storage';
import type { components, paths } from './schema';

type S = components['schemas'];
export type Item = S['tmdb.Item'];
export type Details = S['tmdb.Details'];
export type Episode = S['tmdb.Episode'];
export type Row = S['api.row'];
export type Link = S['api.link'];
export type User = S['store.User'];
export type Profile = S['store.Profile'];
export type Progress = S['store.Progress'];
export type ListItem = S['store.ListItem'];
export type MediaType = 'movie' | 'tv';

/**
 * Server origin. The web app is served by the Go binary, so it is relative. Native and TV apps will need the
 * address of the instance (EXPO_PUBLIC_API_URL for now, a "server address" screen when the TV builds land).
 */
export const API_ORIGIN = Platform.OS === 'web' ? '' : (process.env.EXPO_PUBLIC_API_URL ?? '');

/** Absolute URL for a server path, e.g. the proxied stream handed to a native player. */
export const absolute = (path: string) => (path.startsWith('http') ? path : API_ORIGIN + path);

export const api = createClient<paths>({ baseUrl: API_ORIGIN + '/api/v1', credentials: 'include' });

// --- selected profile: kept in memory for synchronous access, persisted per device.

const PROFILE_KEY = 'kinora.profile';
let profile: Profile | null = null;
const listeners = new Set<() => void>();

export async function loadProfile() {
  try {
    profile = JSON.parse((await getItem(PROFILE_KEY)) ?? 'null');
  } catch {
    profile = null;
  }
}

export function currentProfile() {
  return profile;
}

export function setCurrentProfile(p: Profile | null) {
  profile = p;
  listeners.forEach((l) => l());
  void setItem(PROFILE_KEY, p ? JSON.stringify(p) : null);
}

export function useProfile() {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => profile,
  );
}

/** Called on 401/403 so the app can route to login or the profile picker. */
export const authEvents = { onUnauthorized: () => {}, onUnknownProfile: () => {} };

api.use({
  onRequest({ request }) {
    if (profile) request.headers.set('X-Profile-ID', String(profile.id));
    // The server rejects non-JSON writes (CSRF guard), even body-less ones.
    if (request.method !== 'GET' && !request.headers.has('Content-Type')) {
      request.headers.set('Content-Type', 'application/json');
    }
    return request;
  },
  onResponse({ request, response }) {
    const path = new URL(request.url, 'http://x').pathname;
    if (response.status === 401 && !path.endsWith('/me') && !path.includes('/auth/')) authEvents.onUnauthorized();
    if (response.status === 403 && path.includes('/library/')) {
      setCurrentProfile(null); // profile deleted from another device
      authEvents.onUnknownProfile();
    }
    return response;
  },
});

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}

/** Unwraps an openapi-fetch result, throwing the API error translated from its code. */
export async function unwrap<T>(p: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, error, response } = await p;
  if (!response.ok) {
    const e = error as { code?: string; error?: string } | undefined;
    const msg = e?.code ? t(`errors.${e.code}` as TKey) : (e?.error ?? response.statusText);
    throw new ApiError(msg, response.status);
  }
  return data as T;
}

export function img(path: string | undefined, size: 'w300' | 'w342' | 'w780' | 'w1280' = 'w780') {
  return path ? `https://image.tmdb.org/t/p/${size}${path}` : undefined;
}
