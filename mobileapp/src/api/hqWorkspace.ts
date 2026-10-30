import type { TranscriptTurn } from './client';

export type MatterState = 'open' | 'needs-input' | 'completed' | 'closed';
export interface HQConversation {
  agent: string;
  home_id?: string;
  session_id: string;
  name: string;
  pane?: string;
  loc?: string;
}
export interface HQAssociation {
  id: string;
  conversation: HQConversation;
  role: 'execution' | 'reference';
  from: number;
  until?: number;
  removed_at?: number;
}
export interface HQMatter {
  id: string;
  title: string;
  state: MatterState;
  assessment?: string;
  updated_at: number;
  revision: number;
  participants: HQAssociation[];
  merged_into?: string;
}
export interface HQPreview {
  message_id?: string;
  kind: string;
  body: string;
  at?: number;
  truncated?: boolean;
}
export interface HQOverviewItem {
  matter_id: string;
  preview?: HQPreview;
}
export interface HQWorkspace {
  schema: 1;
  revision: number;
  matters: HQMatter[];
  overview: {
    generated_at: number;
    since: number;
    needs_input: HQOverviewItem[];
    following: HQOverviewItem[];
    recent: {
      matter_id: string;
      kind: string;
      state?: MatterState;
      at: number;
      seq: number;
    }[];
    recent_total: number;
  };
}
export interface HQMatterMessage {
  id: string;
  seq: number;
  speaker: 'user' | 'hq';
  kind: string;
  body: string;
  at: number;
  delivery?: string;
  reply?: TranscriptTurn;
  target?: HQConversation;
}
export interface HQMatterDetail {
  schema: 1;
  matter: HQMatter;
  messages: HQMatterMessage[];
  message_earlier_available: boolean;
  earlier_before?: number;
  warnings: string[];
}
