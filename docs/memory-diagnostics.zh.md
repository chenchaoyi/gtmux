# 本机 Go 内存诊断

[English](memory-diagnostics.md)

`gtmux serve --memory-diagnostics DIR` 为这一个前台进程增加私有的本机观测 socket。
默认关闭，不增加监听或定时采样；不能与 `--service` / `--unservice` 合用。
现有托管服务需要经过审阅的进程切换才能启用；新进程无法追溯旧进程的堆。
这是定位能力，不代表内存缺陷已经修复。

## 启动与采集

指定已经存在、属于服务用户的绝对路径目录，权限必须是 0700，末级不能是符号链接，
父目录不能允许组或其他用户写入（sticky 临时目录除外）。路径保持简短，macOS 尤其有
Unix socket 路径长度限制。下面仅用于**已选好绑定地址和端口的隔离前台服务**：

```sh
umask 077
memory_dir=$(mktemp -d /tmp/gtmux-memory.XXXXXX)
gtmux serve --bind 127.0.0.1 --port 18765 --memory-diagnostics "$memory_dir"
```

启动信息会显示 `DIR/mem-PID-RANDOM/memory.sock`；子目录为 0700，socket 为 0600。
主 HTTP 服务的地址和端口保持原配置；手机/网页 API 没有内存路由，也没有 TCP 调试监听，
不会向默认 HTTP mux 注册接口。目录不安全或监听失败时打印警告，关闭诊断，主服务仍可启动。
这些权限隔离其他普通用户，不防同一账户/root；权限位检查不等于 ACL 审核。目录不应继承
额外 ACL 授权。profile 保存抽样分配栈、符号和源码路径，不含对象正文；仍需仅留本机，
不要放进 Git、消息或上传文件。

另一终端使用启动信息中的准确路径，将证据保存在私有目录，使用私有 umask 和唯一文件名：

```sh
umask 077
memory_socket=/absolute/private/directory/mem-PID-RANDOM/memory.sock
capture_dir=$(mktemp -d /tmp/gtmux-capture.XXXXXX)
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   http://localhost/stats > "$capture_dir/stats-before.json"
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   -D "$capture_dir/heap.headers" http://localhost/heap > "$capture_dir/heap.pprof"
# 任何下一份 profile 都至少间隔五秒。
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   -D "$capture_dir/allocs.headers" http://localhost/allocs > "$capture_dir/allocs.pprof"
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   http://localhost/stats > "$capture_dir/stats-after.json"
go tool pprof -top -sample_index=inuse_space "$capture_dir/heap.pprof"
go tool pprof -top -sample_index=alloc_space "$capture_dir/allocs.pprof"
```

逐条检查 curl 退出码和 HTTP 状态后，才能把文件当成有效证据。服务自身不会保存 profile。
JSON 计数包括 `PID`、`Version`、`GoVersion`、`StartedAt`（诊断监听启动时间）、
`CapturedAt`、`HeapAlloc/HeapInuse/HeapIdle/HeapReleased/HeapSys/Sys`、`TotalAlloc`、
`HeapObjects/Mallocs/Frees/NextGC/LastGC`、栈及运行时元数据大小、`NumGC/NumForcedGC`、
`NumGoroutine`、`MemProfileRate`。大小单位是字节，`LastGC` 是 Unix 纳秒。
profile 的响应头包含 `X-Gtmux-PID`、`X-Gtmux-Version`、`X-Gtmux-Captured-At`、
`X-Gtmux-NumGC`，应与 profile 一起保留。

## 不改变负载的判读方式

[Go runtime 官方文档](https://pkg.go.dev/runtime#MemStats)定义各项计数。
`HeapAlloc` 包括仍可达的对象和尚未清扫的垃圾；`HeapInuse - HeapAlloc` 估计大小类别碎片；
`HeapIdle - HeapReleased` 表示运行时保留的闲置堆；`HeapSys` 是堆预留地址空间，不是物理驻留内存。
`Sys` 还包括栈和运行时元数据。需同时记录 OS 的 RSS、footprint、压缩内存，它们都不能单独等同于 Go 活堆。

[Go pprof 官方文档](https://pkg.go.dev/runtime/pprof#Profile)说明 heap 抽样与已完成 GC 的关系；
底层 [MemProfile API](https://pkg.go.dev/runtime#MemProfile)可能滞后两个 GC 周期。
`alloc_space` 是累计分配，`inuse_space` 是该采样回收视图中的保留量估计。
累计分配高不等于泄漏。对同一进程连续比较，保留 GC 计数，等待自然 GC 后再判断哪些分配栈持续保留。
用有界、已核实的只读工作负载关联增长栈和对象生命周期。强制 GC 对照只能在隔离夹具中进行，
必须标明干预，不能代替生产原始状态。

## 上限与清理

仅接受无查询参数、无正文的 GET `/stats`、`/heap`、`/allocs`。
不提供 GC、goroutine、trace、CPU profile 或命令行接口，不更改 GC/采样设置。
同时只执行一次读取，忙时返回 429，不排队；profile 共享五秒最小准入间隔。
预检抽样记录超过 16384 条或编码输出超过 8 MiB 返回 413；失败时不会返回伪成功的残缺 profile。
当前读取结束后仍可读取计数，即使 profile 超预算。

这些是准入/输出上限，不是 Go 内部临时内存或执行时间的硬上限：预检后进程仍可能分配，
`runtime/pprof` 内部采集也不可取消。取消会丢弃输出，采集结束前仍保持忙状态。
HTTP 超时限制 socket I/O，不限制运行时采集工作。`ReadMemStats` 也需短暂协调运行时；
建议稀疏采集，例如计数每 15 秒一次，profile 间隔数分钟。

正常返回或已处理的 SIGINT/SIGTERM 会关闭监听，仅移除本次 socket 和运行子目录；
调用方目录和已导出文件保留。路径被替换的文件不会删除。SIGKILL/崩溃可能留下旧运行目录；
下次始终创建新目录。确认对应进程已退出且证据无需保留后，才可清理旧目录。
