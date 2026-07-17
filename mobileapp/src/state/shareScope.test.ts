import {nextLinkScope} from './shareScope';

describe('nextLinkScope (owner-remote-admin See/Type editor)', () => {
  it('See on adds the pane to view only', () => {
    expect(nextLinkScope([], [], '%1', 'see', true)).toEqual({view: ['%1'], input: []});
  });

  it('Type on implies See (adds to both)', () => {
    expect(nextLinkScope([], [], '%1', 'type', true)).toEqual({view: ['%1'], input: ['%1']});
  });

  it('See off drops Type too (Type ⊆ See)', () => {
    expect(nextLinkScope(['%1'], ['%1'], '%1', 'see', false)).toEqual({view: [], input: []});
  });

  it('Type off leaves See intact', () => {
    expect(nextLinkScope(['%1'], ['%1'], '%1', 'type', false)).toEqual({view: ['%1'], input: []});
  });

  it('does not disturb other panes', () => {
    expect(nextLinkScope(['%1', '%2'], ['%2'], '%1', 'type', true)).toEqual({
      view: ['%1', '%2'],
      input: ['%2', '%1'],
    });
  });

  it('is idempotent (toggling on an already-on facet)', () => {
    expect(nextLinkScope(['%1'], [], '%1', 'see', true)).toEqual({view: ['%1'], input: []});
  });
});
