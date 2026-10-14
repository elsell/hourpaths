import Foundation
import Darwin

/// Only presentation data is shared with the extension. Remote refreshes carry
/// the lease identity, never timer state or executable action targets.
final class NativeTimerPresentationCache {
  enum Failure: Error { case unavailable, invalid }
  private let directory: URL
  private let name: String

  init(directory: URL, name: String = "HourPathsTimers") throws {
    guard !name.isEmpty, name.utf8.allSatisfy({ (48...57).contains($0) || (65...90).contains($0) || (97...122).contains($0) || $0 == 95 }) else { throw Failure.invalid }
    self.directory = directory
    self.name = name
  }

  static func configured() throws -> NativeTimerPresentationCache {
    guard let group = Bundle.main.object(forInfoDictionaryKey: "ExpoWidgetsAppGroupIdentifier") as? String,
          let directory = FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: group) else { throw Failure.unavailable }
    return try NativeTimerPresentationCache(directory: directory)
  }

  private var file: URL { directory.appendingPathComponent("__expo_widgets_live_activity_\(name)_presentation.json") }

  /// Held across the durable transaction and publication, including revocation.
  /// A delayed native action therefore cannot republish after sign-out clears it.
  func withExclusiveAccess<T>(_ operation: () throws -> T) throws -> T {
    let path = directory.appendingPathComponent("__expo_widgets_live_activity_\(name)_presentation.lock").path
    let descriptor = open(path, O_CREAT | O_RDWR, S_IRUSR | S_IWUSR)
    guard descriptor >= 0 else { throw Failure.unavailable }
    defer { close(descriptor) }
    guard flock(descriptor, LOCK_EX) == 0 else { throw Failure.unavailable }
    defer { flock(descriptor, LOCK_UN) }
    return try operation()
  }

  func invalidate() throws {
    if FileManager.default.fileExists(atPath: file.path) { try FileManager.default.removeItem(at: file) }
  }

  func publish(_ presentation: String, now: Date) throws -> String {
    let data = Data(presentation.utf8)
    guard data.count <= 131072, let props = try JSONSerialization.jsonObject(with: data) as? [String: Any],
          let owner = props["owner"] as? String, !owner.isEmpty,
          let lease = props["accountLease"] as? String, !lease.isEmpty,
          let expiry = props["activityExpiresAt"] as? Double, expiry.isFinite,
          expiry > now.timeIntervalSince1970 * 1000 else { throw Failure.invalid }
    // This opt-in survives revocation, so remote content can never become a fallback.
    try Data().write(to: directory.appendingPathComponent("__expo_widgets_live_activity_\(name)_local_presentation"), options: .atomic)
    try data.write(to: file, options: .atomic)
    // iOS requires readable data after the first unlock for Lock Screen actions.
    #if os(iOS)
    try FileManager.default.setAttributes([.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication], ofItemAtPath: file.path)
    #endif
    let boundary = ((props["timers"] as? [[String: Any]]) ?? []).flatMap { row in
      (row["goalPeriods"] as? [[String: Any]]) ?? []
    }.compactMap { $0["endsAt"] as? Double }.filter { $0 > now.timeIntervalSince1970 * 1000 }.min()
    let envelope: [String: Any] = ["owner": owner, "accountLease": lease,
      "localPresentation": true, "staleAt": min(boundary ?? expiry, expiry)]
    return String(data: try JSONSerialization.data(withJSONObject: envelope), encoding: .utf8)!
  }
}
