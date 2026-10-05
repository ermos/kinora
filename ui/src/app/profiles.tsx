import { useQuery, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';
import { useState } from 'react';
import { Modal, ScrollView, StyleSheet, Text, View } from 'react-native';
import { api, setCurrentProfile, unwrap, type Profile } from '../api/client';
import { Logo } from '../components/AuthForm';
import { Focusable } from '../components/Focusable';
import { Avatar, Button, Chip, Field, Icon, icons, Spinner, styles as ui } from '../components/ui';
import { Gate } from '../lib/auth';
import { AVATARS, colors, useLayout } from '../theme';
import { t } from '../i18n';

const MAX_PROFILES = 5;

export default function ProfilesScreen() {
  return (
    <Gate needsProfile={false}>
      <Profiles />
    </Gate>
  );
}

function Profiles() {
  const queryClient = useQueryClient();
  const { phone } = useLayout();
  const profiles = useQuery({ queryKey: ['profiles'], queryFn: () => unwrap(api.GET('/profiles')) });
  const [managing, setManaging] = useState(false);
  const [editing, setEditing] = useState<Profile | 'new' | null>(null);

  if (profiles.isPending) return <Spinner full />;
  const list = profiles.data ?? [];
  const size = phone ? 96 : 140;

  const pick = (p: Profile) => {
    if (managing) return setEditing(p);
    setCurrentProfile(p);
    queryClient.removeQueries({ queryKey: ['library'] });
    router.replace('/');
  };

  return (
    <ScrollView style={ui.page} contentContainerStyle={styles.page}>
      <View style={styles.logo}>
        <Logo />
      </View>
      <Text style={styles.title}>{managing ? t('profiles.manage') : t('profiles.whoIsWatching')}</Text>
      <View style={styles.grid}>
        {list.map((p, i) => (
          <Focusable key={p.id} onPress={() => pick(p)} hasTVPreferredFocus={i === 0} style={styles.tile}>
            {(active) => (
              <>
                <View style={[styles.avatar, active && styles.avatarActive]}>
                  <Avatar color={p.avatar} size={size} />
                  {managing && (
                    <View style={styles.editOverlay}>
                      <Icon d={icons.edit} size={40} />
                    </View>
                  )}
                </View>
                <Text style={[styles.name, active && { color: '#fff' }]}>{p.name}</Text>
              </>
            )}
          </Focusable>
        ))}
        {list.length < MAX_PROFILES && (
          <Focusable onPress={() => setEditing('new')} style={styles.tile}>
            {(active) => (
              <>
                <View style={[styles.avatar, styles.add, { width: size, height: size }, active && styles.avatarActive]}>
                  <Icon d={icons.plus} size={56} color={active ? '#fff' : colors.muted} />
                </View>
                <Text style={[styles.name, active && { color: '#fff' }]}>{t('profiles.add')}</Text>
              </>
            )}
          </Focusable>
        )}
      </View>
      <Button kind={managing ? 'white' : 'outline'} label={managing ? t('profiles.done') : t('profiles.manage')} onPress={() => setManaging(!managing)} />
      {editing && (
        <ProfileEditor
          profile={editing === 'new' ? null : editing}
          canDelete={list.length > 1}
          onClose={() => {
            setEditing(null);
            queryClient.invalidateQueries({ queryKey: ['profiles'] });
          }}
        />
      )}
    </ScrollView>
  );
}

function ProfileEditor({ profile, canDelete, onClose }: { profile: Profile | null; canDelete: boolean; onClose: () => void }) {
  const [name, setName] = useState(profile?.name ?? '');
  const [avatar, setAvatar] = useState(profile?.avatar ?? Object.keys(AVATARS)[Math.floor(Math.random() * 8)]);
  const [skipSegments, setSkipSegments] = useState(profile?.skipSegments ?? true);
  const [error, setError] = useState('');
  const [confirmDelete, setConfirmDelete] = useState(false);

  const run = async (fn: () => Promise<unknown>) => {
    try {
      await fn();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const save = () =>
    run(() =>
      profile
        ? unwrap(api.PATCH('/profiles/{id}', { params: { path: { id: profile.id } }, body: { name, avatar, skipSegments } }))
        : unwrap(api.POST('/profiles', { body: { name, avatar, skipSegments } })),
    );

  return (
    <Modal transparent animationType="fade" onRequestClose={onClose}>
      <View style={styles.backdrop}>
        <View style={styles.modal}>
          <Text style={styles.modalTitle}>{profile ? t('profiles.edit') : t('profiles.add')}</Text>
          <View style={styles.editorBody}>
            <Avatar color={avatar} size={110} />
            <View style={{ flex: 1, minWidth: 220, gap: 16 }}>
              <Field value={name} onChangeText={setName} placeholder={t('profiles.name')} maxLength={32} autoFocus onSubmitEditing={save} />
              <View style={styles.picker}>
                {Object.keys(AVATARS).map((c) => (
                  <Focusable key={c} onPress={() => setAvatar(c)} accessibilityLabel={c} style={(active) => [styles.pick, (c === avatar || active) && styles.pickSelected]}>
                    <Avatar color={c} size={44} />
                  </Focusable>
                ))}
              </View>
              <Chip label={t('profiles.skipSegments')} selected={skipSegments} onPress={() => setSkipSegments(!skipSegments)} />
            </View>
          </View>
          {!!error && <Text style={ui.error}>{error}</Text>}
          <View style={styles.actions}>
            <Button label={t('common.save')} onPress={save} />
            <Button kind="outline" label={t('common.cancel')} onPress={onClose} />
            {profile && canDelete && (
              <Button
                kind="danger"
                label={confirmDelete ? t('profiles.confirmDelete') : t('profiles.delete')}
                onPress={() => (confirmDelete ? run(() => unwrap(api.DELETE('/profiles/{id}', { params: { path: { id: profile.id } } }))) : setConfirmDelete(true))}
              />
            )}
          </View>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  page: { flexGrow: 1, alignItems: 'center', justifyContent: 'center', gap: 32, padding: 24, paddingTop: 96 },
  logo: { position: 'absolute', top: 24, left: 24 },
  title: { color: '#fff', fontSize: 48, textAlign: 'center' },
  grid: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'center', gap: 24 },
  tile: { alignItems: 'center', gap: 12 },
  avatar: { borderRadius: 4, borderWidth: 3, borderColor: 'transparent', overflow: 'hidden' },
  avatarActive: { borderColor: '#fff' },
  add: { alignItems: 'center', justifyContent: 'center', borderColor: '#333' },
  editOverlay: { ...StyleSheet.absoluteFill, backgroundColor: 'rgba(0,0,0,0.5)', alignItems: 'center', justifyContent: 'center' },
  name: { color: colors.muted, fontSize: 18 },
  backdrop: { flex: 1, backgroundColor: 'rgba(0,0,0,0.8)', alignItems: 'center', justifyContent: 'center', padding: 16 },
  modal: { backgroundColor: colors.bg, borderRadius: 6, padding: 32, width: '100%', maxWidth: 640, gap: 16 },
  modalTitle: { color: '#fff', fontSize: 30, paddingBottom: 16, borderBottomWidth: 1, borderBottomColor: '#333' },
  editorBody: { flexDirection: 'row', flexWrap: 'wrap', gap: 24, paddingBottom: 24, borderBottomWidth: 1, borderBottomColor: '#333' },
  picker: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  pick: { borderWidth: 2, borderColor: 'transparent', borderRadius: 6 },
  pickSelected: { borderColor: '#fff' },
  actions: { flexDirection: 'row', flexWrap: 'wrap', gap: 12 },
});
