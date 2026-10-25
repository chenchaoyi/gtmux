// Endpoint identity, not the display name or a Direct server's place ID. Do not
// resolve DNS, remove paths, equate http with https, or assume non-default ports.
export function normalizeServerUrl(raw: string): string {
  const trimmed = raw.trim().replace(/\/+$/, '');
  // RN's URL implementation varies with the OS/runtime; this deliberately only
  // canonicalizes the scheme/authority, retaining path spelling and routing.
  return trimmed.replace(/^(https?):\/\/([^/]+)(.*)$/i, (_, scheme, authority, path) => {
    const protocol = scheme.toLowerCase();
    let host = authority.toLowerCase();
    host = host.replace(protocol === 'https' ? /:443$/ : /:80$/, '');
    return `${protocol}://${host}${path}`;
  });
}
