import { useQuery, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState, type ReactNode } from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';
import { api, setCurrentProfile, unwrap, type User } from '../../api/client';
import { Page } from '../../components/Browse';
import { Button, Chip, Field, Spinner, styles as ui } from '../../components/ui';
import { useMe } from '../../lib/auth';
import { colors } from '../../theme';
import { setLanguage, t, useLanguage, type LangCode } from '../../i18n';

export default function Account() {
  const me = useMe();
  const queryClient = useQueryClient();
  if (!me.data) return <Spinner full />;

  const logout = async () => {
    await api.POST('/auth/logout');
    setCurrentProfile(null);
    queryClient.clear();
    router.replace('/login');
  };

  return (
    <Page>
      <View style={{ maxWidth: 900, gap: 24 }}>
        <Text style={[ui.h1, { marginBottom: 0 }]}>{t('account.title')}</Text>
        <View style={styles.head}>
          <Text style={ui.muted}>
            {t('account.signedInAs')}
            <Text style={{ color: '#fff', fontWeight: '700' }}>{me.data.username}</Text>
            {me.data.isAdmin && t('account.admin')}
          </Text>
          <View style={{ flexDirection: 'row', gap: 12 }}>
            {Platform.OS === 'web' && <Button kind="outline" small label={t('link.title')} href="/link" />}
            <Button kind="outline" small label={t('common.logout')} onPress={logout} />
          </View>
        </View>
        <PasswordForm />
        {me.data.isAdmin && (
          <>
            <InstanceLanguage />
            <FlareSolverr />
            <Users me={me.data} />
          </>
        )}
      </View>
    </Page>
  );
}

function Panel({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <View style={styles.panel}>
      <View style={styles.head}>
        <Text style={ui.h2}>{title}</Text>
        {action}
      </View>
      {children}
    </View>
  );
}

function useAction() {
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const run = async (fn: () => Promise<unknown>, ok?: string) => {
    try {
      await fn();
      setMsg(ok ? { ok: true, text: ok } : null);
      return true;
    } catch (err) {
      setMsg({ ok: false, text: err instanceof Error ? err.message : String(err) });
      return false;
    }
  };
  const view = msg && <Text style={msg.ok ? ui.success : ui.error}>{msg.text}</Text>;
  return { run, view };
}

function PasswordForm() {
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const { run, view } = useAction();
  const submit = async () => {
    if (await run(() => unwrap(api.PUT('/me/password', { body: { current, new: next } })))) router.replace('/login');
  };
  return (
    <Panel title={t('account.password')}>
      <View style={styles.form}>
        <Field style={styles.input} placeholder={t('account.currentPassword')} value={current} onChangeText={setCurrent} secureTextEntry autoComplete="current-password" />
        <Field style={styles.input} placeholder={t('account.newPassword')} value={next} onChangeText={setNext} secureTextEntry autoComplete="new-password" />
        <Button small label={t('account.change')} onPress={submit} />
      </View>
      {view}
    </Panel>
  );
}

function InstanceLanguage() {
  const queryClient = useQueryClient();
  const language = useLanguage();
  const instance = useQuery({ queryKey: ['instance'], queryFn: () => unwrap(api.GET('/instance')) });
  const { run, view } = useAction();

  const change = (code: LangCode) =>
    run(async () => {
      await unwrap(api.PUT('/admin/instance', { body: { language: code } }));
      setLanguage(code);
      // Every cached catalog page is in the old language.
      await queryClient.invalidateQueries();
    }, t('account.languageChanged'));

  return (
    <Panel title={t('account.language')}>
      <Text style={ui.muted}>{t('account.languageHint')}</Text>
      <View style={styles.form}>
        {instance.data?.languages.map((l) => (
          <Chip key={l.code} label={l.label} selected={l.code === language} onPress={() => change(l.code as LangCode)} />
        ))}
      </View>
      {view}
    </Panel>
  );
}

function FlareSolverr() {
  const current = useQuery({ queryKey: ['admin', 'flaresolverr'], queryFn: () => unwrap(api.GET('/admin/flaresolverr')) });
  const [url, setUrl] = useState<string | null>(null);
  const { run, view } = useAction();
  const value = url ?? current.data?.url ?? '';

  const save = (next: string) =>
    run(async () => {
      await unwrap(api.PUT('/admin/flaresolverr', { body: { url: next } }));
      setUrl(next);
    }, next ? t('account.flaresolverrOn') : t('account.flaresolverrOff'));

  return (
    <Panel title="FlareSolverr">
      <Text style={ui.muted}>{t('account.flaresolverrHint')}</Text>
      <View style={styles.form}>
        <Field style={styles.input} placeholder="http://flaresolverr:8191" value={value} onChangeText={setUrl} autoCapitalize="none" autoCorrect={false} />
        <Button small label={t('common.save')} onPress={() => save(value.trim())} />
        {!!current.data?.url && <Button kind="outline" small label={t('account.disable')} onPress={() => save('')} />}
      </View>
      {view}
    </Panel>
  );
}

function Users({ me }: { me: User }) {
  const queryClient = useQueryClient();
  const users = useQuery({ queryKey: ['admin', 'users'], queryFn: () => unwrap(api.GET('/admin/users')) });
  const [form, setForm] = useState({ username: '', password: '', isAdmin: false });
  const [confirming, setConfirming] = useState<number | null>(null);
  const { run, view } = useAction();
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });

  const create = async () => {
    if (await run(() => unwrap(api.POST('/admin/users', { body: form })), t('account.created', { name: form.username }))) {
      setForm({ username: '', password: '', isAdmin: false });
      refresh();
    }
  };

  return (
    <Panel title={t('account.accounts')}>
      <Text style={ui.muted}>{t('account.accountsHint')}</Text>
      {users.data?.map((u) => (
        <View key={u.id} style={styles.line}>
          <Text style={[ui.text, { flex: 1 }]}>
            {u.username}
            {u.isAdmin && <Text style={ui.muted}>{t('account.adminBadge')}</Text>}
          </Text>
          {u.id !== me.id && (
            <Button
              kind="danger"
              small
              label={confirming === u.id ? t('account.confirm') : t('account.delete')}
              onPress={() =>
                confirming === u.id
                  ? run(() => unwrap(api.DELETE('/admin/users/{id}', { params: { path: { id: u.id } } }))).then(refresh)
                  : setConfirming(u.id)
              }
            />
          )}
        </View>
      ))}
      <View style={styles.form}>
        <Field style={styles.input} placeholder={t('auth.username')} value={form.username} onChangeText={(username) => setForm({ ...form, username })} autoCapitalize="none" />
        <Field style={styles.input} placeholder={t('auth.password')} value={form.password} onChangeText={(password) => setForm({ ...form, password })} secureTextEntry autoComplete="new-password" />
        <Chip label={t('account.adminToggle')} selected={form.isAdmin} onPress={() => setForm({ ...form, isAdmin: !form.isAdmin })} />
        <Button small label={t('account.create')} onPress={create} />
      </View>
      {view}
    </Panel>
  );
}

const styles = StyleSheet.create({
  head: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' },
  panel: { padding: 24, backgroundColor: colors.bg2, borderRadius: 6, gap: 12 },
  form: { flexDirection: 'row', flexWrap: 'wrap', gap: 10, alignItems: 'center' },
  input: { flex: 1, minWidth: 180, paddingVertical: 8 },
  line: { flexDirection: 'row', alignItems: 'center', gap: 16, paddingVertical: 10, borderBottomWidth: 1, borderBottomColor: '#2a2a2a', flexWrap: 'wrap' },
});
