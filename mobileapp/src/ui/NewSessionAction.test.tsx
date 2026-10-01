import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {NewSessionAction} from './NewSessionAction';
import {NewSessionSheet} from './NewSessionSheet';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {useWorkspace} from '../state/WorkspaceContext';
import {paletteFor} from './theme';
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: jest.fn()}));
const select=jest.fn(); const refresh=jest.fn();
const trees:renderer.ReactTestRenderer[]=[];
beforeEach(() => {
  jest.clearAllMocks();
  (useAgents as jest.Mock).mockReturnValue({client:{},conn:'live',isGuest:false,refresh});
  (useApp as jest.Mock).mockReturnValue({mac:{name:'Studio'},lang:'en',pal:paletteFor('dark')});
  (useWorkspace as jest.Mock).mockReturnValue({select});
});
afterEach(()=>{for(const tree of trees.splice(0))act(()=>tree.unmount());});
function mount(){let tree!:renderer.ReactTestRenderer;act(()=>{tree=renderer.create(<NewSessionAction/>);});trees.push(tree);return tree;}
for(const condition of [{isGuest:true},{demo:true}])test(`no creation entry for ${JSON.stringify(condition)}`,()=>{
 (useAgents as jest.Mock).mockReturnValue({...useAgents(),...condition});expect(mount().toJSON()).toBeNull();
});
test('offline entry is visibly and accessibly disabled',()=>{
 (useAgents as jest.Mock).mockReturnValue({...useAgents(),conn:'offline'});
 const b=mount().root.findByProps({testID:'new-session-open'});expect(b.props.disabled).toBe(true);expect(b.props.accessibilityState.disabled).toBe(true);
});
test.each(['compact', 'regular'])('dismisses first and selects the exact new terminal in the %s shell',mode=>{
 (useWorkspace as jest.Mock).mockReturnValue({select,mode});
 const t=mount();act(()=>t.root.findByProps({testID:'new-session-open'}).props.onPress());
 const sheet=()=>t.root.findByType(NewSessionSheet);
 expect(sheet().props.layout).toBe(mode);
 act(()=>sheet().props.onCreated({session:'work',pane_id:'%8',loc:'work:0.0',window:'0',pane:'0'}));
 expect(select).not.toHaveBeenCalled();expect(sheet().props.visible).toBe(false);
 act(()=>sheet().props.onDismiss());
 expect(select).toHaveBeenCalledWith(expect.objectContaining({kind:'pane',mode:'terminal',agent:expect.objectContaining({pane_id:'%8',session:'work',agent:''})}));
 expect(refresh).toHaveBeenCalledTimes(1);
 act(()=>sheet().props.onDismiss());expect(select).toHaveBeenCalledTimes(1);
});

test('checking an uncertain result refreshes the existing pane browser after dismissal',()=>{
 const onRefresh=jest.fn();let tree!:renderer.ReactTestRenderer;
 act(()=>{tree=renderer.create(<NewSessionAction onRefresh={onRefresh}/>);});trees.push(tree);
 act(()=>tree.root.findByProps({testID:'new-session-open'}).props.onPress());
 act(()=>tree.root.findByType(NewSessionSheet).props.onCheckSessions());
 expect(select).not.toHaveBeenCalled();
 act(()=>tree.root.findByType(NewSessionSheet).props.onDismiss());
 expect(onRefresh).toHaveBeenCalledTimes(1);expect(select).toHaveBeenCalledWith({kind:'panes'});
});
