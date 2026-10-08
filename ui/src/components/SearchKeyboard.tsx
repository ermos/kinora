import { StyleSheet, Text, View } from 'react-native';
import { t } from '../i18n';
import { Focusable } from './Focusable';
import { Icon, icons } from './ui';

const KEYS = [...'abcdefghijklmnopqrstuvwxyz1234567890'];
const COLUMNS = 6;
const KEY_HEIGHT = 34;
const GAP = 4;

/**
 * TV search keyboard, like Netflix's: a letter grid driven by the remote, part of the page, instead of the system
 * keyboard that covers half the screen.
 */
/** Fills the given width: keys widen with it, their height stays the same. onVoice adds a dictation key. */
export function SearchKeyboard({
  width,
  onKey,
  onDelete,
  onVoice,
}: {
  width: number;
  onKey: (key: string) => void;
  onDelete: () => void;
  onVoice?: () => void;
}) {
  const key = (width - (COLUMNS - 1) * GAP) / COLUMNS;
  const n = onVoice ? 3 : 2;
  const wide = (width - (n - 1) * GAP) / n;
  return (
    <View style={[styles.keyboard, { width }]}>
      {KEYS.map((k, i) => (
        <Key key={k} label={k} width={key} onPress={() => onKey(k)} first={i === 0}>
          {(color) => <Text style={[styles.letter, { color }]}>{k}</Text>}
        </Key>
      ))}
      <Key label={t('search.space')} width={wide} onPress={() => onKey(' ')}>
        {(color) => <Text style={[styles.wide, { color }]}>{t('search.space')}</Text>}
      </Key>
      {onVoice && (
        <Key label={t('search.voice')} width={wide} onPress={onVoice}>
          {(color) => <Icon d={icons.mic} size={18} color={color} />}
        </Key>
      )}
      <Key label={t('search.delete')} width={wide} onPress={onDelete}>
        {(color) => <Icon d={icons.backspace} size={18} color={color} />}
      </Key>
    </View>
  );
}

function Key({
  label,
  width,
  first,
  onPress,
  children,
}: {
  label: string;
  width: number;
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
  keyboard: { flexDirection: 'row', flexWrap: 'wrap', gap: GAP },
  key: { height: KEY_HEIGHT, borderRadius: 4, alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(255,255,255,0.08)' },
  keyActive: { backgroundColor: '#fff', transform: [{ scale: 1.15 }] },
  letter: { fontSize: 15, fontWeight: '600', textTransform: 'uppercase' },
  wide: { fontSize: 13, fontWeight: '600' },
});
