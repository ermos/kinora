import { useState } from 'react';
import { Text } from 'react-native';
import { api, unwrap } from '../api/client';
import { AuthShell } from '../components/AuthForm';
import { Button, Field, styles as ui } from '../components/ui';
import { Gate } from '../lib/auth';
import { t } from '../i18n';

/** Signs a TV in to the current account with the code it shows (see DeviceLogin). */
export default function LinkScreen() {
  return (
    <Gate needsProfile={false}>
      <Link />
    </Gate>
  );
}

function Link() {
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  const connect = async () => {
    if (!code.trim()) return;
    setBusy(true);
    setError('');
    try {
      await unwrap(api.POST('/auth/device/approve', { body: { code } }));
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
    setBusy(false);
  };

  return (
    <AuthShell title={t('link.title')} hint={done ? undefined : t('link.hint')}>
      {done ? (
        <Text style={ui.text}>{t('link.done')}</Text>
      ) : (
        <>
          <Field label={t('link.code')} value={code} onChangeText={setCode} autoCapitalize="characters" autoCorrect={false} autoFocus placeholder="ABCD-EFGH" onSubmitEditing={connect} />
          {!!error && <Text style={ui.error}>{error}</Text>}
          <Button kind="red" label={busy ? '…' : t('link.connect')} onPress={connect} disabled={busy} />
        </>
      )}
    </AuthShell>
  );
}
