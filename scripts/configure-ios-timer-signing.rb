require 'xcodeproj'

# Configure individual targets. Global bundle/profile overrides incorrectly sign
# the Live Activity extension as the containing app.
project = Xcodeproj::Project.open(ARGV.fetch(0))
bundle = ENV.fetch('APPLE_BUNDLE_ID')
profiles = {
  bundle => ENV.fetch('APPLE_PROVISIONING_PROFILE_NAME'),
  "#{bundle}.ExpoWidgetsTarget" => ENV.fetch('APPLE_WIDGET_PROVISIONING_PROFILE_NAME')
}
configured = []
project.native_targets.each do |target|
  next unless ['com.apple.product-type.application', 'com.apple.product-type.app-extension'].include?(target.product_type)
  target.build_configurations.each do |configuration|
    id = configuration.build_settings.fetch('PRODUCT_BUNDLE_IDENTIFIER')
    raise "Unexpected signing target #{id}" unless profiles.key?(id)
    configuration.build_settings['CODE_SIGN_STYLE'] = 'Manual'
    configuration.build_settings['DEVELOPMENT_TEAM'] = ENV.fetch('APPLE_TEAM_ID')
    configuration.build_settings['CODE_SIGN_IDENTITY'] = 'Apple Distribution'
    configuration.build_settings['PROVISIONING_PROFILE_SPECIFIER'] = profiles.fetch(id)
    configured << id
  end
end
raise 'Missing app or Live Activity extension signing target' unless configured.uniq.sort == profiles.keys.sort
project.save
