'use client';

import { useEffect, useState } from 'react';
import { getCurrentUser, restoreSession, subscribe, type SessionUser } from '@/lib/auth';

/**
 * Subscribes a component to the in-memory session and revalidates it against the
 * API once on first mount.
 *
 * The session token lives in memory only, so a page reload has no session until
 * `/user/profile` answers. The restore runs a single time per page load no
 * matter how many components use this hook; the shared promise is the guard.
 */
let restoreOnce: Promise<SessionUser | null> | null = null;

function restore(): Promise<SessionUser | null> {
  if (!restoreOnce) {
    restoreOnce = restoreSession().catch(() => null);
  }
  return restoreOnce;
}

export function useSession(): { user: SessionUser | null; loading: boolean } {
  const [user, setUser] = useState<SessionUser | null>(getCurrentUser());
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const unsubscribe = subscribe((next) => {
      if (!cancelled) setUser(next);
    });
    void restore().then((restored) => {
      if (cancelled) return;
      setUser(restored ?? getCurrentUser());
      setLoading(false);
    });
    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

  return { user, loading };
}
