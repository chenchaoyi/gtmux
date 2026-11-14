import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';

/** Read-only reload: parent owns the request and its exact identity. */
export function ConversationReload({onRetry, busy = false, failed = false, zh, testID = 'chat-read-retry', noteColor}:  {onRetry: () => void; busy?: boolean; failed?: boolean; zh: boolean; testID?: string; noteColor?: string}) {
 return <View style={styles.container}>
  <TouchableOpacity testID={testID} accessibilityRole="button" accessibilityState={{busy, disabled: busy}}
   accessibilityLabel={zh ? '重新加载对话' : 'Reload conversation'} disabled={busy} onPress={busy ? undefined : onRetry} style={styles.button}>
   <Text style={styles.label}>{busy ? (zh ? '正在重新加载…' : 'Reloading…') : (zh ? '重新加载对话' : 'Reload conversation')}</Text>
  </TouchableOpacity>
  {failed && !busy && <Text accessibilityLiveRegion="polite" style={[styles.note, noteColor ? {color: noteColor} : undefined]}>{zh ? '仍无法加载，请检查连接后重试。' : 'Still unable to load. Check the connection and try again.'}</Text>}
 </View>;
}
const styles = StyleSheet.create({
 container:{alignItems:'center',gap:6}, button:{minHeight:44,paddingHorizontal:16,paddingVertical:11,borderRadius:10,borderWidth:1,borderColor:'#72727E',backgroundColor:'#2C2C32',justifyContent:'center'},
 label:{fontSize:14,fontWeight:'600',color:'#F2F2F7'},note:{fontSize:12,lineHeight:18,color:'#B8B8C2',textAlign:'center'},
});
