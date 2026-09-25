#!/usr/bin/env python3
"""Structural contract for Google Play internal delivery."""

from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
WORKFLOW = (ROOT / ".github/workflows/google-play.yml").read_text(encoding="utf-8")
CONFIG = (ROOT / "apps/mobile/app.config.ts").read_text(encoding="utf-8")
SIGNING = (ROOT / "scripts/configure-android-release-signing.mjs").read_text(encoding="utf-8")
UPLOAD = (ROOT / "scripts/google-play-api.mjs").read_text(encoding="utf-8")


def require(subject: str, fragment: str, description: str) -> None:
    if fragment not in subject:
        raise AssertionError(f"missing {description}: {fragment}")


require(WORKFLOW, "runs-on: ubuntu-24.04", "hosted runner")
require(WORKFLOW, "workflows: [Release]", "automatic tagged-release trigger")
require(WORKFLOW, "environment: google-play", "protected release environment")
require(WORKFLOW, "com.hourpaths.mobile", "immutable package name")
require(WORKFLOW, "ctdb003l6t7f3d5hidfm7", "production Logto client identifier")
require(WORKFLOW, "./gradlew bundleRelease", "Android App Bundle build")
require(WORKFLOW, "GOOGLE_PLAY_SERVICE_ACCOUNT_JSON", "Publisher API credential")
require(WORKFLOW, "ANDROID_UPLOAD_KEYSTORE_BASE64", "upload keystore secret")
require(WORKFLOW, "access_token_scopes: https://www.googleapis.com/auth/androidpublisher", "least-privilege OAuth scope")
require(WORKFLOW, "node scripts/google-play-api.mjs", "internal-track uploader")
require(WORKFLOW, "rm -f", "signing cleanup")
require(WORKFLOW, "for job in changes verify acceptance android-native ios-native", "complete CI gate")
require(WORKFLOW, '"$sdkmanager_path" --sdk_root="$sdk_root"', "direct Android toolchain installation")
if "self-hosted" in WORKFLOW or "track: production" in WORKFLOW or "workflow_call:" in WORKFLOW:
    raise AssertionError("Google Play delivery must remain hosted and internal-only")
if 'yes | "$sdkmanager_path"' in WORKFLOW:
    raise AssertionError("sdkmanager must not inherit a pipefail-sensitive yes pipeline")

require(CONFIG, "HOURPATHS_MOBILE_RELEASE_TAG", "tag-derived version name")
require(CONFIG, "HOURPATHS_MOBILE_BUILD_NUMBER", "numeric platform build version")
require(CONFIG, "versionCode", "Android version code override")
require(SIGNING, "HOURPATHS_ANDROID_UPLOAD_KEYSTORE_PATH", "ephemeral keystore path")
require(SIGNING, "signingConfigs.hourPathsRelease", "release signing selection")
require(UPLOAD, "'internal'", "fixed internal track")
require(UPLOAD, ":commit", "atomic Play edit commit")

print("Google Play delivery contract passed")
