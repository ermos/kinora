import { useQuery, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { Text, View } from 'react-native';
import { api, unwrap } from '../api/client';
import { AuthForm } from '../components/AuthForm';
import { Chip, Spinner, styles as ui } from '../components/ui';
import { setLanguage, t, useLanguage, type LangCode } from '../i18n';

export default function Setup() {
  const queryClient = useQueryClient();
  const language = useLanguage();
  const instance = useQuery({ queryKey: ['instance'], queryFn: () => unwrap(api.GET('/instance')) });
  if (instance.isPending) return <Spinner full />;
  const languages = instance.data?.languages ?? [];

  return (
    <AuthForm
      title={t('auth.welcome')}
      hint={t('auth.setupHint')}
      cta={t('auth.createAccount')}
      extra={
        // Only the languages the server supports end to end (sources, catalog, UI).
        <View style={{ gap: 8 }}>
          <Text style={ui.muted}>{t('auth.language')}</Text>
          <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 8 }}>
            {languages.map((l) => (
              <Chip key={l.code} label={l.label} selected={l.code === language} onPress={() => setLanguage(l.code as LangCode)} />
            ))}
          </View>
          <Text style={[ui.muted, { fontSize: 12 }]}>{t('auth.languageHint')}</Text>
        </View>
      }
      submit={async (username, password) => {
        const user = await unwrap(api.POST('/setup', { body: { username, password, language } }));
        queryClient.setQueryData(['me'], user);
        queryClient.invalidateQueries({ queryKey: ['instance'] });
        router.replace('/profiles');
      }}
    />
  );
}
