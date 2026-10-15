import Foundation
import ExpoSQLite

@main
struct NativeTimerTransactionAcceptance {
  static func main() throws {
    let root = URL(fileURLWithPath: CommandLine.arguments[1])
    let script = try String(contentsOf: root.appendingPathComponent("apps/mobile/modules/hourpaths-timers/ios/TimerDomain.js"), encoding: .utf8)
    let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: folder) }
    let cache = try NativeTimerPresentationCache(directory: folder)
    let cacheNow = Date(timeIntervalSince1970: 1_800_000_000)
    try cache.withExclusiveAccess {
      let local: [String: Any] = ["owner": "alice", "accountLease": "lease", "activityExpiresAt": 1_800_007_200_000.0,
        "timers": [["action": ["target": "local-only"], "goalPeriods": [["endsAt": 1_800_003_600_000.0]]]]]
      let json = String(data: try JSONSerialization.data(withJSONObject: local), encoding: .utf8)!
      let envelope = try cache.publish(json, now: cacheNow)
      precondition(!envelope.contains("local-only"), "Native action leaked into refresh envelope")
      let value = try JSONSerialization.jsonObject(with: Data(envelope.utf8)) as! [String: Any]
      precondition(value["staleAt"] as? Double == 1_800_003_600_000.0)
      let file = folder.appendingPathComponent("__expo_widgets_live_activity_HourPathsTimers_presentation.json")
      let cached = try String(contentsOf: file, encoding: .utf8)
      precondition(cached == json)
      try cache.invalidate()
      precondition(!FileManager.default.fileExists(atPath: file.path))
      precondition(FileManager.default.fileExists(atPath: folder.appendingPathComponent("__expo_widgets_live_activity_HourPathsTimers_local_presentation").path), "Revocation removed the no-fallback fence")
    }
    let url = folder.appendingPathComponent("tracking.db")
    var connection: OpaquePointer?
    precondition(exsqlite3_open(url.path, &connection) == SQLITE_OK)
    let db = connection!
    defer { exsqlite3_close(db) }
    func sql(_ value: String) {
      precondition(exsqlite3_exec(db, value, nil, nil, nil) == SQLITE_OK)
    }
    func read() -> String {
      var statement: OpaquePointer?
      precondition(exsqlite3_prepare_v2(db, "SELECT payload FROM tracking_accounts_v1 WHERE owner='alice'", -1, &statement, nil) == SQLITE_OK)
      defer { exsqlite3_finalize(statement) }
      precondition(exsqlite3_step(statement) == SQLITE_ROW)
      return String(cString: exsqlite3_column_text(statement, 0))
    }
    func encoded(_ value: Any) throws -> String {
      String(data: try JSONSerialization.data(withJSONObject: value), encoding: .utf8)!
    }
    let now = Date(timeIntervalSince1970: 1_800_000_000)
    let snapshot: [String: Any] = ["owner": "alice", "revision": 4,
      "paths": [["id": "guitar", "name": "Guitar", "timeZone": "UTC",
        "goal": ["recurrence": "hourly", "alignment": ["minute": 0], "targetSeconds": 600]]],
      "timers": [], "operations": [], "history": [], "historyRetainedAt": NSNull(),
      "summaries": ["guitar": ["savedTotalSeconds": 0, "period": ["savedSeconds": 0, "targetSeconds": 600,
        "startsAt": now.timeIntervalSince1970 * 1000, "endsAt": now.addingTimeInterval(3600).timeIntervalSince1970 * 1000]]], "unreflected": [], "corrections": [], "notices": []]
    sql("""
      PRAGMA journal_mode=WAL;
      CREATE TABLE tracking_accounts_v1(owner TEXT PRIMARY KEY,revision INTEGER,payload TEXT);
      CREATE TABLE account_deletion_fences_v1(owner TEXT PRIMARY KEY);
      CREATE TABLE native_timer_account_v1(singleton INTEGER PRIMARY KEY,owner TEXT,generation TEXT,expires_at REAL);
      INSERT INTO native_timer_account_v1 VALUES(1,'alice','lease',1800003600000);
      """)
    let original = try encoded(snapshot)
    sql("INSERT INTO tracking_accounts_v1 VALUES('alice',4,'\(original)')")
    let presentation: [String: Any] = ["activityExpiresAt": now.addingTimeInterval(7200).timeIntervalSince1970 * 1000, "owner": "alice", "accountLease": "lease", "startLabel": "Start", "stopLabel": "Stop",
      "titles": ["0": "Ready", "1": "Running"], "timers": [["pathId": "guitar", "startedAt": 1_800_000_000_000.0, "stoppedAt": 1_800_000_000_000.0]]]
    // Native registration must respect the foreground connection's WAL writer lock.
    // Separate SQLite library instances in one process bypass this lock and can
    // truncate each other's mapped WAL index (the build 525 launch crash).
    sql("BEGIN IMMEDIATE")
    do {
      try NativeTimerTransaction.register(databaseURL: url, presentation: encoded(presentation), now: now)
      preconditionFailure("Native writer bypassed the foreground transaction")
    } catch NativeTimerTransaction.Failure.storage {}
    sql("ROLLBACK")
    try NativeTimerTransaction.register(databaseURL: url, presentation: encoded(presentation), now: now)
    var stalePresentation = presentation
    stalePresentation["trackingRevision"] = 3
    do {
      try NativeTimerTransaction.register(databaseURL: url, presentation: encoded(stalePresentation), now: now)
      preconditionFailure("Old foreground presentation replaced native state")
    } catch {}
    stalePresentation["trackingRevision"] = 4
    stalePresentation["accountLease"] = "revoked"
    do {
      try NativeTimerTransaction.register(databaseURL: url, presentation: encoded(stalePresentation), now: now)
      preconditionFailure("Revoked presentation was registered")
    } catch {}
    var action: [String: Any] = ["owner": "alice", "revision": 4, "pathId": "guitar", "kind": "start", "timerId": "timer", "operationId": "start"]
    func execute(_ value: [String: Any], lease: String = "lease", time: Date? = nil) throws -> String {
      try NativeTimerTransaction.execute(databaseURL: url, lease: lease, actionJSON: encoded(value), domainScript: script, now: time ?? now)
    }
    func rejected(_ operation: () throws -> String) {
      let before = read()
      do { _ = try operation(); preconditionFailure("Native command unexpectedly accepted") }
      catch { precondition(read() == before, "Rejected native command changed tracking") }
    }
    rejected {
      try NativeTimerTransaction.execute(databaseURL: url, lease: "lease", actionJSON: encoded(action), domainScript: script,
        now: now, beforeCommit: { throw NativeTimerPresentationCache.Failure.unavailable })
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
    let stoppedPresentation = try execute(action, time: now.addingTimeInterval(60))
    let stoppedObject = try JSONSerialization.jsonObject(with: Data(stoppedPresentation.utf8)) as! [String: Any]
    let stoppedRows = stoppedObject["timers"] as! [[String: Any]]
    let periods = stoppedRows[0]["goalPeriods"] as! [[String: Any]]
    precondition(periods.count == 2)
    precondition((periods[0]["savedSeconds"] as! NSNumber).intValue == 60)
    precondition((periods[1]["savedSeconds"] as! NSNumber).intValue == 0)
    let saved = try JSONSerialization.jsonObject(with: Data(read().utf8)) as! [String: Any]
    precondition((saved["timers"] as! [Any]).isEmpty)
    precondition((saved["operations"] as! [[String: Any]]).count == 2)
    sql("DELETE FROM native_timer_account_v1")
    action["revision"] = 6
    rejected { try execute(action) }
  }
}
