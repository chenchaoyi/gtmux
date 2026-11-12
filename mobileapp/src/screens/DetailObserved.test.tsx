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
  expect(chat().readOnlyHistory).toBe(true);expect(compose().enabled).toBe(false);
  await act(async()=>compose().onSend({text:'must not send'}));expect(client.sendResult).not.toHaveBeenCalled();
  client.transcript.mockResolvedValueOnce({turns:[],dropped:0,readOnlyHistory:false,unchanged:true});
  await act(async()=>chat().onRetry());
  expect(chat().readOnlyHistory).toBe(false);expect(compose().enabled).toBe(true);
 }finally{act(()=>tree?.unmount());jest.clearAllTimers();jest.useRealTimers();}
});
