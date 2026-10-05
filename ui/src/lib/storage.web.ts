// Web: localStorage, which can throw in private mode or when site data is blocked.
export async function getItem(key: string): Promise<string | null> {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export async function setItem(key: string, value: string | null): Promise<void> {
  try {
    if (value === null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    // the value is only kept in memory for this session
  }
}
