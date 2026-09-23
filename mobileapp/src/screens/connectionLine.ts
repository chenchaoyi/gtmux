// Naming a connection's destination in Settings.
//
// The place, not the host name: once Direct became a pool, the answer a user away from
// their Mac wants is WHERE this connection goes. The host stays for the connections that
// have no place to name — every local address, and the standard tunnel. The rows that use
// these live in connectionGroup.
//
// Shown, never changed from here (openspec/changes/direct-server-choice): moving cuts the
// very connection the phone would ask through, and the case where it is wanted — the
// server is down and you are away — is the case where the phone cannot reach the Mac at
// all. The Mac is where that choice lives.

export interface RouteName {
  id: string;
  en?: string;
  zh?: string;
}

/** The place this connection goes through, in the reader's language ('' when unnamed). */
export function routeName(route: RouteName | undefined, zh: boolean): string {
  if (!route) return '';
  const name = (zh ? route.zh : route.en) || route.en || route.zh || '';
  return name.trim();
}

/** host strips the scheme, so the fallback reads as an address and not as a URL. */
export function hostOf(url: string | undefined): string {
  return (url || '').replace(/^https?:\/\//, '');
}
