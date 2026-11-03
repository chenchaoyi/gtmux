import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {SessionToolsMenu} from './SessionToolsMenu';
import {paletteFor} from './theme';
test('reading mode exposes only available tools directly, with distinct accessible actions',()=>{
 const trace=jest.fn(),diff=jest.fn();let tree!:renderer.ReactTestRenderer;
 act(()=>{tree=renderer.create(<SessionToolsMenu presentation="direct" pal={paletteFor('dark')} lang="en" onTrace={trace} onDiff={diff} />);});
 const button=(name:string)=>tree.root.findAll(node=>node.props.testID===`session-tool-${name}`&&typeof node.props.onPress==='function')[0];
 expect(button('trace').props.accessibilityLabel).toBe('Trace');expect(button('diff').props.accessibilityLabel).toBe('Diff');
 act(()=>button('trace').props.onPress());expect(trace).toHaveBeenCalledTimes(1);expect(diff).not.toHaveBeenCalled();
 act(()=>tree.update(<SessionToolsMenu presentation="direct" pal={paletteFor('dark')} lang="zh" onDiff={diff} />));
 expect(tree.root.findAllByProps({testID:'session-tool-trace'})).toHaveLength(0);
 act(()=>button('diff').props.onPress());expect(diff).toHaveBeenCalledTimes(1);act(()=>tree.unmount());
});
