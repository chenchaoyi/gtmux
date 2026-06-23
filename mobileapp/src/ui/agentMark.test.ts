import {agentMark} from './agentMark';

describe('agentMark', () => {
  it('maps each known exact agent name to its mark', () => {
    expect(agentMark('claude code')).toBe('CC');
    expect(agentMark('claude')).toBe('CC');
    expect(agentMark('codex')).toBe('Cx');
    expect(agentMark('gemini')).toBe('G');
    expect(agentMark('aider')).toBe('Ai');
    expect(agentMark('opencode')).toBe('oc');
    expect(agentMark('cursor')).toBe('Cu');
    expect(agentMark('crush')).toBe('Cr');
    expect(agentMark('amp')).toBe('Am');
    expect(agentMark('cline')).toBe('Cl');
  });

  it('is case-insensitive and trims whitespace', () => {
    expect(agentMark('  Claude Code  ')).toBe('CC');
    expect(agentMark('CODEX')).toBe('Cx');
    expect(agentMark('\tGemini\n')).toBe('G');
  });

  it('matches a known key as a substring of a longer name', () => {
    // e.g. process names like "claude code (1.2.3)" or "node codex-cli".
    expect(agentMark('claude code v2')).toBe('CC');
    expect(agentMark('running codex now')).toBe('Cx');
    expect(agentMark('opencode-server')).toBe('oc');
  });

  it('prefers an exact map hit over substring scanning', () => {
    // "claude" is exact -> CC, not a partial of "claude code".
    expect(agentMark('claude')).toBe('CC');
  });

  it('falls back to the first two characters when nothing matches', () => {
    expect(agentMark('zed')).toBe('ze');
    expect(agentMark('Helix')).toBe('He');
  });

  it('preserves original case in the 2-char fallback', () => {
    expect(agentMark('Xyz')).toBe('Xy');
  });

  it('returns a single char when the name is one char', () => {
    expect(agentMark('q')).toBe('q');
  });

  it('returns "?" for empty / whitespace-only / undefined / null', () => {
    expect(agentMark('')).toBe('?');
    expect(agentMark('   ')).toBe('?');
    expect(agentMark(undefined as any)).toBe('?');
    expect(agentMark(null as any)).toBe('?');
  });
});
