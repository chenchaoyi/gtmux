import React from 'react';
import renderer,{act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {ChatView} from '../ui/ChatView';
import {Composer} from '../ui/Composer';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';
jest.mock('../state/AgentsContext',()=>({useAgents:jest.fn(),useAgentsOptional:jest.fn()}));
jest.mock('../state/AppContext',()=>({useApp:jest.fn()}));
const ComposerFn=(Composer as unknown as {type:React.ComponentType<any>}).type;

test('SID006 mounted Detail fences historical chat and clears restriction after verified refresh',async()=>{
 jest.useFakeTimers();let tree:renderer.ReactTestRenderer|undefined;
 const agent:any={pane_id:'%fixture',session:'fixture',agent:'Codex',source:'tmux',status:'idle'};
 const client={base:'https://synthetic.invalid',token:'fixture',pane:jest.fn(async()=>({text:'ready',cols:80})),transcript:jest.fn(async()=>({turns:[{prompt:'old',response:'historical',time:''}],dropped:0,readOnlyHistory:true,unchanged:false})),sendResult:jest.fn(async()=>({ok:true})),panes:jest.fn(async()=>[]),tasks:jest.fn(async()=>[]),options:jest.fn(async()=>[]),theme:jest.fn(async()=>undefined)};
 (useAgents as jest.Mock).mockReturnValue({client,agents:[agent],conn:'live',isGuest:false,inputPanes:[],demo:false});
 (useApp as jest.Mock).mockReturnValue({pal:paletteFor('dark'),lang:'en',defaultDetailMode:'chat',fontPref:'auto',mac:{name:'Fixture'},returnSends:false});
 try{
  await act(async()=>{tree=renderer.create(<DetailView agent={agent} initialMode="chat"/>);});
  const chat=()=>tree!.root.findByType(ChatView).props;const compose=()=>tree!.root.findByType(ComposerFn).props;
  expect(chat().readOnlyHistory).toBe(true);expect(compose().enabled).toBe(false);expect(compose().draftOnly).toBe(true);
  await act(async()=>compose().onSend({text:'must not send'}));expect(client.sendResult).not.toHaveBeenCalled();
  client.transcript.mockResolvedValueOnce({turns:[],dropped:0,readOnlyHistory:false,unchanged:true});
  await act(async()=>chat().onRetry());
  expect(chat().readOnlyHistory).toBe(false);expect(compose().enabled).toBe(true);expect(compose().draftOnly).toBe(false);
 }finally{act(()=>tree?.unmount());jest.clearAllTimers();jest.useRealTimers();}
});

test('CKD003 Detail keeps the pending draft scope when the same conversation is verified', async () => {
 jest.useFakeTimers(); let tree: renderer.ReactTestRenderer | undefined;
 const observed: any = {pane_id:'%draft-fixture',session:'fixture',agent:'Codex',source:'tmux',status:'idle',observed_conversation:{session_id:'history-a',home_id:'home',state:'last-known'}};
 const verified = {...observed, observed_conversation:undefined, session_id:'history-a'};
 const other = {...observed, observed_conversation:{...observed.observed_conversation, session_id:'history-b'}};
 const client={base:'https://synthetic.invalid',token:'fixture',pane:jest.fn(async()=>({text:'ready',cols:80})),transcript:jest.fn(async()=>({turns:[],dropped:0,readOnlyHistory:false})),sendResult:jest.fn(),panes:jest.fn(async()=>[]),tasks:jest.fn(async()=>[]),options:jest.fn(async()=>[]),theme:jest.fn(async()=>undefined)};
 const context={client,agents:[observed],conn:'live',isGuest:false,inputPanes:[],demo:false};
 (useAgents as jest.Mock).mockReturnValue(context);
 (useApp as jest.Mock).mockReturnValue({pal:paletteFor('dark'),lang:'en',defaultDetailMode:'chat',fontPref:'auto',mac:{name:'Fixture'},returnSends:false});
 try {
  await act(async()=>{tree=renderer.create(<DetailView agent={observed} initialMode="chat"/>);});
  const composer=()=>tree!.root.findByType(ComposerFn).props;
  const pendingKey=composer().draftKey;
  expect(composer().enabled).toBe(false);
  context.agents=[verified];
  await act(async()=>tree!.update(<DetailView agent={verified} initialMode="chat"/>));
  expect(composer().draftKey).toBe(pendingKey);
  expect(composer().enabled).toBe(true);
  context.agents=[other];
  await act(async()=>tree!.update(<DetailView agent={other} initialMode="chat"/>));
  expect(composer().draftKey).not.toBe(pendingKey);
  expect(composer().enabled).toBe(false);
  expect(client.sendResult).not.toHaveBeenCalled();
 } finally {act(()=>tree?.unmount());jest.clearAllTimers();jest.useRealTimers();}
});
