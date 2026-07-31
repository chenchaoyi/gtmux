#!/bin/bash
# verify-server-mode.sh — 验证 openspec change `server-mode` 的阻断性前提(tasks.md §0)
#
# 只做验证、不改仓库、不装任何东西。唯一会改动系统的是 `arm`,它:
#   1. 执行 `pmset -c disablesleep 1`。【2026-07-31 实测:-c 并不被尊重 —— 设置落在
#      plist 的 SystemPowerSettings 顶层,是【全局】的,电池供电时同样生效。所以
#      "拔电由内核自动恢复睡眠"这条路走不通,兜底/守护是唯一防线。】
#   2. 同时启动一个 30 分钟后自动恢复的 root 兜底进程 —— 万一你忘了 disarm,半小时后自动还原。
#      (这个兜底本身就是设计里 "de-escalation-only guard" 思路的最小实证,现在它是必需品。)
#
# 用法:
#   bash verify-server-mode.sh check          # 只读,随时可跑。也会警告"当前是否被留在开启态"
#   bash verify-server-mode.sh test-assert    # 0.0 先证明 root 是必需的(无 sudo)
#   bash verify-server-mode.sh arm            # 0.1/0.2 开启(需要 sudo)+ 30min 自动兜底
#   bash verify-server-mode.sh readback       # 0.3 采集"开启态"的 readback fixture
#   bash verify-server-mode.sh disarm         # 立即恢复(需要 sudo)—— 测完务必跑
#
#   bash verify-server-mode.sh watch          # 合盖【前】起记录器(不需要第二台设备)
#   bash verify-server-mode.sh verdict        # 开盖【后】出判定:睡了/没睡,服务是否连续
set -uo pipefail

PMSET=/usr/bin/pmset
PLIST=/Library/Preferences/com.apple.PowerManagement.plist
GRACE=1800   # 兜底恢复的秒数

# 会改动系统 / 会阻塞的步骤必须有真人在场:非交互执行(CI、agent、管道)直接拒绝,
# 否则 `read` 会从 /dev/null 立刻返回,确认提示形同虚设。(自测时真踩到过。)
require_tty() {
  [ -t 0 ] || { printf '\033[31m拒绝:%s 需要交互终端(有真人确认)。\033[0m\n' "$1"; exit 2; }
}

hr() { printf '\n\033[1m── %s\033[0m\n' "$1"; }
warn() { printf '\033[31m%s\033[0m\n' "$1"; }
ok() { printf '\033[32m%s\033[0m\n' "$1"; }

# 读回状态。【2026-07-31 实测,三条路径两次纠错才找到权威源】
#   · `pmset -g` / `-g custom` / `-g live` —— 永远不显示 disablesleep,不论开关。
#     用它判断会在机器实际【不会睡】时报"关闭态":本功能最坏的失败模式。
#   · 磁盘 plist(SystemPowerSettings.SleepDisabled)—— 能看到,但【写盘有延迟】:
#     disarm 之后 1 秒去读仍是旧值,于是误报"恢复失败"。而且它表达的是【持久化设置】,
#     不是此刻内核的行为。
#   · ioreg 的 IOPMrootDomain.SleepDisabled —— 内核活状态,实时、免 root。← 唯一正确判据
# 注意 plist 关闭后是 `=> false`(key 仍在),不能用"key 是否存在"判断。
sleep_disabled() {
  ioreg -r -c IOPMrootDomain -d 1 -w0 2>/dev/null \
    | LC_ALL=C grep -qE '"SleepDisabled"[[:space:]]*=[[:space:]]*Yes'
}

