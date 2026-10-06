import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { router, Stack } from 'expo-router';
import { useEffect, useState } from 'react';
import { View } from 'react-native';
import { api, authEvents, loadProfile, loadServer, needsServer } from '../api/client';
import { TypingPreview } from '../components/TypingPreview';
import { UpdatePrompt } from '../components/UpdatePrompt';
import { Spinner } from '../components/ui';
import { setLanguage } from '../i18n';
import { colors } from '../theme';

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 5 * 60_000, retry: 1, refetchOnWindowFocus: false } },
});

authEvents.onUnauthorized = () => {
  queryClient.removeQueries({ queryKey: ['me'] }); // or /login, seeing a cached account, would send back home
  router.replace('/login');
};
authEvents.onUnknownProfile = () => router.replace('/profiles');

export default function RootLayout() {
  // Before the first screen: the server address and selected profile (async storage on native), the instance language.
  const [ready, setReady] = useState(false);
  const [updating, setUpdating] = useState(false);
  useEffect(() => {
    // TV and phone apps first need the saved server address.
    const instance = loadServer().then(async () => {
      if (needsServer()) return;
      const { data } = await api.GET('/instance');
      if (data) setLanguage(data.language);
    });
    Promise.allSettled([loadProfile(), instance]).finally(() => setReady(true));
  }, []);

  return (
    <QueryClientProvider client={queryClient}>
      {ready ? (
        // Hidden, not unmounted, while the update prompt is up: hidden views cannot take the TV focus from it.
        <View style={{ flex: 1, display: updating ? 'none' : 'flex' }}>
          <Stack screenOptions={{ headerShown: false, contentStyle: { backgroundColor: colors.bg }, animation: 'fade' }} />
        </View>
      ) : (
        <Spinner full />
      )}
      <TypingPreview />
      {ready && <UpdatePrompt onShow={setUpdating} />}
    </QueryClientProvider>
  );
}
