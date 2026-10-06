import { useEffect, useState, type ReactNode } from 'react';
import { StyleSheet, Text } from 'react-native';
import { api, ApiError, serverOrigin, unwrap, type User } from '../api/client';
import { t } from '../i18n';
import { AuthShell } from './AuthForm';
import { Spinner, styles as ui } from './ui';

const POLL = 3000;

/**
 * TV sign-in without typing: shows a code to enter on /link from a signed-in computer or phone, and polls until it
 * is approved. A new code replaces it when it expires, so the code shown keeps rotating.
 */
export function DeviceLogin({ onSignedIn, children }: { onSignedIn: (user: User) => void; children?: ReactNode }) {
  const [device, setDevice] = useState<{ code: string; token: string; expiresIn: number } | null>(null);
  const [round, setRound] = useState(0); // bumped to ask for a new code

  useEffect(() => {
    let cancelled = false;
    setDevice(null);
    unwrap(api.POST('/auth/device')).then(
      (d) => !cancelled && setDevice(d),
      () => !cancelled && setTimeout(() => setRound((r) => r + 1), POLL), // server unreachable: retry
    );
    return () => {
      cancelled = true;
    };
  }, [round]);

  useEffect(() => {
    if (!device) return;
    const renew = setTimeout(() => setRound((r) => r + 1), device.expiresIn * 1000);
    const poll = setInterval(async () => {
      try {
        const res = await unwrap(api.POST('/auth/device/poll', { body: { token: device.token } }));
        if (res.approved && res.user) {
          clearInterval(poll);
          onSignedIn(res.user);
        }
      } catch (err) {
        if (err instanceof ApiError && err.status === 410) setRound((r) => r + 1);
      }
    }, POLL);
    return () => {
      clearTimeout(renew);
      clearInterval(poll);
    };
  }, [device, onSignedIn]);

  return (
    <AuthShell title={t('auth.signIn')}>
      <Text style={ui.text}>{t('auth.deviceHint', { url: `${serverOrigin()}/link` })}</Text>
      {device ? <Text style={styles.code}>{device.code}</Text> : <Spinner />}
      <Text style={ui.muted}>{t('auth.deviceRotates')}</Text>
      {children}
    </AuthShell>
  );
}

const styles = StyleSheet.create({
  code: { color: '#fff', fontSize: 56, fontWeight: '900', letterSpacing: 6, textAlign: 'center', fontVariant: ['tabular-nums'], marginVertical: 8 },
});
