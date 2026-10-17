import SwiftUI

struct AccessHistoryHelp: View {
    let zh: Bool
    let retainDays: Int
    @State private var presented = false
    static func rules(zh: Bool, days: Int) -> [(String, String)] {
        zh ? [
            ("记录范围", "手机、浏览器等外部客户端对这台服务器的访问。本机工具的内部请求不在此列表；无法确认来源的记录保留在诊断日志中。"),
            ("记录方式", "相同来源、操作和目标的成功读取，5 分钟内最多记录一次。修改操作、档案下载和已认证请求的失败分别记录；未认证访问每分钟最多记录一次。"),
            ("请求结果", "“已接受”表示服务器已受理请求。任务进展请查看对应会话。"),
            ("保留时间", "最多保留 \(days) 天；日志达到容量上限时可能提前清理。")
        ] : [
            ("Coverage", "Activity from external clients such as phones and browsers. Internal tools are excluded from this list; records with unidentified origins remain in diagnostics."),
            ("Recording", "Successful reads with the same source, operation and target are recorded at most once every five minutes. Changes, archive downloads and authenticated failures are recorded separately. Unauthenticated access is recorded at most once per minute."),
            ("Results", "Accepted means the server handled the request. Check the corresponding session for task progress."),
            ("Retention", "Retained for up to \(days) days. Capacity limits may remove older records sooner.")
        ]
    }
    var body: some View {
        Button { presented = true } label: {
            Label(zh ? "记录规则" : "Recording rules", systemImage: "info.circle")
        }
        .accessibilityLabel(zh ? "查看访问记录规则" : "About access recording")
        .accessibilityIdentifier("access-history-rules")
        .popover(isPresented: $presented, arrowEdge: .bottom) {
            VStack(alignment: .leading, spacing: 14) {
                Text(zh ? "关于访问记录" : "About access records").font(Theme.Font.session)
                    .accessibilityAddTraits(.isHeader)
                ForEach(Self.rules(zh: zh, days: retainDays), id: \.0) { title, text in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(title).font(Theme.Font.task.weight(.semibold))
                        Text(text).font(Theme.Font.task).fixedSize(horizontal: false, vertical: true).textSelection(.enabled)
                    }
                }
                HStack { Spacer(); Button(zh ? "完成" : "Done") { presented = false }.keyboardShortcut(.cancelAction) }
            }.padding(16).frame(width: 360)
        }
    }
}
