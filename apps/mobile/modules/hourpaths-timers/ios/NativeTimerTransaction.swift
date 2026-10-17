import Foundation
import JavaScriptCore
// Share Expo's engine and its process-local WAL lock registry with the foreground.
import ExpoSQLite

/// Runs the shared domain mutation while holding the same SQLite writer lock as
/// the foreground store. The native intent never owns a second timer ledger.
final class NativeTimerTransaction {
  enum Failure: Error { case unavailable, stale, invalid, storage, domain }
  private static let transient = unsafeBitCast(-1, to: sqlite3_destructor_type.self)

  static func execute(databaseURL: URL, lease: String, actionJSON: String, domainScript: String, now: Date, beforeCommit: () throws -> Void = {}) throws -> String {
    guard let data = actionJSON.data(using: .utf8),
          let action = try JSONSerialization.jsonObject(with: data) as? [String: Any],
          let owner = action["owner"] as? String, !owner.isEmpty,
          action["revision"] is NSNumber else { throw Failure.invalid }
    var database: OpaquePointer?
    guard exsqlite3_open_v2(databaseURL.path, &database, SQLITE_OPEN_READWRITE | SQLITE_OPEN_FULLMUTEX, nil) == SQLITE_OK,
          let db = database else {
      if let database { exsqlite3_close(database) }
      throw Failure.unavailable
    }
    defer { exsqlite3_close(db) }
    exsqlite3_busy_timeout(db, 3000)
    try run(db, "PRAGMA synchronous = FULL")
    try run(db, "BEGIN IMMEDIATE")
    var committed = false
    defer { if !committed { try? run(db, "ROLLBACK") } }

    // A sign-out/replacement/deletion must revoke this lease under the same
    // writer lock. A stale notification cannot recreate a lease or database.
    let leaseQuery = try prepare(db, """
      SELECT owner, generation, expires_at FROM native_timer_account_v1 WHERE singleton = 1
      AND owner NOT IN (SELECT owner FROM account_deletion_fences_v1)
      """)
    defer { exsqlite3_finalize(leaseQuery) }
    guard exsqlite3_step(leaseQuery) == SQLITE_ROW,
          string(leaseQuery, 0) == owner, string(leaseQuery, 1) == lease,
          exsqlite3_column_double(leaseQuery, 2) > now.timeIntervalSince1970 * 1000 else { throw Failure.stale }

    let read = try prepare(db, "SELECT payload, revision FROM tracking_accounts_v1 WHERE owner = ?")
    defer { exsqlite3_finalize(read) }
    bind(read, 1, owner)
    guard exsqlite3_step(read) == SQLITE_ROW,
          let payload = string(read, 0), let context = JSContext() else { throw Failure.stale }
    // The domain validates the displayed action against current timer identity.
    // Commit against the revision read under this writer lock, not the old UI.
    let revision = exsqlite3_column_int64(read, 1)
    // No network, tokens, file APIs or host callbacks are exposed to this VM.
    context.evaluateScript(domainScript)
    guard context.exception == nil,
          let apply = context.objectForKeyedSubscript("HourPathsTimerDomain")?.objectForKeyedSubscript("applyNativeTimerAction") else { throw Failure.domain }
    guard let snapshotData = payload.data(using: .utf8),
          let snapshot = try JSONSerialization.jsonObject(with: snapshotData) as? [String: Any],
          snapshot["revision"] as? Int64 == revision else { throw Failure.invalid }
    let result = apply.call(withArguments: [snapshot, action, now.timeIntervalSince1970 * 1000])
    guard context.exception == nil, let next = result?.toDictionary(),
          let nextOwner = next["owner"] as? String, nextOwner == owner,
          let nextRevision = next["revision"] as? NSNumber, nextRevision.int64Value == revision + 1 else { throw Failure.domain }
    let presentationRead = try prepare(db, "SELECT payload FROM native_timer_presentation_v1 WHERE singleton = 1")
    defer { exsqlite3_finalize(presentationRead) }
    guard exsqlite3_step(presentationRead) == SQLITE_ROW, let presentationJSON = string(presentationRead, 0),
          let presentationData = presentationJSON.data(using: .utf8),
          let presentation = try JSONSerialization.jsonObject(with: presentationData) as? [String: Any],
          presentation["accountLease"] as? String == lease,
          let rows = presentation["timers"] as? [[String: Any]],
          let refresh = context.objectForKeyedSubscript("HourPathsTimerDomain")?.objectForKeyedSubscript("refreshNativeTimerPresentation") else { throw Failure.stale }
    let identifiers = (0..<(rows.count * 2)).map { _ in UUID().uuidString }
    guard let refreshed = refresh.call(withArguments: [presentation, next, now.timeIntervalSince1970 * 1000, identifiers]),
          context.exception == nil,
          let propsJSON = context.objectForKeyedSubscript("JSON")?.invokeMethod("stringify", withArguments: [refreshed])?.toString(),
          context.exception == nil else { throw Failure.domain }
    let presentationWrite = try prepare(db, "UPDATE native_timer_presentation_v1 SET payload = ? WHERE singleton = 1")
    defer { exsqlite3_finalize(presentationWrite) }
    bind(presentationWrite, 1, propsJSON)
    guard exsqlite3_step(presentationWrite) == SQLITE_DONE else { throw Failure.storage }
    let encoded = try JSONSerialization.data(withJSONObject: next)
    guard let nextPayload = String(data: encoded, encoding: .utf8) else { throw Failure.invalid }
    let update = try prepare(db, "UPDATE tracking_accounts_v1 SET payload = ?, revision = ? WHERE owner = ? AND revision = ?")
    defer { exsqlite3_finalize(update) }
    bind(update, 1, nextPayload)
    exsqlite3_bind_int64(update, 2, revision + 1)
    bind(update, 3, owner)
    exsqlite3_bind_int64(update, 4, revision)
    guard exsqlite3_step(update) == SQLITE_DONE, exsqlite3_changes(db) == 1 else { throw Failure.storage }
    try beforeCommit()
    try run(db, "COMMIT")
    committed = true
    return propsJSON
  }

