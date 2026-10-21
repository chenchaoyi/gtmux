// SenderAvatar — whose words these are, when they are not yours.
//
// Every prompt in the chat used to wear the reader's own person-battery, because a session
// log records what ARRIVED in a pane and never who caused it to arrive. On 2026-09-20 a
// message HQ relayed into the commander's pane appeared under his face 「对话里应该增加hq的
// 角色，对于hq发的信息用hq自己的头像标识」, and his own history stopped telling him which
// instructions were his.
//
// The server now says (`from` on a turn), and this draws it:
//
//   • HQ wears gtmux's pane-grid logo. The role is also named below the message.
//   • Another session wears ITS agent's icon, the one the radar draws for that row.
//
// Nothing else is drawn. A turn with no sender is the reader's own and keeps UserAvatar,
// which is nearly every turn and the reason this stays additive.
//
// The colours are FIXED light-on-dark, not the theme's: the chat surface is always dark
// whatever the app's appearance, and a pal.fg here goes invisible in light mode
// ([[mobile-light-mode-dark-surface-trap]], which cost ChatView once already).

import React from 'react';
import {Agent} from '../api/types';
import type {TranscriptSender} from '../api/client';
import {AgentAvatar} from './AgentAvatar';
import {HQAvatar} from './HQAvatar';

// The chat surface's own colours (ChatView's CHAT_FG / CHAT_FG_DIM neighbourhood).
const MARK_BG = 'rgba(255,255,255,0.10)';
const ICON_FG = 'rgba(235,235,245,0.7)';

export function SenderAvatar({from, size = 26}: {from: TranscriptSender; size?: number}) {
  if (from.kind === 'agent') {
    // AgentAvatar reads the agent NAME off a row; the sender carries one when the pane
    // was still resolvable. Without it the avatar falls back to its neutral letter mark,
    // which is the right answer for a session that has since closed.
    const as = {agent: from.agent ?? '', session: from.label} as Agent;
    return <AgentAvatar agent={as} size={size} radius={Math.round(size * 0.27)} bg={MARK_BG} fg={ICON_FG} />;
  }
  return <HQAvatar size={size} />;
}
