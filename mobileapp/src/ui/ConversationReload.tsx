import {readingColors, useReadingPalette} from './ReadingPalette';
import type {Palette} from './theme';
import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {LoadingMark} from './LoadingMark';
import {ReloadIcon} from './Icons';

/** Read-only reload: parent owns the request and its exact identity. */
export function ConversationReload({onRetry, busy = false, failed = false, zh, testID = 'chat-read-retry', noteColor, accessibilityLabel}:  {onRetry: () => void; busy?: boolean; failed?: boolean; zh: boolean; testID?: string; noteColor?: string; accessibilityLabel?: string}) {
 const pal = useReadingPalette();
 const styles = React.useMemo(() => makeStyles(pal), [pal]);
 return <View style={styles.container}>
  <TouchableOpacity testID={testID} accessibilityRole="button" accessibilityState={{busy, disabled: busy}}
   accessibilityLabel={accessibilityLabel ?? (zh ? '重新加载对话' : 'Reload conversation')} disabled={busy} onPress={busy ? undefined : onRetry} style={styles.button}>
   {busy ? <LoadingMark size={15} color={pal.fg2} accessible={false} /> : <ReloadIcon size={16} color={readingColors(pal).link} />}
   <Text style={styles.label}>{busy ? (zh ? '正在重新加载…' : 'Reloading…') : (zh ? '重试' : 'Retry')}</Text>
  </TouchableOpacity>
  {failed && !busy && <Text accessibilityLiveRegion="polite" style={[styles.note, noteColor ? {color: noteColor} : undefined]}>{zh ? '仍无法加载，请稍后重试。' : 'Still unable to load. Try again shortly.'}</Text>}
 </View>;
}
const makeStyles = (pal: Palette) => StyleSheet.create({
 container:{alignItems:'flex-start',gap:6}, button:{minHeight:44,paddingHorizontal:13,paddingVertical:10,borderRadius:9,borderWidth:StyleSheet.hairlineWidth,borderColor:pal.divider,backgroundColor:pal.raised,flexDirection:'row',alignItems:'center',justifyContent:'center',gap:7},
 label:{fontSize:13,fontWeight:'600',color:pal.fg},note:{fontSize:12,lineHeight:18,color:pal.fg2},
});