  static func register(databaseURL: URL, presentation: String, now: Date = Date()) throws {
    guard let props = try JSONSerialization.jsonObject(with: Data(presentation.utf8)) as? [String: Any],
          let owner = props["owner"] as? String, let lease = props["accountLease"] as? String else { throw Failure.invalid }
    var database: OpaquePointer?
    guard exsqlite3_open_v2(databaseURL.path, &database, SQLITE_OPEN_READWRITE | SQLITE_OPEN_FULLMUTEX, nil) == SQLITE_OK,
          let db = database else {
      if let database { exsqlite3_close(database) }
      throw Failure.unavailable
    }
    defer { exsqlite3_close(db) }
    exsqlite3_busy_timeout(db, 3000)
    try run(db, "BEGIN IMMEDIATE")
    var committed = false
    defer { if !committed { try? run(db, "ROLLBACK") } }
    let account = try prepare(db, """
      SELECT a.owner, a.generation, a.expires_at, t.revision FROM native_timer_account_v1 a
      JOIN tracking_accounts_v1 t ON t.owner = a.owner WHERE a.singleton = 1
      AND NOT EXISTS (SELECT 1 FROM account_deletion_fences_v1 d WHERE d.owner = a.owner)
      """)
    defer { exsqlite3_finalize(account) }
    guard exsqlite3_step(account) == SQLITE_ROW, string(account, 0) == owner, string(account, 1) == lease,
          exsqlite3_column_double(account, 2) > now.timeIntervalSince1970 * 1000 else { throw Failure.stale }
    if let revision = props["trackingRevision"] as? Int64,
       revision != exsqlite3_column_int64(account, 3) { throw Failure.stale }
    try run(db, "CREATE TABLE IF NOT EXISTS native_timer_presentation_v1(singleton INTEGER PRIMARY KEY CHECK(singleton = 1), payload TEXT NOT NULL)")
    let statement = try prepare(db, "INSERT INTO native_timer_presentation_v1(singleton,payload) VALUES(1,?) ON CONFLICT(singleton) DO UPDATE SET payload=excluded.payload")
    defer { exsqlite3_finalize(statement) }
    bind(statement, 1, presentation)
    guard exsqlite3_step(statement) == SQLITE_DONE else { throw Failure.storage }
    try run(db, "COMMIT")
    committed = true
  }

  private static func prepare(_ db: OpaquePointer, _ sql: String) throws -> OpaquePointer {
    var statement: OpaquePointer?
    guard exsqlite3_prepare_v2(db, sql, -1, &statement, nil) == SQLITE_OK, let statement else { throw Failure.storage }
    return statement
  }
  private static func run(_ db: OpaquePointer, _ sql: String) throws {
    guard exsqlite3_exec(db, sql, nil, nil, nil) == SQLITE_OK else { throw Failure.storage }
  }
  private static func bind(_ statement: OpaquePointer, _ index: Int32, _ value: String) {
    _ = value.withCString { exsqlite3_bind_text(statement, index, $0, -1, transient) }
  }
  private static func string(_ statement: OpaquePointer, _ index: Int32) -> String? {
    guard let value = exsqlite3_column_text(statement, index) else { return nil }
    return String(cString: value)
  }
}
