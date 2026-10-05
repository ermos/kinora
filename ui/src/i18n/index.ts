import { useSyncExternalStore } from 'react';
import type { components } from '../api/schema';
import { en } from './en';
import { fr } from './fr';

/** Language codes the server supports; adding one in Go (scraper.Languages + enums tag) breaks the build
 * until its dictionary is added below. */
export type LangCode = components['schemas']['api.language']['code'];

type Dict = typeof fr;
type DeepString<T> = { [K in keyof T]: T[K] extends string ? string : DeepString<T[K]> };

const dictionaries: Record<LangCode, DeepString<Dict>> = { en, fr };

/** Dotted keys of the dictionary: "nav.home", "watch.loadingFrom"... */
type Keys<T, P extends string = ''> = {
  [K in keyof T & string]: T[K] extends string ? `${P}${K}` : Keys<T[K], `${P}${K}.`>;
}[keyof T & string];
export type TKey = Keys<Dict>;

let current: LangCode = 'fr';
const listeners = new Set<() => void>();

export function setLanguage(code: LangCode) {
  if (!(code in dictionaries)) return;
  if (typeof document !== 'undefined') document.documentElement.lang = code; // web: screen readers, hyphenation
  if (code === current) return;
  current = code;
  listeners.forEach((l) => l());
}

/** Re-renders the caller when the instance language changes (setup picker). */
export function useLanguage() {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l);
      return () => listeners.delete(l);
    },
    () => current,
  );
}

/** Translates a key, replacing {placeholders} with vars. */
export function t(key: TKey, vars?: Record<string, string | number>): string {
  let s: unknown = dictionaries[current];
  for (const part of key.split('.')) s = (s as Record<string, unknown>)[part];
  let out = typeof s === 'string' ? s : key;
  for (const [k, v] of Object.entries(vars ?? {})) out = out.replaceAll(`{${k}}`, String(v));
  return out;
}
