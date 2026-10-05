import * as SecureStore from 'expo-secure-store';

// Native (phones, Android TV, tvOS): SecureStore, supported on TV.
export async function getItem(key: string): Promise<string | null> {
  return SecureStore.getItemAsync(key);
}

export async function setItem(key: string, value: string | null): Promise<void> {
  if (value === null) await SecureStore.deleteItemAsync(key);
  else await SecureStore.setItemAsync(key, value);
}
