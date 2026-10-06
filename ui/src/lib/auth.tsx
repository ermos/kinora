import { useQuery } from '@tanstack/react-query';
import { Redirect } from 'expo-router';
import type { ReactNode } from 'react';
import { api, needsServer, unwrap, useProfile } from '../api/client';
import { Spinner } from '../components/ui';

export function useMe() {
  return useQuery({ queryKey: ['me'], queryFn: () => unwrap(api.GET('/me')), retry: false, enabled: !needsServer() });
}

/** Requires a session, and a selected profile unless needsProfile is false. */
export function Gate({ children, needsProfile = true }: { children: ReactNode; needsProfile?: boolean }) {
  const me = useMe();
  const profile = useProfile();
  if (needsServer()) return <Redirect href="/server" />;
  if (me.isPending) return <Spinner full />;
  if (me.isError) return <Redirect href="/login" />;
  if (needsProfile && !profile) return <Redirect href="/profiles" />;
  return children;
}
