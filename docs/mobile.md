# Native mobile development and delivery

The generated Expo application targets iOS and Android. Its checks intentionally
separate JavaScript export, native project generation, native compilation, a
development client, and a signed store build.

## First development build

Run `make dev`, then create and launch an unsigned development client in another
terminal:

```sh
make mobile-dev-ios      # macOS with Xcode
make mobile-dev-android  # macOS or Linux with Android Studio/SDK
```

The app includes `expo-dev-client`. Later JavaScript-only changes can normally use
`make mobile`, which starts Metro for that installed client. Expo Go is not a
supported authentication target because it does not reliably own the generated
custom callback scheme.

`apps/mobile/eas.json` defines an internal `development` profile. EAS is optional;
local `expo run:ios` and `expo run:android` remain first-class paths. If you use
EAS, use the locked `eas-cli 21.0.2` tooling workspace through the checked package
scripts rather than a floating global CLI or `pnpm dlx`.
The wrapper verifies that EAS runs from `apps/mobile`, so it discovers this
application's `app.json` and `eas.json`.

## Device-reachable development URLs

The root `.env` is loaded by the Make targets. Set all four values together:

```sh
HOURPATHS_API_URL=http://localhost:8080
HOURPATHS_OIDC_ISSUER=http://localhost:5556/dex
HOURPATHS_MOBILE_OIDC_CLIENT_ID=hourpaths-mobile
HOURPATHS_EXPO_PROJECT_ID=<your-expo-project-uuid>
```

- An iOS simulator normally reaches host services through `localhost`.
- The Android emulator normally uses `10.0.2.2` instead of `localhost`.
- A physical device needs the development computer's LAN address and services
  intentionally bound to that interface. Never expose bundled development
  credentials on an untrusted network.

Register `hourpaths://callback` on the provider's public native client.
The checked native identifiers are explicit and must match the Apple/Android
store records; the current application identifier is `com.hourpaths.mobile`.
When changing a LAN issuer, change the provider issuer and API issuer together.
Plain HTTP is development-only.

## What each check proves

```sh
make mobile-validate       # Expo Doctor, dependency compatibility, config identity
make mobile-export         # iOS and Android Metro/Expo bundles only
make mobile-prebuild       # clean generated iOS and Android native projects
make mobile-build-android  # unsigned Android debug APK through Gradle
make mobile-build-ios      # unsigned iOS simulator app through Xcode
```

Bundle export is useful but is not a native build. Generated CI compiles Android
on Linux and compiles iOS on the hosted `macos-26` runner. No iOS prebuild,
dependency-resolution, simulator validation, or iOS-specific check runs on a
Linux runner. Pull requests use the same unsigned simulator build path for
native-affecting changes; no Apple signing access is required.

## Release configuration

Before the first store build:

1. Replace the neutral files under `apps/mobile/assets/` with product artwork.
2. Set `expo.version`, `ios.buildNumber`, and `android.versionCode` deliberately.
3. Review every native permission emitted by clean prebuild. Add only permissions
   justified by a product spec and store disclosure.
4. Decide whether to enable Expo Updates. It is disabled by default; runtime
   version follows application version for an explicit compatibility boundary.
5. Configure production HTTPS endpoints, callback URIs, Expo ownership, store
   metadata, signing credentials, and access controls outside Git. Never commit
   signing keys or provider client secrets.

The guarded commands refuse missing, local, or non-HTTPS production endpoints:

```sh
make mobile-release-ios
make mobile-release-android
```

These invoke the checked production EAS profile. Store publication is not
automatic; review and submit signed artifacts deliberately.
The Expo SDK 55 profile selects the reviewed
`ubuntu-24.04-jdk-17-ndk-r27b-sdk-55` Android image and
`macos-sequoia-15.6-xcode-26.2` iOS image for EAS builds. Local iOS builds
install the exact CocoaPods 1.16.2 graph from `Gemfile.lock`; hosted CI pins
the macos-26 image's Homebrew Ruby 3.4, Bundler 2.6.9 resolves that same lockfile, and the Android job installs
the exact CMake 3.22.1 SDK package when needed, then verifies its Java 17, SDK,
NDK, CMake, and Ninja inputs before Gradle runs. The
dependency-age and OSV gates inspect every locked Ruby gem, and `make-app doctor`
checks the Ruby/Bundler versions used by local builds. Hosted iOS compilation
uses GitHub's `macos-26` runner to satisfy Expo SDK 55's Xcode 26 requirement.

The production profile selects the EAS environment named `production`. Configure
`HOURPATHS_API_URL`, `HOURPATHS_OIDC_ISSUER`, and
`HOURPATHS_MOBILE_OIDC_CLIENT_ID` in that EAS environment before using the EAS
path. `HOURPATHS_EXPO_PROJECT_ID` is only needed when Expo push notifications or
EAS project linking is enabled; the native TestFlight workflow may omit it and
will build an app with push registration disabled. These are public application
configuration values, not secrets, but the API and issuer must use HTTPS. The
generated application sets `HOURPATHS_APP_ENV=production` and deliberately
fails bundling if either production endpoint is absent; it cannot silently fall
back to localhost.

The server release workflow publishes only server artifacts. The protected
`TestFlight` workflow below performs the native archive and upload on a GitHub
hosted macOS runner; signing credentials remain outside the repository.

### TestFlight workflow inputs

The workflow is manual and uses the GitHub `testflight` environment. Add these
environment secrets/variables before running it:

- secrets: `APPLE_DISTRIBUTION_P12_BASE64`, `APPLE_DISTRIBUTION_P12_PASSWORD`,
  `APPLE_PROVISIONING_PROFILE_BASE64`, `ASC_API_KEY_P8_BASE64`
- variables: `APPLE_TEAM_ID`, `APPLE_PROVISIONING_PROFILE_NAME`,
  `ASC_API_KEY_ID`, and `ASC_API_ISSUER_ID`

The App Store Connect key must be an active App Store Connect API key, not a
Sign in with Apple key. The provisioning profile must target
`com.hourpaths.mobile` and the distribution certificate's private key must be
included in the `.p12`. The workflow uses the production endpoints
`https://api.hourpaths.com` and `https://login.hourpaths.com/oidc` and the
broker-assigned public Logto App ID `ctdb003l6t7f3d5hidfm7`. The Logto
application name `hourpaths-mobile` is only a display name and must not be sent
as the OAuth `client_id`. The native application uses authorization code with
PKCE and therefore has no client secret.

## Session behavior

Native sessions live in platform secure storage. The shared state machine
distinguishes authenticated online, authenticated temporarily offline,
authentication required, and unreadable local storage. Network loss, rate limits,
and server outages retain an otherwise valid credential. Expiry, explicit 401
rejection, revocation, or malformed secure storage removes it. Product-specific
offline data and mutation synchronization remain application concerns. Cold
launch reads secure storage before OIDC discovery succeeds, so an existing valid
application session can enter offline mode without provider connectivity.