cmd_check() {
  hr "环境"
  sw_vers | sed 's/^/  /'
  printf '  arch: %s\n' "$(uname -m)"
  printf '  电源: %s\n' "$($PMSET -g ps | head -1)"
  $PMSET -g ps | sed -n '2p' | sed 's/^/  电量: /'

  hr "disablesleep 当前状态"
  if sleep_disabled; then
    warn "  ⚠️  当前是【开启】态 —— 这台 Mac 现在不会睡。"
    warn "  ⚠️  如果不是你正在做测试,请立刻跑: bash $0 disarm"
    echo "  真相源(pmset 看不到它): $PLIST → SystemPowerSettings.SleepDisabled = true"
  else
    ok "  关闭(正常)。注意:关闭态在 pmset -g 里【完全没有对应行】,这是设计文档里说的『关闭态不可见』。"
  fi

  hr "setting 名是否仍被识别(非 root 探测,不改任何东西)"
  printf '  pmset -c disablesleep 1  → %s\n' "$($PMSET -c disablesleep 1 2>&1 | head -1)"
  printf '  pmset -a 胡编的名字      → %s\n' "$($PMSET -a totallybogussetting 1 2>&1 | head -1)"
  echo   "  (前者走到『must be run as root』= 参数被接受;后者吐 Usage = 探测有区分度)"

  hr "当前持有的 sleep assertion(说明为什么 assertion 不够)"
  $PMSET -g assertions 2>/dev/null | sed -n '2,12p' | sed 's/^/  /'

  hr "gtmux serve / tunnel 是否在跑(合盖测试要靠它验证)"
  pgrep -fl 'gtmux serve' | sed 's/^/  /' || echo "  (没有 gtmux serve 在跑 —— 合盖测试需要它)"
  [ -f "$HOME/.config/gtmux/tunnel-url" ] && printf '  tunnel: %s\n' "$(cat "$HOME/.config/gtmux/tunnel-url")" || echo "  (没有 tunnel-url)"
}

cmd_test_assert() {
  require_tty "test-assert"
  hr "0.0 —— 先证明 root 是必需的(无特权路径能不能扛住合盖)"
  cat <<EOF
  这一步先排除"根本不需要 root"的可能。如果 caffeinate 就能扛住合盖,
  整个特权设计都不必要 —— 那是最好的结果,必须先排除。

  步骤(不需要第二台设备):
    1. 本窗口会起一个 caffeinate -dis(阻止 idle / 显示 / 系统 idle 睡眠)
    2. 另开一个窗口跑:  bash $0 watch
    3. 合上盖子,等 2 分钟以上,再打开
    4. 回到第二个窗口跑: bash $0 verdict   ← 它直接给出"睡了/没睡"
    5. 回本窗口 Ctrl-C 结束 caffeinate

  预期(研究文档 §2):合盖后仍然会睡 —— assertion 不参与合盖这条硬件路径。
  如果它【没睡】:停下来告诉设计方,方案可以大幅简化。
EOF
  printf '\n  按 Enter 开始,Ctrl-C 结束…'; read -r _
  caffeinate -dis
}

cmd_arm() {
  require_tty "arm"
  if ! $PMSET -g ps | head -1 | grep -q 'AC Power'; then
    warn "拒绝:请先插上电源。测试流程是【AC 下开启 → 再拔电】,这样才分得清是哪个因素。"; exit 1
  fi
  hr "0.1 / 0.2 —— 开启 disablesleep + ${GRACE}s 自动兜底"
  echo "  即将执行(需要输入你的管理员密码):"
  echo "    sudo pmset -c disablesleep 1"
  echo "    并在后台启动:${GRACE}s 后自动 pmset -a disablesleep 0"
  printf '\n  按 Enter 继续,Ctrl-C 放弃…'; read -r _

  sudo bash -c "$PMSET -c disablesleep 1; ( sleep $GRACE; $PMSET -a disablesleep 0 ) </dev/null >/dev/null 2>&1 &" || {
    warn "失败 —— 注意 pmset 即使拒绝也可能 exit 0,以下面的 readback 为准"; }

  hr "readback(内核活状态 —— 不要看退出码,pmset 也永远不显示这一项)"
  if sleep_disabled; then
    ok "  ✅ 已开启:ioreg 的 IOPMrootDomain.SleepDisabled = Yes"
  else
    warn "  ❌ 内核里没有生效(ioreg 读到 No)—— 请把上面 sudo 的报错贴回给设计方。"
    exit 1
  fi

  cat <<EOF

  接下来 —— 0.2b「换会议室」测试(A1 已于 2026-07-31 确认成立,这次测电池那半):
    1. 另开一个窗口起记录器:  bash $0 watch
       (它会打印"服务器模式:【已开启】",确认这次算正式测试)
    2. 【顺序要紧】先【拔掉电源】,再【合上盖子】
    3. 合着等 2~3 分钟,然后开盖、插回电源
    4. 出判定:  bash $0 verdict
       → 全程醒着 = 【A2b 成立】,拔电合盖照样干活,换会议室的场景可以交付
       → 睡了     = 【A2b 不成立】,服务器模式只能是插电特性
    5. 测完务必: bash $0 disarm
  兜底:即使你什么都不做,${GRACE}s 后 root 后台进程会自动恢复,电池耗不掉。
EOF
}

