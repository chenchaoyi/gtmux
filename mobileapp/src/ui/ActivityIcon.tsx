import React from 'react';
import Svg, {Path} from 'react-native-svg';
import {ActivityOperation} from './activity';

// One neutral line-icon family. These identify actions, never outcomes.
const paths: Record<ActivityOperation, string> = {
  read: 'M12 5C9 3 5 3 3 4v15c3-1 6-1 9 1 3-2 6-2 9-1V4c-2-1-6-1-9 1v15 M6 8h3 M15 8h3 M6 12h3 M15 12h3',
  list: 'M3 7V5h6l2 2h10v13H3V7Z M3 10h18',
  search: 'M16 10a6 6 0 1 1-12 0 6 6 0 0 1 12 0Z M14.5 14.5 21 21',
  command: 'M3 4h18v16H3Z M7 9l3 3-3 3 M13 15h4',
  edit: 'm14 5 5 5 M5 14l11-11 5 5-11 11-6 1 1-6Z',
  web: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z M3 12h18 M12 3c5 5 5 13 0 18-5-5-5-13 0-18Z',
  code: 'm8 6-6 6 6 6 M16 6l6 6-6 6 M14 3l-4 18',
  wait: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z M12 6v6l4 2',
  agent: 'M14 7a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z M4 21v-3a6 6 0 0 1 10-4 M16 17h6 M19 14l3 3-3 3',
  tool: 'M14 5a6 6 0 0 0-7 7l-5 5 5 5 5-5a6 6 0 0 0 7-7l-4 3-4-4 3-4Z',
};
export function ActivityIcon({operation, size = 18}: {operation: ActivityOperation; size?: number}) {
  return <Svg testID={`activity-icon-${operation}`} accessible={false} width={size} height={size} viewBox="0 0 24 24" fill="none">
    <Path d={paths[operation]} stroke="#AAAAB4" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" />
  </Svg>;
}
