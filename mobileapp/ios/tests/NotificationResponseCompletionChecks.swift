import Foundation

@main struct NotificationResponseCompletionChecks {
  static func main() {
    var called: [String] = []
    let first = NotificationResponseCompletions.retain(id: "one", action: "2") { called.append("one") }
    let duplicate = NotificationResponseCompletions.retain(id: "one", action: "2") { called.append("duplicate") }
    let other = NotificationResponseCompletions.retain(id: "two", action: "1") { called.append("two") }
    precondition(called.isEmpty)
    NotificationResponseCompletions.finish(id: "one", action: "1")
    precondition(called.isEmpty)
    NotificationResponseCompletions.finish(id: "one", action: "2")
    precondition(Set(called) == Set(["one", "duplicate"]))
    NotificationResponseCompletions.expire(token: first)
    NotificationResponseCompletions.expire(token: duplicate)
    precondition(called.count == 2)
    NotificationResponseCompletions.expire(token: other)
    NotificationResponseCompletions.finish(id: "two", action: "1")
    precondition(called.count == 3 && called.last == "two")
    print("notification response completion checks passed")
  }
}
