import { useEffect, useState, useSyncExternalStore } from 'react';
import { Keyboard, Platform, StyleSheet, Text, View } from 'react-native';
import { colors } from '../theme';

type Typing = { label: string; value: string; secure: boolean };

let typing: Typing | null = null;
const listeners = new Set<() => void>();

/** Field publishes the focused field here on TV, null once it loses the focus. */
export function setTyping(next: Typing | null) {
  typing = next;
  listeners.forEach((l) => l());
}

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};

/**
 * TV: the on-screen keyboard covers the bottom of the screen, often the field being typed in. While it is open, this
 * banner at the top (keyboards never reach it) shows what is being typed.
 */
export function TypingPreview() {
  const current = useSyncExternalStore(subscribe, () => typing);
  const [keyboard, setKeyboard] = useState(false);
  useEffect(() => {
    if (!Platform.isTV) return;
    const subs = [Keyboard.addListener('keyboardDidShow', () => setKeyboard(true)), Keyboard.addListener('keyboardDidHide', () => setKeyboard(false))];
    return () => subs.forEach((s) => s.remove());
  }, []);
  if (!Platform.isTV || !keyboard || !current) return null;
  return (
    <View style={styles.bar} pointerEvents="none">
      {!!current.label && <Text style={styles.label}>{current.label}</Text>}
      <Text style={styles.value} numberOfLines={1}>
        {current.secure ? '•'.repeat(current.value.length) : current.value}
        <Text style={styles.caret}>|</Text>
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: { position: 'absolute', top: 0, left: 0, right: 0, zIndex: 1000, paddingHorizontal: 64, paddingVertical: 20, gap: 4, backgroundColor: 'rgba(20,20,20,0.97)', borderBottomWidth: 2, borderBottomColor: colors.red },
  label: { color: colors.muted, fontSize: 15 },
  value: { color: '#fff', fontSize: 28, fontWeight: '600' },
  caret: { color: colors.red, fontWeight: '400' },
});
