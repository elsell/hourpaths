import Foundation
import SQLite3

@main
struct NativeTimerTransactionAcceptance {
  static func main() throws {
    let root = URL(fileURLWithPath: CommandLine.arguments[1])
    let script = try String(contentsOf: root.appendingPathComponent("apps/mobile/modules/hourpaths-timers/ios/TimerDomain.js"), encoding: .utf8)
    let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: folder) }
    let url = folder.appendingPathComponent("tracking.db")
    var connection: OpaquePointer?
    precondition(sqlite3_open(url.path, &connection) == SQLITE_OK)
    let db = connection!
    defer { sqlite3_close(db) }
    func sql(_ value: String) {
      precondition(sqlite3_exec(db, value, nil, nil, nil) == SQLITE_OK)
    }
    func read() -> String {
      var statement: OpaquePointer?
      precondition(sqlite3_prepare_v2(db, "SELECT payload FROM tracking_accounts_v1 WHERE owner='alice'", -1, &statement, nil) == SQLITE_OK)
      defer { sqlite3_finalize(statement) }
      precondition(sqlite3_step(statement) == SQLITE_ROW)
      return String(cString: sqlite3_column_text(statement, 0))
    }
    func encoded(_ value: Any) throws -> String {
      String(data: try JSONSerialization.data(withJSONObject: value), encoding: .utf8)!
    }
    let now = Date(timeIntervalSince1970: 1_800_000_000)
    let snapshot: [String: Any] = ["owner": "alice", "revision": 4,
      "paths": [["id": "guitar", "name": "Guitar", "timeZone": "UTC"]],
      "timers": [], "operations": [], "history": [], "historyRetainedAt": NSNull(),
      "summaries": [:], "unreflected": [], "corrections": [], "notices": []]
    sql("""
      PRAGMA journal_mode=WAL;
      CREATE TABLE tracking_accounts_v1(owner TEXT PRIMARY KEY,revision INTEGER,payload TEXT);
      CREATE TABLE account_deletion_fences_v1(owner TEXT PRIMARY KEY);
      CREATE TABLE native_timer_account_v1(singleton INTEGER PRIMARY KEY,owner TEXT,generation TEXT,expires_at REAL);
      INSERT INTO native_timer_account_v1 VALUES(1,'alice','lease',1800003600000);
      """)
    let original = try encoded(snapshot)
    sql("INSERT INTO tracking_accounts_v1 VALUES('alice',4,'\(original)')")
    let presentation: [String: Any] = ["owner": "alice", "accountLease": "lease", "startLabel": "Start", "stopLabel": "Stop",
      "titles": ["0": "Ready", "1": "Running"], "timers": [["pathId": "guitar", "startedAt": 1_800_000_000_000.0, "stoppedAt": 1_800_000_000_000.0]]]
    try NativeTimerTransaction.register(databaseURL: url, presentation: encoded(presentation))
    var action: [String: Any] = ["owner": "alice", "revision": 4, "pathId": "guitar", "kind": "start", "timerId": "timer", "operationId": "start"]
    func execute(_ value: [String: Any], lease: String = "lease", time: Date? = nil) throws -> String {
      try NativeTimerTransaction.execute(databaseURL: url, lease: lease, actionJSON: encoded(value), domainScript: script, now: time ?? now)
    }
    func rejected(_ operation: () throws -> String) {
      let before = read()
      do { _ = try operation(); preconditionFailure("Native command unexpectedly accepted") }
      catch { precondition(read() == before, "Rejected native command changed tracking") }
    }
    rejected { try execute(action, lease: "previous-account-lease") }
    rejected { try execute(action, time: now.addingTimeInterval(7200)) }
    var other = action; other["owner"] = "bob"
    rejected { try execute(other) }
    sql("INSERT INTO account_deletion_fences_v1 VALUES('alice')")
    rejected { try execute(action) }
    sql("DELETE FROM account_deletion_fences_v1")
    let active = try execute(action)
    let activeObject = try JSONSerialization.jsonObject(with: Data(active.utf8)) as! [String: Any]
    let activeRows = activeObject["timers"] as! [[String: Any]]
    precondition(activeRows[0]["stoppedAt"] == nil, "undefined became null across JavaScriptCore")
    rejected { try execute(action) }
    action["revision"] = 5; action["kind"] = "stop"; action["operationId"] = "stop"
    _ = try execute(action, time: now.addingTimeInterval(60))
    let saved = try JSONSerialization.jsonObject(with: Data(read().utf8)) as! [String: Any]
    precondition((saved["timers"] as! [Any]).isEmpty)
    precondition((saved["operations"] as! [[String: Any]]).count == 2)
    sql("DELETE FROM native_timer_account_v1")
    action["revision"] = 6
    rejected { try execute(action) }
  }
}
