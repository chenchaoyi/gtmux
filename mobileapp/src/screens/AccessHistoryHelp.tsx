import React from 'react';
import {Alert, StyleSheet, Text, TouchableOpacity, View} from 'react-native';

export function accessHistoryRules(zh: boolean, days: number): string {
  return zh
    ? `记录手机、浏览器等外部客户端的访问。本机工具的内部请求不在此列表；无法确认来源的记录保留在诊断日志中。\n\n相同来源、操作和目标的成功读取，5 分钟内最多记录一次。修改操作、档案下载和已认证请求的失败分别记录；未认证访问每分钟最多记录一次。\n\n“已接受”表示服务器已受理请求。任务进展请查看对应会话。\n\n最多保留 ${days} 天；日志达到容量上限时可能提前清理。`
    : `Activity from external clients such as phones and browsers. Internal tools are excluded from this list; records with unidentified origins remain in diagnostics.\n\nSuccessful reads with the same source, operation and target are recorded at most once every five minutes. Changes, archive downloads and authenticated failures are recorded separately. Unauthenticated access is recorded at most once per minute.\n\nAccepted means the server handled the request. Check the corresponding session for task progress.\n\nRetained for up to ${days} days. Capacity limits may remove older records sooner.`;
}

export function AccessHistoryHelp({zh, days, color}: {zh: boolean; days: number; color: string}) {
  return <View style={styles.row}>
    <Text style={[styles.note, {color}]}>{zh ? `本地时间 · 最多保留 ${days} 天` : `Local time · Up to ${days} days`}</Text>
    <TouchableOpacity accessibilityRole="button" accessibilityLabel={zh ? '查看访问记录规则' : 'About access recording'}
      onPress={() => Alert.alert(zh ? '关于访问记录' : 'About access records', accessHistoryRules(zh, days), [{text: zh ? '完成' : 'Done'}])}
      style={styles.button}>
      <Text style={[styles.note, {color}]}>{zh ? '记录规则' : 'Recording rules'}</Text>
    </TouchableOpacity>
  </View>;
}
const styles = StyleSheet.create({
  row: {flexDirection: 'row', alignItems: 'center', flexWrap: 'wrap', justifyContent: 'space-between', gap: 8},
  note: {fontSize: 14, lineHeight: 20},
  button: {minWidth: 44, minHeight: 44, justifyContent: 'center'},
});
