import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Redirect, router } from 'expo-router';
import { Platform, Text } from 'react-native';
import { api, needsServer, serverOrigin, setCurrentProfile, unwrap } from '../api/client';
import { AuthForm, AuthShell } from '../components/AuthForm';
import { Button, Spinner, styles as ui } from '../components/ui';
import { t } from '../i18n';

export default function Login() {
  const queryClient = useQueryClient();
  const instance = useQuery({ queryKey: ['instance'], queryFn: () => unwrap(api.GET('/instance')), enabled: !needsServer() });
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

  return (
    <AuthForm
      title={t('auth.signIn')}
      cta={t('auth.signIn')}
      existingAccount
      extra={changeServer}
      submit={async (username, password) => {
        const user = await unwrap(api.POST('/auth/login', { body: { username, password } }));
        setCurrentProfile(null);
        // A session that expired lands here without a logout: drop what the previous account cached (its profiles...).
        queryClient.clear();
        queryClient.setQueryData(['me'], user);
        router.replace('/profiles');
      }}
    />
  );
}
