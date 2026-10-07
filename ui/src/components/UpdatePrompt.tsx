import Constants from 'expo-constants';
import { File, Paths } from 'expo-file-system';
import { ActivityAction, startActivityAsync } from 'expo-intent-launcher';
import { useEffect, useState } from 'react';
import { BackHandler, Platform, ScrollView, StyleSheet, Text, TVFocusGuideView, View } from 'react-native';
import { absolute, api, useProfile } from '../api/client';
import { getItem, setItem } from '../lib/storage';
import { newer } from '../lib/version';
import { t } from '../i18n';
import { colors } from '../theme';
import { Button, Spinner, styles as ui } from './ui';

const SKIPPED = 'update.skipped';
const GRANT_READ_URI_PERMISSION = 1;
const PACKAGE = 'com.ermos.kinora';

export type Release = { version: string; notes: string; apk: string };

let show: (r: Release) => void = () => {};
/** Opens the prompt for a release, even a skipped one: the admin asked for it (account page). */
export const offerUpdate = (r: Release) => show(r);

/** The server picks the release: GitHub's latest, or the local build in development (APK served by the server). */
async function latestRelease(): Promise<Release | null> {
  const { data, response } = await api.GET('/update');
  return response.status === 200 && data ? data : null;
}

/**
 * Android: on launch, offers the latest release when it is newer than this build and not skipped. Updating downloads
 * the APK and opens the system installer (it asks to confirm, and the first time to allow installs from kinora).
 */
export function UpdatePrompt({ onShow }: { onShow?: (shown: boolean) => void }) {
  const current = Constants.expoConfig?.version ?? '0';
  const signedIn = !!useProfile(); // /update needs a session: checked once a profile is picked
  const [release, setRelease] = useState<Release | null>(null);
  const [phase, setPhase] = useState<'ask' | 'downloading' | 'failed'>('ask');
  useEffect(() => {
    show = setRelease;
  }, []);
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
    if (Platform.OS !== 'android' || !signedIn) return;
    (async () => {
      const r = await latestRelease();
      if (r && newer(r.version, current) && (await getItem(SKIPPED)) !== r.version) setRelease(r);
    })().catch(() => {}); // no network, no release: try again next launch
  }, [current, signedIn]);

  if (!release) return null;

  const install = async () => {
    setPhase('downloading');
    try {
      const file = new File(Paths.cache, 'kinora-update.apk');
      if (file.exists) file.delete();
      const apk = await File.downloadFileAsync(absolute(release.apk), file);
      setPhase('ask');
      await startActivityAsync('android.intent.action.VIEW', { data: apk.contentUri, flags: GRANT_READ_URI_PERMISSION, type: 'application/vnd.android.package-archive' });
    } catch {
      setPhase('failed');
    }
  };
  // Android asks, the first time, to allow kinora to install apps; its own dialog only leads to a generic settings page.
  const allowInstalls = () => startActivityAsync(ActivityAction.MANAGE_UNKNOWN_APP_SOURCES, { data: `package:${PACKAGE}` }).catch(() => {});
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
              <Text style={ui.muted}>{t('update.permission')}</Text>
              {/* Up goes nowhere: above are only the notes, the focus would vanish there. */}
              <Box style={styles.row} trapFocusUp>
                <Button kind="red" label={phase === 'failed' ? t('update.retry') : t('update.install')} onPress={install} hasTVPreferredFocus={focusReady} />
                <Button kind="grey" label={t('update.later')} onPress={() => setRelease(null)} />
                <Button kind="outline" label={t('update.skip')} onPress={skip} />
                <Button kind="outline" label={t('update.allow')} onPress={allowInstalls} />
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
