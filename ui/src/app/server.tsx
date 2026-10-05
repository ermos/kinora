import { useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';
import { Text } from 'react-native';
import { normalizeServer, serverOrigin, setServer } from '../api/client';
import { AuthShell } from '../components/AuthForm';
import { Button, Field, styles as ui } from '../components/ui';
import { setLanguage, t } from '../i18n';

/** TV and phone apps: the address of the kinora instance, asked on first launch and kept on the device. */
export default function Server() {
  const queryClient = useQueryClient();
  const [url, setUrl] = useState(serverOrigin() || 'http://');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const connect = async () => {
    const server = normalizeServer(url);
    if (!server) return;
    setBusy(true);
    setError('');
    try {
      // Checked before saving: the instance endpoint is public and only a kinora server answers it this way.
      const abort = new AbortController();
      const timer = setTimeout(() => abort.abort(), 8000);
      const res = await fetch(`${server}/api/v1/instance`, { signal: abort.signal }).finally(() => clearTimeout(timer));
      const info = res.ok ? await res.json() : null;
      if (!info?.languages) throw new Error();
      setServer(server);
      setLanguage(info.language);
      queryClient.clear(); // everything cached came from the previous server
      router.replace('/');
    } catch {
      setError(t('server.unreachable'));
      setBusy(false);
    }
  };

  return (
    <AuthShell title={t('server.title')} hint={t('server.hint')}>
      <Field
        label={t('server.address')}
        value={url}
        onChangeText={setUrl}
        autoCapitalize="none"
        autoCorrect={false}
        keyboardType="url"
        autoFocus
        onSubmitEditing={connect}
      />
      {!!error && <Text style={ui.error}>{error}</Text>}
      <Button kind="red" label={busy ? '…' : t('server.connect')} onPress={connect} disabled={busy} />
    </AuthShell>
  );
}
