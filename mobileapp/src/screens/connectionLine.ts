// What the Settings "Connection" row says under the Mac's name.
//
// It used to be the state and the host: "Connected · tunnel.example.dev/p35047". A host
// name is not where you are, and once Direct became a pool of servers the answer a user
// away from their Mac actually wants is the PLACE: "Connected · Shanghai". The host stays
// for the connections that have no place to name, which is every LAN address and the
// standard tunnel.
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

/**
 * connectionLine is that subtitle: the state, then the place if there is one, else the
 * address. Offline keeps the place too, in the past tense — where it was last reached is
 * exactly what a user wonders while it is not answering.
 */
export function connectionLine(
  state: string,
  mac: {url?: string; route?: RouteName} | null | undefined,
  zh: boolean,
  online: boolean,
): string {
  if (!mac) return state;
  const place = routeName(mac.route, zh);
  if (!place) {
    const host = hostOf(mac.url);
    return host ? `${state} · ${host}` : state;
  }
  if (online) return `${state} · ${place}`;
  return `${state} · ${zh ? `上次走${place}` : `last on ${place}`}`;
}
