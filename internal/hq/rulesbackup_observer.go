package hq

// Nil in production; tests use this synchronous boundary to replace a backup
// path after creation without a timing-dependent filesystem watcher.
var hqBackupWritten func(string)
