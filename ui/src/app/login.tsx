import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Redirect, router } from 'expo-router';
import { useCallback, useState } from 'react';
import { Platform, Text } from 'react-native';
import { api, needsServer, serverOrigin, setCurrentProfile, unwrap, type User } from '../api/client';
import { AuthForm, AuthShell } from '../components/AuthForm';
import { DeviceLogin } from '../components/DeviceLogin';
import { Button, Spinner, styles as ui } from '../components/ui';
import { t } from '../i18n';

export default function Login() {
  const queryClient = useQueryClient();
  const instance = useQuery({ queryKey: ['instance'], queryFn: () => unwrap(api.GET('/instance')), enabled: !needsServer() });
  // TV: a code to enter from a computer first, typing a password with the remote is the fallback.
  const [password, setPassword] = useState(!Platform.isTV);
  const signedIn = useCallback(
    (user: User) => {
      setCurrentProfile(null);
      // A session that expired lands here without a logout: drop what the previous account cached (its profiles...).
      queryClient.clear();
      queryClient.setQueryData(['me'], user);
      router.replace('/profiles');
    },
    [queryClient],
  );
  if (needsServer()) return <Redirect href="/server" />;
  if (instance.isPending) return <Spinner full />;
  if (instance.data?.setupNeeded) return <Redirect href="/setup" />;
  // TV and phone apps: the saved server may be gone or have moved.
  const changeServer = Platform.OS !== 'web' && <Button kind="outline" small label={t('server.change')} onPress={() => router.push('/server')} />;
  if (instance.isError) {
    return (
      <AuthShell title={t('server.title')}>
        <Text style={ui.error}>{t('server.unreachable')}</Text>
        <Text style={ui.muted}>{serverOrigin()}</Text>
        {changeServer}
      </AuthShell>
    );
  }

  if (!password) {
    return (
      <DeviceLogin onSignedIn={signedIn}>
        <Button kind="grey" label={t('auth.usePassword')} onPress={() => setPassword(true)} hasTVPreferredFocus />
        {changeServer}
      </DeviceLogin>
    );
  }
  return (
    <AuthForm
      title={t('auth.signIn')}
      cta={t('auth.signIn')}
      existingAccount
      extra={
        <>
          {Platform.isTV && <Button kind="outline" small label={t('auth.useCode')} onPress={() => setPassword(false)} />}
          {changeServer}
        </>
      }
      submit={async (username, pass) => signedIn(await unwrap(api.POST('/auth/login', { body: { username, password: pass } })))}
    />
  );
}
