import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {HQConsoleView} from './HQScreen';
import {Composer} from '../ui/Composer';
import {ChatView} from '../ui/ChatView';
import {SendFailedBar} from '../ui/SendFailedBar';
import {SendDeadlineError} from '../api/sendDeadline';
jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/ChatView', () => ({ChatView: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));
const mockClient = {
 base: 'https://synthetic.invalid', tasks: jest.fn(async () => []), digest: jest.fn(async () => []), usage: jest.fn(async () => null),
 hqBoard: jest.fn(async () => ({exists:false})),hqKnowledge:jest.fn(async()=>({entries:[],topics:[],promotions:{pending:0},candidates:{pending:0}})),
 hqEvents:jest.fn(async()=>[]),pane:jest.fn(async()=>({text:''})),transcript:jest.fn(async()=>({turns:[] as any[]})),
 send:jest.fn(async()=>({text:''})),sendResult:jest.fn(),
};
const hq:any={pane_id:'%synthetic',session_id:'session-A',home_id:'home',agent:'Codex',status:'idle',role:'supervisor'};
const mockContext={client:mockClient,agents:[hq],conn:'connected',demo:false};
const ComposerFn=(Composer as unknown as {type:React.ComponentType<any>}).type;
let tree:renderer.ReactTestRenderer;
const flush=async()=>{for(let i=0;i<6;i++)await act(async()=>{await Promise.resolve();});};
async function mount(agent=hq){await act(async()=>{tree=renderer.create(<HQConsoleView agent={agent} workspaceMode />);});await flush();}
async function send(text:string){await act(async()=>{tree.root.findByType(ComposerFn).props.onSend({text,enter:true});});await flush();}
const chat=()=>tree.root.findByType(ChatView).props;
beforeEach(()=>{jest.useFakeTimers();jest.clearAllMocks();mockContext.agents=[hq];mockClient.sendResult.mockReset().mockResolvedValue({ok:true,delivery:'accepted'});mockClient.transcript.mockResolvedValue({turns:[]});});
afterEach(()=>{act(()=>tree?.unmount());jest.useRealTimers();});

test.each(['queued','accepted','submitted',undefined])('MSD-007 HQ exposes %s receipt with stable message identity',async delivery=>{
 mockClient.sendResult.mockResolvedValue({ok:true,delivery});await mount();await send('new prompt');
 expect(mockClient.sendResult).toHaveBeenCalledTimes(1);expect(mockClient.send).not.toHaveBeenCalled();
 expect(mockClient.sendResult.mock.calls[0][1].send_id).toEqual(expect.any(String));
 expect(chat()).toMatchObject({pendingPrompt:'new prompt',pendingDelivered:true,pendingDelivery:delivery});
});
test('MSD-007 HQ timeout retry after 21 seconds reuses original send ID',async()=>{
 mockClient.sendResult.mockRejectedValueOnce(new SendDeadlineError());await mount();await send('kept prompt');
 expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(1);
 const payload=mockClient.sendResult.mock.calls[0][1];expect(payload.send_id).toEqual(expect.any(String));
 await act(async()=>{jest.advanceTimersByTime(21000);});await flush();
 expect(mockClient.sendResult).toHaveBeenCalledTimes(1);
 await act(async()=>{tree.root.findByType(SendFailedBar).props.onRetry();});await flush();
 expect(mockClient.sendResult.mock.calls[1][1]).toEqual(payload);
 expect(chat()).toMatchObject({pendingPrompt:'kept prompt',pendingDelivered:true,pendingDelivery:'accepted'});
});
test('MSD-007 old identical transcript cannot retire the new HQ message',async()=>{
 mockClient.transcript.mockResolvedValue({turns:[{prompt:'repeat',time:'old'}]});await mount();await send('repeat');
 expect(chat()).toMatchObject({pendingPrompt:'repeat',pendingDelivered:true});
 mockClient.transcript.mockResolvedValue({turns:[{prompt:'repeat',time:'old'},{prompt:'repeat',time:'new'}]});
 await act(async()=>{jest.advanceTimersByTime(8000);});await flush();
 expect(chat().pendingPrompt).toBeUndefined();
});
