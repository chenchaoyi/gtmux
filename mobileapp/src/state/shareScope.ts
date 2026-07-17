// nextLinkScope computes a share link's new (view, input) allowlists after toggling
// one pane's See or Type. The invariant is Type ⊆ See (a guest can never type into a
// pane it can't see): turning See OFF drops Type for that pane too; turning Type ON
// implies See. Pure so the owner-remote-admin editor's core rule is unit-tested.

export function nextLinkScope(
  view: string[],
  input: string[],
  pane: string,
  facet: 'see' | 'type',
  on: boolean,
): {view: string[]; input: string[]} {
  const v = new Set(view);
  const i = new Set(input);
  if (facet === 'see') {
    if (on) v.add(pane);
    else {
      v.delete(pane);
      i.delete(pane); // Type ⊆ See
    }
  } else {
    if (on) {
      i.add(pane);
      v.add(pane); // typing implies seeing
    } else i.delete(pane);
  }
  return {view: [...v], input: [...i]};
}
