import { useQueryClient } from '@tanstack/react-query';
import { LinearGradient } from 'expo-linear-gradient';
import { router, usePathname, type Href } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Animated, StyleSheet, Text, View } from 'react-native';
import { api, setCurrentProfile, useProfile } from '../api/client';
import { colors, useLayout } from '../theme';
import { Focusable } from './Focusable';
import { Avatar, Icon, icons } from './ui';
import { t, type TKey } from '../i18n';

const MAIN: { href: Href; label: TKey; icon: string }[] = [
  { href: '/search', label: 'nav.search', icon: icons.search },
  { href: '/', label: 'nav.home', icon: icons.home },
  { href: '/tv', label: 'nav.shows', icon: icons.tv },
  { href: '/movies', label: 'nav.movies', icon: icons.movie },
  { href: '/list', label: 'nav.myList', icon: icons.plus },
];

const EXPANDED = 260;

/**
 * Netflix TV style navigation rail. It expands when any item has focus (remote / keyboard) or under the mouse,
 * so it behaves the same on a TV and in a browser. Phones get a bottom tab bar instead.
 */
export function Sidebar() {
  const { rail, phone } = useLayout();
  const profile = useProfile();
  const pathname = usePathname();
  const queryClient = useQueryClient();
  const [focused, setFocused] = useState(false);
  const [hovered, setHovered] = useState(false);
  const blurTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const open = !phone && (focused || hovered);
  const width = useRef(new Animated.Value(rail)).current;

  useEffect(() => {
    Animated.timing(width, { toValue: open ? EXPANDED : rail, duration: 200, useNativeDriver: false }).start();
  }, [open, rail, width]);

  // Focus moves item to item: a blur immediately followed by a focus must not close the rail.
  const focusProps = {
    onFocus: () => {
      clearTimeout(blurTimer.current);
      setFocused(true);
    },
    onBlur: () => {
      blurTimer.current = setTimeout(() => setFocused(false), 50);
    },
  };

  const logout = async () => {
    await api.POST('/auth/logout');
    setCurrentProfile(null);
    queryClient.clear();
    router.replace('/login');
  };

  const isActive = (href: Href) => (href === '/' ? pathname === '/' : pathname.startsWith(String(href)));

  const item = (href: Href | undefined, label: string, icon: React.ReactNode, onPress?: () => void, key?: string) => {
    const selected = href ? isActive(href) : false;
    return (
      <Focusable key={key ?? label} href={href} onPress={onPress} {...focusProps} style={[styles.item, phone && styles.itemPhone]} accessibilityLabel={label}>
        {(active) => (
          <>
            {selected && <View style={[styles.marker, phone && styles.markerPhone]} />}
            <View style={{ opacity: selected || active ? 1 : 0.7 }}>{icon}</View>
            {!phone && (
              <Text numberOfLines={1} style={[styles.label, { opacity: open ? 1 : 0 }, (selected || active) && styles.labelActive]}>
                {label}
              </Text>
            )}
          </>
        )}
      </Focusable>
    );
  };

  const profileItem = item(
    '/profiles',
    profile?.name ?? t('nav.profile'),
    <Avatar color={profile?.avatar ?? 'red'} size={30} />,
    () => setCurrentProfile(null),
    'profile',
  );

  if (phone) {
    return (
      <View style={styles.tabBar}>
        {profileItem}
        {MAIN.map((m) => item(m.href, t(m.label), <Icon d={m.icon} />))}
        {item('/account', t('nav.account'), <Icon d={icons.gear} />)}
      </View>
    );
  }

  return (
    <Animated.View
      style={[styles.rail, { width }]}
      onPointerEnter={() => setHovered(true)}
      onPointerLeave={() => setHovered(false)}
    >
      <LinearGradient
        colors={open ? ['#000', 'rgba(0,0,0,0.85)', 'transparent'] : ['rgba(0,0,0,0.85)', 'rgba(0,0,0,0.35)', 'transparent']}
        locations={open ? [0, 0.75, 1] : [0, 0.7, 1]}
        start={{ x: 0, y: 0.5 }}
        end={{ x: 1, y: 0.5 }}
        style={StyleSheet.absoluteFill}
      />
      {profileItem}
      <View style={styles.main}>{MAIN.map((m) => item(m.href, t(m.label), <Icon d={m.icon} />))}</View>
      <View>
        {item('/account', t('nav.account'), <Icon d={icons.gear} size={22} />)}
        {item(undefined, t('common.logout'), <Icon d={icons.logout} size={22} />, logout)}
      </View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  rail: { position: 'absolute', top: 0, bottom: 0, left: 0, zIndex: 50, paddingVertical: 24, overflow: 'hidden' },
  main: { flex: 1, justifyContent: 'center', gap: 6 },
  item: { flexDirection: 'row', alignItems: 'center', gap: 22, paddingVertical: 10, paddingLeft: 25 },
  itemPhone: { paddingLeft: 10, paddingRight: 10, gap: 0 },
  label: { color: '#b3b3b3', fontSize: 17, minWidth: 160 },
  labelActive: { color: '#fff', fontWeight: '700' },
  marker: { position: 'absolute', left: 0, top: 10, bottom: 10, width: 3, borderRadius: 2, backgroundColor: colors.red },
  markerPhone: { top: 'auto', bottom: 2, left: 10, right: 10, width: 'auto', height: 3 },
  tabBar: {
    position: 'absolute',
    left: 0,
    right: 0,
    bottom: 0,
    height: 64,
    zIndex: 50,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-around',
    backgroundColor: 'rgba(0,0,0,0.95)',
    borderTopWidth: 1,
    borderTopColor: '#222',
  },
});
