// The agent icons the fake serve hands out over GET /api/icon.
//
// The real serve answers from the PNGs built into the CLI (assets/agent-icons/<key>.png,
// assets.AgentIcon) and then from the agent's installed app (internal/app/serve.go
// agentIconPNG). The fake has no installed apps, so it answers from one directory:
// GTMUX_FAKE_ICON_DIR when set, else the repository's assets/agent-icons.
//
// The override exists for the website screenshots. The repository ships only the marks
// its owner provided, and there is deliberately no claude.png among them, so a capture
// that should show the Claude mark points GTMUX_FAKE_ICON_DIR at a local directory that
// has one. That directory is never committed.

import {existsSync, readFileSync} from 'fs';
import {join, resolve} from 'path';

/**
 * Display label → registry key, as internal/agents/registry.go declares them. The app asks
 * for an icon by the row's `agent` field, which is the LABEL ("Claude Code"); the files are
 * named by KEY ("claude"). The real serve accepts either spelling (agentIconPNG tries the
 * name, then agents.KeyForLabel), and so does the fake.
 *
 * icons.test.ts reads the Go registry and fails when this table no longer matches it.
 */
export const AGENT_KEYS: Readonly<Record<string, string>> = {
  'Claude Code': 'claude',
  Codex: 'codex',
  Gemini: 'gemini',
  Cursor: 'cursor',
  opencode: 'opencode',
  'Kimi Code': 'kimi',
  Copilot: 'copilot',
  Kiro: 'kiro',
  Hermes: 'hermes-agent',
  Grok: 'grok',
  Aider: 'aider',
  Crush: 'crush',
  Amp: 'amp',
};

/** The repository's built-in icons, the same files the CLI embeds. */
export const REPO_ICON_DIR = resolve(__dirname, '../../../assets/agent-icons');

/** iconDir is where /api/icon reads from now. Read per call, so a suite can set the env late. */
export function iconDir(): string {
  const dir = (process.env.GTMUX_FAKE_ICON_DIR ?? '').trim();
  return dir ? resolve(dir) : REPO_ICON_DIR;
}

// A key names a file, so only a registry-shaped word is one. "../x" is not an agent.
const KEY_SHAPE = /^[a-z0-9][a-z0-9-]*$/;

/**
 * iconFile is the PNG that answers `GET /api/icon?agent=<name>`, or null when there is none
 * (the serve's 404, after which the app shows its monogram). `name` may be the label or the
 * key, as on the real serve: the name as given first, then the key for that label.
 */
export function iconFile(name: string): string | null {
  const dir = iconDir();
  for (const key of [name, AGENT_KEYS[name]]) {
    if (!key || !KEY_SHAPE.test(key)) continue;
    const file = join(dir, `${key}.png`);
    if (existsSync(file)) return file;
  }
  return null;
}

/** iconBytes reads iconFile's PNG, or null when there is none. */
export function iconBytes(name: string): Buffer | null {
  const file = iconFile(name);
  if (!file) return null;
  try {
    return readFileSync(file);
  } catch {
    return null;
  }
}
