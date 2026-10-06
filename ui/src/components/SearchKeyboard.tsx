import { StyleSheet, Text, View } from 'react-native';
import { t } from '../i18n';
import { Focusable } from './Focusable';
import { Icon, icons } from './ui';

const KEYS = [...'abcdefghijklmnopqrstuvwxyz1234567890'];
const COLUMNS = 6;
export const KEY_SIZE = 34;
const GAP = 4;
export const KEYBOARD_WIDTH = COLUMNS * KEY_SIZE + (COLUMNS - 1) * GAP;

/**
 * TV search keyboard, like Netflix's: a letter grid driven by the remote, part of the page, instead of the system
 * keyboard that covers half the screen.
 */
export function SearchKeyboard({ onKey, onDelete }: { onKey: (key: string) => void; onDelete: () => void }) {
  const half = (KEYBOARD_WIDTH - GAP) / 2;
  return (
    <View style={styles.keyboard}>
      <Key label={t('search.space')} width={half} onPress={() => onKey(' ')}>
        {(color) => <Text style={[styles.wide, { color }]}>{t('search.space')}</Text>}
      </Key>
      <Key label={t('search.delete')} width={half} onPress={onDelete}>
        {(color) => <Icon d={icons.backspace} size={18} color={color} />}
      </Key>
      {KEYS.map((k, i) => (
        <Key key={k} label={k} onPress={() => onKey(k)} first={i === 0}>
          {(color) => <Text style={[styles.letter, { color }]}>{k}</Text>}
        </Key>
      ))}
    </View>
  );
}

function Key({
  label,
  width = KEY_SIZE,
  first,
  onPress,
  children,
}: {
  label: string;
  width?: number;
  first?: boolean;
  onPress: () => void;
  /** Content in the given color: black on the focused (white) key. */
  children: (color: string) => React.ReactNode;
}) {
  return (
    <Focusable
      accessibilityLabel={label}
      onPress={onPress}
      hasTVPreferredFocus={first}
      style={(active) => [styles.key, { width }, active && styles.keyActive]}
    >
      {(active) => children(active ? '#000' : '#fff')}
    </Focusable>
  );
}

const styles = StyleSheet.create({
  keyboard: { width: KEYBOARD_WIDTH, flexDirection: 'row', flexWrap: 'wrap', gap: GAP },
  key: { height: KEY_SIZE, borderRadius: 4, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(255,255,255,0.08)' },
  keyActive: { backgroundColor: '#fff', transform: [{ scale: 1.15 }] },
  letter: { fontSize: 15, fontWeight: '600', textTransform: 'uppercase' },
  wide: { fontSize: 13, fontWeight: '600' },
});
