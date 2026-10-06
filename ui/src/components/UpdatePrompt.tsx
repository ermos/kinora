import Constants from 'expo-constants';
import { File, Paths } from 'expo-file-system';
import { startActivityAsync } from 'expo-intent-launcher';
import { useEffect, useState } from 'react';
import { BackHandler, Platform, ScrollView, StyleSheet, Text, TVFocusGuideView, View } from 'react-native';
import { getItem, setItem } from '../lib/storage';
import { t } from '../i18n';
import { colors } from '../theme';
import { Button, Spinner, styles as ui } from './ui';

/**
 * Where the latest release is described, set at build time. Same shape as GitHub's "latest release" API
 * (https://api.github.com/repos/<owner>/<repo>/releases/latest): tag_name, body (the notes), assets[] with the APK.
 */
const UPDATE_URL = process.env.EXPO_PUBLIC_UPDATE_URL;
const SKIPPED = 'update.skipped';
const GRANT_READ_URI_PERMISSION = 1;

type Release = { version: string; notes: string; apk: string };

/** "1.2.10" > "1.2.9": numeric parts compared in order. */
export function newer(a: string, b: string) {
  const pa = a.split('.').map(Number);
  const pb = b.split('.').map(Number);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] || 0) - (pb[i] || 0);
    if (d) return d > 0;
  }
  return false;
}

async function latestRelease(): Promise<Release | null> {
  if (!UPDATE_URL) return null;
  const res = await fetch(UPDATE_URL, { headers: { Accept: 'application/vnd.github+json' } });
  if (!res.ok) return null;
  const r = (await res.json()) as { tag_name?: string; body?: string; assets?: { name?: string; browser_download_url?: string }[] };
  const apk = r.assets?.find((a) => a.name?.endsWith('.apk'))?.browser_download_url;
  return r.tag_name && apk ? { version: r.tag_name.replace(/^v/, ''), notes: r.body ?? '', apk } : null;
}

/**
 * Android: on launch, offers the latest release when it is newer than this build and not skipped. Updating downloads
 * the APK and opens the system installer (it asks to confirm, and the first time to allow installs from kinora).
 */
export function UpdatePrompt({ onShow }: { onShow?: (shown: boolean) => void }) {
  const current = Constants.expoConfig?.version ?? '0';
  const [release, setRelease] = useState<Release | null>(null);
  const [phase, setPhase] = useState<'ask' | 'downloading' | 'failed'>('ask');
  useEffect(() => onShow?.(!!release), [release, onShow]);
  // Asked once the app behind is hidden: hiding it moves the focus, the button would lose it right away.
  const [focusReady, setFocusReady] = useState(false);
  useEffect(() => {
    if (!release) return;
    const timer = setTimeout(() => setFocusReady(true), 200);
    return () => clearTimeout(timer);
  }, [release]);

  // Back on the remote is "Later".
  useEffect(() => {
    if (!release) return;
    const sub = BackHandler.addEventListener('hardwareBackPress', () => {
      setRelease(null);
      return true;
    });
    return () => sub.remove();
  }, [release]);

  useEffect(() => {
    if (Platform.OS !== 'android') return;
    (async () => {
      const r = await latestRelease();
      if (r && newer(r.version, current) && (await getItem(SKIPPED)) !== r.version) setRelease(r);
    })().catch(() => {}); // no network, no release: try again next launch
  }, [current]);

  if (!release) return null;

  const install = async () => {
    setPhase('downloading');
    try {
      const file = new File(Paths.cache, 'kinora-update.apk');
      if (file.exists) file.delete();
      const apk = await File.downloadFileAsync(release.apk, file);
      setPhase('ask');
      await startActivityAsync('android.intent.action.VIEW', { data: apk.contentUri, flags: GRANT_READ_URI_PERMISSION, type: 'application/vnd.android.package-archive' });
    } catch {
      setPhase('failed');
    }
  };
  const skip = async () => {
    await setItem(SKIPPED, release.version);
    setRelease(null);
  };

  return (
    // An overlay rather than a Modal: a Modal is a separate Android window, the TV focus stayed on the screen behind it.
    // The focus guide keeps the remote inside the box; the layout hides the app meanwhile (onShow), or the page loading
    // behind would take the focus.
    <View style={styles.backdrop}>
      <Box style={styles.box} trapFocusUp trapFocusDown trapFocusLeft trapFocusRight autoFocus>
          <Text style={ui.h2}>{t('update.title', { version: release.version })}</Text>
          <Text style={ui.muted}>{t('update.current', { version: current })}</Text>
          {!!release.notes && (
            <ScrollView style={styles.notes} contentContainerStyle={{ padding: 16 }}>
              <Text style={ui.text}>{release.notes.trim()}</Text>
            </ScrollView>
          )}
          {phase === 'downloading' ? (
            <View style={styles.row}>
              <Spinner />
              <Text style={ui.text}>{t('update.downloading')}</Text>
            </View>
          ) : (
            <>
              {phase === 'failed' && <Text style={ui.error}>{t('update.failed')}</Text>}
              {/* Up goes nowhere: above are only the notes, the focus would vanish there. */}
              <Box style={styles.row} trapFocusUp>
                <Button kind="red" label={phase === 'failed' ? t('update.retry') : t('update.install')} onPress={install} hasTVPreferredFocus={focusReady} />
                <Button kind="grey" label={t('update.later')} onPress={() => setRelease(null)} />
                <Button kind="outline" label={t('update.skip')} onPress={skip} />
              </Box>
            </>
          )}
      </Box>
    </View>
  );
}

const Box = Platform.isTV ? TVFocusGuideView : View;

const styles = StyleSheet.create({
  backdrop: { ...StyleSheet.absoluteFill, zIndex: 2000, backgroundColor: 'rgba(0,0,0,0.75)', alignItems: 'center', justifyContent: 'center', padding: 24 },
  box: { width: '100%', maxWidth: 720, maxHeight: '90%', backgroundColor: '#181818', borderRadius: 8, borderWidth: 1, borderColor: '#333', padding: 32, gap: 16 },
  notes: { maxHeight: 260, backgroundColor: colors.bg, borderRadius: 6 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 12, flexWrap: 'wrap' },
});
