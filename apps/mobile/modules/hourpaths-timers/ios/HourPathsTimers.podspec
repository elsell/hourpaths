Pod::Spec.new do |s|
  s.name = 'HourPathsTimers'
  s.version = '1.0.0'
  s.summary = 'HourPaths durable native timer controls'
  s.description = s.summary
  s.license = 'Apache-2.0'
  s.author = 'HourPaths'
  s.homepage = 'https://github.com/elsell/hourpaths'
  s.source = { git: 'https://github.com/elsell/hourpaths.git' }
  s.platforms = { :ios => '16.2' }
  s.swift_version = '5.9'
  s.static_framework = true
  s.dependency 'ExpoModulesCore'
  s.dependency 'ExpoWidgets'
  s.frameworks = 'JavaScriptCore'
  s.libraries = 'sqlite3'
  s.source_files = '*.swift'
  s.resource_bundles = { 'HourPathsTimerDomain' => ['TimerDomain.js'] }
end
