import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Redirect, router } from 'expo-router';
import { api, setCurrentProfile, unwrap } from '../api/client';
import { AuthForm } from '../components/AuthForm';
import { Spinner } from '../components/ui';
import { t } from '../i18n';

export default function Login() {
  const queryClient = useQueryClient();
  const instance = useQuery({ queryKey: ['instance'], queryFn: () => unwrap(api.GET('/instance')) });
  if (instance.isPending) return <Spinner full />;
  if (instance.data?.setupNeeded) return <Redirect href="/setup" />;

  return (
    <AuthForm
      title={t('auth.signIn')}
      cta={t('auth.signIn')}
      existingAccount
      submit={async (username, password) => {
        const user = await unwrap(api.POST('/auth/login', { body: { username, password } }));
        setCurrentProfile(null);
        queryClient.setQueryData(['me'], user);
        router.replace('/profiles');
      }}
    />
  );
}
