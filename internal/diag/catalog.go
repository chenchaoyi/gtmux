package diag

// Catalog is every action gtmux records, with the commands whose run records it. It is
// the contract for `gtmux logs --event` and for a bug report that names an event: the
// event name is stable, the message beside it is not.
//
// Three tests hold it to the code. Each command the command table marks as writing must
// appear here (a writing command with no act is a change nothing records), each event
// here must be written somewhere in the source, and each act the source writes must be
// listed here. docs/cli.md renders it.
//
// "serve" stands for what serve does for a phone, a browser, a share link or the local
// CLI; "hook" for what the agent hook does on its own; "app" for the menu bar app, which
// writes to the same store from Swift (macapp's DiagLog).
var Catalog = []CatalogEntry{
	{"act.adopt", []string{"adopt"}},
	{"act.app.launch", []string{"app"}},
	{"act.attach", []string{"attach", "serve"}},
	{"act.awake.off", []string{"awake"}},
	{"act.awake.on", []string{"awake"}},
	{"act.capture", []string{"capture"}},
	{"act.cleanup", []string{"doctor", "serve"}},
	{"act.config.set", []string{"config", "quiet"}},
	{"act.doctor.bundle", []string{"doctor"}},
	{"act.doctor.fix", []string{"doctor"}},
	{"act.focus", []string{"focus", "serve"}},
	{"act.hq.export", []string{"hq"}},
	{"act.hq.import", []string{"hq"}},
	{"act.hq.rotate", []string{"hq"}},
	{"act.hq.start", []string{"hq"}},
	{"act.install.app", []string{"install"}},
	{"act.install.hooks", []string{"install"}},
	{"act.knowledge", []string{"knowledge", "serve"}},
	{"act.knowledge.sync", []string{"knowledge", "doctor"}},
	{"act.mint", []string{"pair", "serve"}},
	{"act.narrow", []string{"serve"}},
	{"act.new", []string{"new"}},
	{"act.notify", []string{"hook"}},
	{"act.notify.post", []string{"app"}},
	{"act.pair", []string{"serve"}},
	{"act.push.forget", []string{"devices", "serve"}},
	{"act.push.register", []string{"serve"}},
	{"act.reap", []string{"reap"}},
	{"act.reap.snooze", []string{"reap"}},
	{"act.restore", []string{"restore"}},
	{"act.resume", []string{"restore"}},
	{"act.revoke", []string{"pair", "devices", "share", "serve"}},
	{"act.send", []string{"send", "serve"}},
	{"act.share.config", []string{"share", "serve"}},
	{"act.share.create", []string{"share", "serve"}},
	{"act.share.set", []string{"share", "serve"}},
	{"act.spawn", []string{"spawn"}},
	{"act.tunnel.off", []string{"tunnel"}},
	{"act.tunnel.on", []string{"tunnel"}},
	{"act.tunnel.redeem", []string{"tunnel"}},
	{"act.uninstall.app", []string{"uninstall"}},
	{"act.uninstall.hooks", []string{"uninstall"}},
	{"act.unwatch", []string{"panes"}},
	{"act.update", []string{"update"}},
	{"act.upload", []string{"serve"}},
	{"act.wake.delivered", []string{"serve", "hook"}},
	{"act.wake.dropped", []string{"serve", "hook"}},
	{"act.watch", []string{"panes"}},
}

// A CatalogEntry is one action and the commands that record it.
type CatalogEntry struct {
	Event    string
	Commands []string
}
