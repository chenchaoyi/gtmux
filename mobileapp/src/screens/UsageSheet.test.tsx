import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {UsageSheet} from './UsageSheet';
import {UsageReport} from '../api/client';
import {MachineIcon} from '../ui/MachineIcon';
import {AgentAvatar} from '../ui/AgentAvatar';
import {paletteFor} from '../ui/theme';

// Walk the rendered JSON for text: <Text> lands as a HOST node whose type is the string
// 'Text', which a `typeof type !== 'string'` predicate silently excludes.
type Json = {children?: (Json | string)[]} | string | null;
function texts(tree: renderer.ReactTestRenderer): string[] {
  const out: string[] = [];
  const walk = (n: Json) => {
    if (n == null) return;
    if (typeof n === 'string') return void out.push(n);
    (n.children ?? []).forEach(walk);
  };
  walk(tree.toJSON() as Json);
  return out;
}

const render = (usage: UsageReport | null) => {
  let t!: renderer.ReactTestRenderer;
  act(() => {
    t = renderer.create(
      <UsageSheet visible usage={usage} agents={[]} pal={paletteFor('dark')} lang="zh" onClose={() => {}} />,
    );
  });
  return t;
};

// An agent with a plan and no live session: Codex on a Claude-only day.
const codexOnly: UsageReport = {
  limits: {
    at: 1_789_000_000,
    windows: [
      {agent: 'claude', agent_name: 'Claude Code', label: 'claude week (all models)', pct_used: 76, reset_at: 'Sep 11 at 10:59pm'},
      {agent: 'codex', agent_name: 'Codex', label: 'codex week', pct_used: 0, reset_at: 'Sep 14 at 7:19pm'},
    ],
  },
};

describe('an agent that has a plan but no live session', () => {
  it('is called by its name, not by its registry key', () => {
    // It read "codex" beside "Claude Code" until the report carried the name (2026-09-10):
    // the phone learned spellings from SESSION rows, and this agent has none.
    const seen = texts(render(codexOnly));
    expect(seen).toContain('Codex');
    expect(seen).not.toContain('codex');
  });

  it('asks for an icon at all — the fallback used to carry no hint', () => {
    // No hint meant no request, so the neutral monogram stood in beside a plan gtmux had
    // read perfectly well. The hint is the registry key; /api/icon answers to it.
    const avatars = render(codexOnly).root.findAllByType(AgentAvatar);
    const codex = avatars.find(a => a.props.agent.agent === 'Codex');
    expect(codex).toBeDefined();
    expect(codex!.props.agent.icon).toBeTruthy();
  });
});

describe('a session row', () => {
  const warned: UsageReport = {
    sessions: [
      {pane_id: '%1', loc: 'mp-server-reqs:0.0', agent: 'Claude Code', agent_key: 'claude', tok: 1_345_727, ctx: 0.995, rate: 0, usage_warn: 'ctx 99%'},
      {pane_id: '%2', loc: 'gtmux dev:0.0', agent: 'Claude Code', agent_key: 'claude', tok: 11_270_018, ctx: 0.36, rate: 3447},
      {pane_id: '%3', loc: 'quiet:0.0', agent: 'Claude Code', agent_key: 'claude', tok: 500_000, ctx: 0.2, rate: 0},
    ],
  } as unknown as UsageReport;

  it('states ctx ONCE — two roundings of it used to sit side by side', () => {
    // The screen carried the core's "ctx 99%" in the warn column and its own
    // Math.round(ctx*100) = "ctx 100%" in the sub-line, on the same row (2026-09-10).
    const joined = texts(render(warned)).join('|');
    expect(joined).toContain('ctx 99%');
    expect(joined).not.toContain('ctx 100%');
  });

  it('keeps a warned session on screen and counts only the quiet ones', () => {
    const t = render(warned);
    const seen = texts(t).join('|');
    expect(seen).toContain('mp-server-reqs:0.0'); // warned, idle — never folded away
    expect(seen).toContain('gtmux dev:0.0'); // producing
    expect(seen).not.toContain('quiet:0.0'); // idle and silent → a count
    expect(t.root.findAllByProps({testID: 'usage-rest'}).length).toBeGreaterThan(0);
  });
});

describe('the lead', () => {
  it('says the tightest window before any section', () => {
    expect(texts(render(codexOnly)).join('|')).toContain('最紧的额度用掉 76%');
  });

  it('lifts the machine warning out of the bottom of the page', () => {
    // 18 session rows put an amber disk warning below the fold on 2026-09-10.
    const u = {...codexOnly, resource: {machine: {disk_free_gb: 28, disk_use_pct: 93, tier: 'amber' as const, warn: '磁盘快满了 · 28GB 可用'}}};
    expect(texts(render(u)).join('|')).toContain('磁盘快满了 · 28GB 可用');
  });

  it('draws nothing when there is neither', () => {
    expect(render({sessions: []} as UsageReport).root.findAllByProps({testID: 'usage-lead'})).toHaveLength(0);
  });
});

describe('the machine block', () => {
  const u = {resource: {machine: {disk_free_gb: 28, disk_use_pct: 93, mem_free_pct: 33, load_ratio: 0.79, ncpu: 14, tier: 'amber' as const}}} as UsageReport;

  it('names each resource with an icon', () => {
    const kinds = render(u).root.findAllByType(MachineIcon).map(i => i.props.kind);
    expect(kinds).toEqual(['disk', 'memory', 'load']);
  });

  it('still encodes state three ways, so the icon can stay identity-only', () => {
    // DESIGN §1: colour + shape + glyph. The icon took the shape channel's old place, so
    // the ⚠ must still be there beside the value.
    expect(texts(render(u))).toContain('⚠');
  });
});

// A count is a summary, not a deletion: every one of those sessions used to be on screen.
it('opens the idle ones back up', () => {
  const u = {
    sessions: [
      {pane_id: '%1', loc: 'busy:0.0', agent: 'Claude Code', agent_key: 'claude', tok: 9, ctx: 0.1, rate: 500},
      {pane_id: '%2', loc: 'quiet:0.0', agent: 'Claude Code', agent_key: 'claude', tok: 9, ctx: 0.1, rate: 0},
    ],
  } as unknown as UsageReport;
  const t = render(u);
  expect(texts(t).join('|')).not.toContain('quiet:0.0');
  act(() => t.root.findByProps({testID: 'usage-rest'}).props.onPress());
  expect(texts(t).join('|')).toContain('quiet:0.0');
});
