require 'xcodeproj'

# Expo silently excludes a local pod whose minimum OS exceeds the app target.
# Check the generated integration, not merely that its source files exist.
ios = ARGV.fetch(0)
project = Xcodeproj::Project.open(File.join(ios, 'Pods', 'Pods.xcodeproj'))
raise 'HourPathsTimers is missing from the native build' unless project.native_targets.any? { |target| target.name == 'HourPathsTimers' }
providers = Dir.glob(File.join(ios, 'Pods', 'Target Support Files', 'Pods-HourPaths', 'ExpoModulesProvider.swift'))
raise 'HourPathsTimersModule is not registered with Expo' unless providers.any? { |file| File.read(file).include?('HourPathsTimersModule.self') }
delegate = File.read(File.join(ios, 'HourPaths', 'AppDelegate.swift'))
raise 'Application does not include the Live Activity intent package' unless delegate.include?('struct HourPathsAppIntentsPackage: AppIntentsPackage') && delegate.include?('[ExpoWidgetsIntentsPackage.self]')
widget = File.read(File.join(ios, 'ExpoWidgetsTarget', 'index.swift'))
raise 'Widget extension does not include the Live Activity intent package' unless widget.include?('struct ExpoWidgetsExtensionIntentsPackage: AppIntentsPackage') && widget.include?('[ExpoWidgetsIntentsPackage.self]')
puts 'Verified native timer linkage, Expo registration, and App Intents packages'
