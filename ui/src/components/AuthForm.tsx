import { LinearGradient } from 'expo-linear-gradient';
import { useState, type ReactNode } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { t } from '../i18n';
import { colors } from '../theme';
import { Button, Field, styles as ui } from './ui';

export function Logo({ size = 34 }: { size?: number }) {
  return <Text style={[styles.logo, { fontSize: size }]}>KINORA</Text>;
}

export function AuthForm({
  title,
  hint,
  cta,
  extra,
  existingAccount,
  submit,
}: {
  title: string;
  hint?: string;
  cta: string;
  extra?: ReactNode;
  existingAccount?: boolean;
  submit: (u: string, p: string) => Promise<void>;
}) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  const onSubmit = async () => {
    if (!username || !password) return;
    setBusy(true);
    setError('');
    try {
      await submit(username, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  };

  return (
    <AuthShell title={title} hint={hint}>
      <Field label={t('auth.username')} value={username} onChangeText={setUsername} autoCapitalize="none" autoComplete="username" autoFocus onSubmitEditing={onSubmit} />
      <Field
        label={t('auth.password')}
        value={password}
        onChangeText={setPassword}
        secureTextEntry
        autoComplete={existingAccount ? 'current-password' : 'new-password'}
        onSubmitEditing={onSubmit}
      />
      {extra}
      {!!error && <Text style={ui.error}>{error}</Text>}
      <Button kind="red" label={busy ? '…' : cta} onPress={onSubmit} disabled={busy} />
    </AuthShell>
  );
}

/** Backdrop, logo and card shared by the sign-in, setup and server screens. */
export function AuthShell({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <View style={{ flex: 1, backgroundColor: '#000' }}>
      <LinearGradient colors={['#3a0a0d', '#000']} locations={[0, 0.7]} style={StyleSheet.absoluteFill} />
      <ScrollView contentContainerStyle={styles.page} keyboardShouldPersistTaps="handled">
        <Logo />
        <View style={styles.card}>
          <Text style={styles.title}>{title}</Text>
          {hint && <Text style={ui.muted}>{hint}</Text>}
          {children}
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  page: { padding: 24, minHeight: '100%' },
  logo: { color: colors.red, fontWeight: '900', letterSpacing: -0.5, transform: [{ scaleY: 1.15 }] },
  card: { width: '100%', maxWidth: 450, alignSelf: 'center', marginTop: 48, padding: 40, backgroundColor: 'rgba(0,0,0,0.75)', borderRadius: 6, gap: 16 },
  title: { color: '#fff', fontSize: 32, fontWeight: '700', marginBottom: 8 },
});
