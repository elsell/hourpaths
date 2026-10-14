import ExpoModulesCore
import ExpoWidgets
import Foundation

public final class HourPathsTimersModule: Module {
  public func definition() -> ModuleDefinition {
    Name("HourPathsTimers")
    AsyncFunction("configureActions") { (databasePath: String, presentation: String) in
      let url = URL(fileURLWithPath: databasePath)
      try NativeTimerTransaction.register(databaseURL: url, presentation: presentation)
      UserDefaults.standard.set(databasePath, forKey: "HourPathsNativeTimerDatabase")
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
    return try NativeTimerTransaction.execute(databaseURL: URL(fileURLWithPath: databasePath), lease: lease,
      actionJSON: actionJSON, domainScript: String(contentsOf: scriptURL, encoding: .utf8), now: Date())
  }
}
