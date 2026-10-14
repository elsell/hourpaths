import ExpoModulesCore
import ExpoWidgets
import Foundation

public final class HourPathsTimersModule: Module {
  public func definition() -> ModuleDefinition {
    Name("HourPathsTimers")
    AsyncFunction("configureActions") { (databasePath: String, presentation: String) in
      let url = URL(fileURLWithPath: databasePath)
      let cache = try NativeTimerPresentationCache.configured()
      return try cache.withExclusiveAccess {
        try NativeTimerTransaction.register(databaseURL: url, presentation: presentation)
        UserDefaults.standard.set(databasePath, forKey: "HourPathsNativeTimerDatabase")
        let envelope = try cache.publish(presentation, now: Date())
        return envelope
      }
    }
    AsyncFunction("clearPresentation") {
      let cache = try NativeTimerPresentationCache.configured()
      try cache.withExclusiveAccess { try cache.invalidate() }
    }
  }
}

@objc(HourPathsNativeTimerIntent)
public final class HourPathsNativeTimerIntent: NSObject, ExpoLiveActivityActionHandler {
  public static func performAction(_ target: String) throws -> String {
    guard let data = target.data(using: .utf8),
          let value = try JSONSerialization.jsonObject(with: data) as? [String: Any],
          let lease = value["lease"] as? String, let action = value["action"] as? [String: Any],
          let databasePath = UserDefaults.standard.string(forKey: "HourPathsNativeTimerDatabase"),
          let bundleURL = Bundle(for: HourPathsNativeTimerIntent.self).url(forResource: "HourPathsTimerDomain", withExtension: "bundle"),
          let bundle = Bundle(url: bundleURL), let scriptURL = bundle.url(forResource: "TimerDomain", withExtension: "js") else {
      throw NativeTimerTransaction.Failure.unavailable
    }
    let actionData = try JSONSerialization.data(withJSONObject: action)
    guard let actionJSON = String(data: actionData, encoding: .utf8) else { throw NativeTimerTransaction.Failure.invalid }
    let cache = try NativeTimerPresentationCache.configured()
    return try cache.withExclusiveAccess {
      let now = Date()
      let props = try NativeTimerTransaction.execute(databaseURL: URL(fileURLWithPath: databasePath), lease: lease,
        actionJSON: actionJSON, domainScript: String(contentsOf: scriptURL, encoding: .utf8), now: now,
        beforeCommit: { try cache.invalidate() })
      return try cache.publish(props, now: now)
    }
  }
}
