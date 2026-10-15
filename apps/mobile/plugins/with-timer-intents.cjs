const { withAppDelegate } = require('expo/config-plugins');

// ExpoWidgets owns the LiveActivityIntent. Its metadata must be imported by
// the application target so iOS can launch the app's native handler directly.
module.exports = config => withAppDelegate(config, mod => {
  if (mod.modResults.language !== 'swift') throw new Error('timer_intents_require_swift_app_delegate');
  if (!mod.modResults.contents.includes('struct HourPathsAppIntentsPackage:')) {
    mod.modResults.contents += `
import AppIntents
import ExpoWidgets

@available(iOS 17.0, *)
struct HourPathsAppIntentsPackage: AppIntentsPackage {
  static var includedPackages: [any AppIntentsPackage.Type] {
    [ExpoWidgetsIntentsPackage.self]
  }
}
`;
  }
  return mod;
});