cmd_readback() {
  hr "0.3 —— 采集 readback fixture(把下面整段贴回给设计方)"
  echo "### pmset -g";        $PMSET -g
  echo "### pmset -g custom"; $PMSET -g custom
  echo "### plist";           plutil -p "$PLIST" 2>&1
  echo "### state";           sleep_disabled && echo "ON" || echo "OFF"
}

# 证据必须活过它要测的那些事件 —— 睡眠、唤醒、乃至重启。
# 【教训:曾经放在 $TMPDIR,一次重启就抹掉了整场合盖测试的唯一产物。】
VDIR="${HOME}/.local/state/gtmux"; mkdir -p "$VDIR" 2>/dev/null
LOG="$VDIR/servermode-verify.log"
PIDF="$VDIR/servermode-verify.pid"
OUT="$VDIR/servermode-verdict.txt"
HEALTH="http://127.0.0.1:8765/api/health"

# 重启会把 "Sleep/Wakes since boot" 计数器清零 → 计数判据失效。
# 记下 boot 时刻,verdict 才能识别"测试期间重启过"并改用日志断档判定。
# 注意 `.*sec = ` 会【贪婪】匹配到 "usec = ",抓回 usec(每次都不同)→ 每次都误报重启。
# 必须锚定行首的 "{ sec =".
bootstamp() { sysctl -n kern.boottime 2>/dev/null | sed 's/^{ sec = \([0-9]*\).*/\1/'; }

# 内核自己记的睡眠/唤醒总次数 —— 合盖前后一比就是权威判据,不用解析日志文本。
# LC_ALL=C 是必须的:`pmset -g log` 里混着设备名等非 UTF-8 字节(实测撞到过蓝牙触控板
# 的事件行),UTF-8 locale 下 awk 会 "towc: multibyte conversion failure" 直接崩掉,
# 于是基线取不到、整个判据静默失效。按字节处理才稳。
sleepwakes() {
  $PMSET -g log 2>/dev/null | LC_ALL=C awk '/Total Sleep\/Wakes since boot/{n=$NF} END{split(n,a,":"); print a[2]+0}'
}

# 停记录器。绝不裸 kill 文件里记的 pid ——【曾经真的关掉过用户的终端窗口】:
# 合盖期间记录器被 SIGHUP 杀掉,pid 被系统回收复用,kill 就打中了无辜进程。
# 所以 kill 前必须确认这个 pid 现在【仍然是】我们自己的记录器。
stop_recorder() {
  [ -f "$PIDF" ] || return 0
  local rpid; rpid=$(cat "$PIDF" 2>/dev/null)
  rm -f "$PIDF"
  case "$rpid" in ''|*[!0-9]*) return 0 ;; esac          # 空/非数字 → 不动
  [ "$rpid" -le 1 ] && return 0                           # 绝不碰 init
  # ps -p 是单进程查询(不是全表扫描,躲开 EDR 卡死全表 ps 的老坑)
  if ps -p "$rpid" -o command= 2>/dev/null | grep -q 'verify-premise'; then
    kill "$rpid" 2>/dev/null
  fi
}

