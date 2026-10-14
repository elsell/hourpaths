require 'xcodeproj'

# Expo silently excludes a local pod whose minimum OS exceeds the app target.
# Check the generated integration, not merely that its source files exist.
ios = ARGV.fetch(0)
project = Xcodeproj::Project.open(File.join(ios, 'Pods', 'Pods.xcodeproj'))
raise 'HourPathsTimers is missing from the native build' unless project.native_targets.any? { |target| target.name == 'HourPathsTimers' }
providers = Dir.glob(File.join(ios, 'Pods', 'Target Support Files', 'Pods-HourPaths', 'ExpoModulesProvider.swift'))
raise 'HourPathsTimersModule is not registered with Expo' unless providers.any? { |file| File.read(file).include?('HourPathsTimersModule.self') }
puts 'Verified native timer module linkage and Expo registration'
