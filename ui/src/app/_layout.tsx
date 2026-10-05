import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { router, Stack } from 'expo-router';
import { useEffect, useState } from 'react';
import { api, authEvents, loadProfile } from '../api/client';
import { Spinner } from '../components/ui';
import { setLanguage } from '../i18n';
import { colors } from '../theme';

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 5 * 60_000, retry: 1, refetchOnWindowFocus: false } },
});

authEvents.onUnauthorized = () => router.replace('/login');
authEvents.onUnknownProfile = () => router.replace('/profiles');

export default function RootLayout() {
  // Before the first screen: the selected profile (async storage on native) and the instance language.
  const [ready, setReady] = useState(false);
  useEffect(() => {
    const instance = api.GET('/instance').then(({ data }) => data && setLanguage(data.language));
    Promise.allSettled([loadProfile(), instance]).finally(() => setReady(true));
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      {ready ? (
        <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg }, animation: 'fade' }} />
      ) : (
        <Spinner full />
      )}
    </QueryClientProvider>
  );
}