# 电源现状:ac | batt
powersrc() { $PMSET -g ps 2>/dev/null | head -1 | grep -q 'AC Power' && echo ac || echo batt; }

cmd_watch() {
  stop_recorder
  local base armed; base=$(sleepwakes)
  # 【防呆】把"这次测的是哪一档"在起点就钉死,而不是等 verdict 再让人二选一 ——
  # 曾经因为忘了先 arm,拿"没开服务器模式"的合盖数据当成了正式测试。
  if sleep_disabled; then armed=yes; else armed=no; fi
  { echo "# base_sleepwakes=$base boot=$(bootstamp) armed=$armed start=$(date '+%F %T')"
    while :; do
      c=$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "$HEALTH" 2>/dev/null || echo 000)
      echo "$(date +%s) $(date '+%T') health=$c power=$(powersrc)"
      sleep 5
    done
  } > "$LOG" 2>&1 &
  echo $! > "$PIDF"
  hr "记录器已启动(每 5 秒一行:时间戳 + 健康码 + 电源)"
  echo "  日志: $LOG"
  echo "  基线 Sleep/Wake 次数: $base"
  if [ "$armed" = yes ]; then
    ok "  服务器模式:【已开启】⇒ 这次测的是 0.1 / 0.2b(clamshell 档)"
  else
    warn "  服务器模式:【未开启】⇒ 这次只能当 0.0 对照测试"
    warn "  如果你想测「开着服务器模式合盖」,先 Ctrl-C,跑 arm,再重来。"
  fi
  cat <<EOF

  现在:合上盖子 → 等 2 分钟以上 → 打开盖子 → 回来跑:
      bash $0 verdict
  判定逻辑(两个独立信号必须一致):
    · Sleep/Wake 计数器有没有涨   → 机器到底睡没睡(内核说了算)
    · 日志时间戳有没有断档        → 用户态是否被冻住
    · health 是不是全程 200       → serve 在合盖期间有没有继续服务
EOF
}

