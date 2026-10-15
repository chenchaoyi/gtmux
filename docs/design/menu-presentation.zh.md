# 应用主动切换焦点时的菜单行为

普通菜单仍按系统临时弹出层处理。只有从菜单启动 Restore、自动展示更新内容时，
`MenuPopoverPresentation` 才保护当次展示，避免终端或阅读窗口取得焦点后菜单消失。
原有视图和 AgentStore 继续负责数据、恢复进度，不重复发起恢复或查询。

Restore 在启动 CLI 前保护菜单。恢复尚未结束时重新打开菜单，也能保留进度。
恢复结束只更新原有内容，不重新弹出菜单、不抢终端焦点。保护持续到用户关闭菜单，
因此终端在前台时也能看到恢复结果。

自动更新说明在菜单打开后展示，空间允许时放在菜单旁边；点击两个窗口都不会收起菜单。
屏幕横向空间不足时，窗口仍可按正常方式移动和访问。菜单关闭后才返回的更新说明留到下次打开。
手动查看更新、偏好设置、配对及跳转等明确导航仍按原有规则先收起菜单。

保护中的菜单可通过点击外部、在菜单内按 Escape、再次点击菜单栏图标或明确导航关闭。
只全局观察鼠标点击，不监听其他应用的按键，也不新增权限。除菜单内 Escape 外，
本地事件继续正常传递。阅读窗口及其子窗口不算外部。菜单关闭后移除监听，恢复默认行为。
只用键盘切换应用时，保护中的菜单可能继续显示；点击外部或明确关闭即可收起。
这只影响当次展示，不是全局常驻设置。

测试使用模拟 AppKit 窗口，覆盖生命周期、外部点击、阅读窗口、Escape 和迟到回调。
隔离预览不得查询真实 CLI、恢复真实会话或修改个人更新阅读记录。

接口依据：[AppKit 弹出层行为](https://developer.apple.com/documentation/appkit/nspopover/behavior-swift.enum/applicationdefined)
与[全局鼠标事件观察](https://developer.apple.com/documentation/appkit/nsevent/addglobalmonitorforevents(matching:handler:))。