cmd_verdict() {
  [ -f "$LOG" ] || { warn "没有记录器日志 —— 先跑: bash $0 watch"; exit 1; }
  # 注意顺序:先算、先【打印结论】,最后才去停记录器。
  # 结论是这趟测试唯一的产出,绝不能被任何收尾动作挡在前面(曾经就是这么丢的)。
  local base now delta gap bad total oldboot rebooted=0 armed sawbatt
  base=$(awk -F'base_sleepwakes=' '/base_sleepwakes/{split($2,a," "); print a[1]}' "$LOG")
  oldboot=$(awk -F'boot=' '/boot=/{split($2,a," "); print a[1]}' "$LOG")
  armed=$(awk -F'armed=' '/armed=/{split($2,a," "); print a[1]}' "$LOG"); armed=${armed:-unknown}
  sawbatt=$(grep -c 'power=batt' "$LOG")
  now=$(sleepwakes); delta=$(( now - base ))
  gap=$(awk '$1 ~ /^[0-9]+$/ {if(p){d=$1-p; if(d>m) m=d} p=$1} END{print m+0}' "$LOG")
  bad=$(grep -c 'health=[^2]' "$LOG"); total=$(grep -c 'health=' "$LOG")
  [ -n "$oldboot" ] && [ "$oldboot" != "$(bootstamp)" ] && rebooted=1

  hr "判定"
  if [ "$rebooted" = 1 ]; then
    warn "  ⚠️ 测试期间这台机器【重启过】—— Sleep/Wake 计数器已清零,该判据作废。"
    warn "     只看日志断档;若断档也被重启截断,这次测试无效,请重跑。"
  fi
  printf '  Sleep/Wake 计数: %s → %s  (增加 %s)\n' "$base" "$now" "$delta"
  printf '  日志最大断档:   %s 秒  (正常应 ≈5)\n' "$gap"
  printf '  health 非 200:  %s / %s 次\n' "$bad" "$total"
  printf '  测试开始时服务器模式: %s%s\n' "$armed" \
    "$([ "$sawbatt" -gt 0 ] && echo "   ·  期间用过电池(${sawbatt} 次采样)" || echo "   ·  全程 AC")"
  echo
  if [ "$rebooted" = 1 ] && [ "$gap" -le 30 ]; then
    warn "  ⇒ 无效:重启把证据截断了,没有可信结论 —— 请重跑 watch + 合盖。"
  elif [ "$delta" -gt 0 ] || [ "$gap" -gt 30 ]; then
    warn "  ⇒ 机器【睡了】(计数器上涨 或 出现长断档)"
    case "$armed" in
      no)  echo "     ⇒ 服务器模式当时【没开】,这是 0.0 对照:符合预期,assertion 撑不住合盖,root 必需。" ;;
      yes) if [ "$sawbatt" -gt 0 ]; then
             warn "     ⇒ 服务器模式开着、且期间在电池上 ⇒ 【A2b 不成立】:拔电后合盖撑不住,"
             warn "        换会议室的场景用这个机制交付不了 —— 服务器模式只能是插电特性。"
           else
             warn "     ⇒ 服务器模式开着、全程 AC 却仍睡了 ⇒ 【A1 不成立】,停下来重新评估。"
           fi ;;
      *)   warn "     ⇒ 日志没记 armed 状态(旧格式),无法判断测的是哪一档 —— 请重跑。" ;;
    esac
  elif [ "$bad" -gt 0 ]; then
    warn "  ⇒ 机器没睡,但 serve 有 $bad 次没答 —— 记下来,这是另一个问题(不是 A1 的结论)"
  else
    ok   "  ⇒ 机器【全程醒着】且 serve 全程在服务"
    case "$armed" in
      yes) if [ "$sawbatt" -gt 0 ]; then
             ok   "     ⇒ 服务器模式开着、期间在电池上、仍不睡 ⇒ 【A2b 成立】:"
             ok   "        换会议室的场景可以交付。护栏改用电量下限(design §2.3)。"
           else
             ok   "     ⇒ 服务器模式开着(全程 AC)⇒ 【A1 成立】。电池那半还要单独测(0.2b)。"
           fi ;;
      no)  warn "     ⇒ 但服务器模式当时【没开】却也没睡 —— 很可能盖子没真的合上,或合盖时间太短。请重测。" ;;
      *)   warn "     ⇒ 日志没记 armed 状态(旧格式),无法判断测的是哪一档 —— 请重跑。" ;;
    esac
  fi
  echo; echo "  日志留档: $LOG"
  stop_recorder                     # 收尾放最后:结论已经落地了,停不掉也不影响判定
  hr "顺带:当前 disablesleep 状态"
  if sleep_disabled; then
    warn "  ⚠️ 仍是【开启】态 —— 测完请跑: bash $0 disarm"
  else
    ok "  关闭态,机器会正常睡眠。"
  fi
}

cmd_disarm() {
  hr "恢复睡眠"
  sudo $PMSET -a disablesleep 0
  sleep 1
  if sleep_disabled; then warn "  ❌ 仍是开启态!手动跑: sudo pmset -a disablesleep 0"; exit 1
  else ok "  ✅ 已恢复,这台 Mac 会正常睡眠了。"; fi
}

case "${1:-check}" in
  check)       cmd_check ;;
  test-assert) cmd_test_assert ;;
  arm)         cmd_arm ;;
  readback)    cmd_readback ;;
  watch)       cmd_watch ;;
  verdict)     cmd_verdict 2>&1 | tee "$OUT"; printf '\n  (结论已落盘,窗口没了也能回看: %s)\n' "$OUT" ;;
  disarm)      cmd_disarm ;;
  *) echo "用法: bash $0 [check|test-assert|arm|watch|verdict|readback|disarm]"; exit 2 ;;
esac
