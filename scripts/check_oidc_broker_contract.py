#!/usr/bin/env python3
import re
import sys
from pathlib import Path


REQUIRED_FRAGMENTS = {
    "docs/oidc.md": (
        "one fixed HourPaths-owned production broker issuer",
        "Google and Apple are upstream identity providers",
        "three public authorization-code clients",
        "broker-hosted provider chooser",
        "must not automatically link identities by email",
        "same upstream identity must receive the same broker subject",
        "stable and must never be reassigned",
        "Apple may omit `name`",
        "Apple private-relay",
        "external secret manager",
        "key rotation",
        "Dex is for local development and CI only",
        "Changing the production issuer is an identity-data migration",
    ),
    "specs/platform/architecture.spec.md": (
        "one fixed HourPaths-owned OIDC broker issuer",
        "Google and Apple are upstream identity providers",
        "must not automatically link upstream identities by email",
        "provider-distinct",
        "Dex remains a local-development and CI adapter only",
    ),
    "specs/platform/testing.spec.md": (
        "production-broker conformance",
        "Google and Apple claim fixtures",
        "missing Apple `name`",
        "Apple private-relay email",
        "cross-client subject stability",
        "broker key rotation and unavailability",
    ),
}

LOCAL_DEX_FRAGMENTS = {
    "deploy/dex/config.yaml": (
        "issuer: http://localhost:5556/dex",
        "enablePasswordDB: true",
    ),
    "compose.yaml": (
        "image: dexidp/dex:",
        'ports: ["127.0.0.1:${HOURPATHS_DEX_HOST_PORT:-5556}:5556"]',
        'ports: ["127.0.0.1:${HOURPATHS_API_HOST_PORT:-8080}:8080"]',
        'ports: ["127.0.0.1:${HOURPATHS_SPICEDB_HOST_PORT:-50051}:50051"]',
        "${HOURPATHS_DEX_CONFIG_PATH:-./deploy/dex/config.yaml}:/etc/dex/config.yaml:ro",
        "HOURPATHS_OIDC_ISSUER: ${HOURPATHS_DEX_PUBLIC_ISSUER:-http://localhost:5556/dex}",
        "HOURPATHS_PUBLIC_BASE_URL: ${HOURPATHS_API_PUBLIC_BASE_URL:-http://localhost:8080}",
        'HOURPATHS_OIDC_INSECURE: "true"',
    ),
}

PRODUCTION_SURFACES = (
    ".github/workflows",
    "deploy",
    "apps/mobile/eas.json",
    "apps/api/Dockerfile",
    "apps/web/Dockerfile",
)
DEX_PRODUCTION_PATTERN = re.compile(r"(?:\bdexidp\b|\bdex\b|localhost:5556)", re.IGNORECASE)


def production_files(root: Path):
    for relative in PRODUCTION_SURFACES:
        target = root / relative
        if target.is_file():
            yield target
        elif target.is_dir():
            for candidate in target.rglob("*"):
                if candidate.is_file() and not candidate.is_relative_to(root / "deploy/dex"):
                    yield candidate


def validate(root: Path) -> list[str]:
    errors: list[str] = []
    for relative, fragments in REQUIRED_FRAGMENTS.items():
        target = root / relative
        if not target.is_file():
            errors.append(f"OIDC broker contract file is missing: {relative}")
            continue
        contents = " ".join(target.read_text(encoding="utf-8").split()).casefold()
        for fragment in fragments:
            if " ".join(fragment.split()).casefold() not in contents:
                errors.append(f"{relative} must preserve OIDC broker contract: {fragment}")
    for relative, fragments in LOCAL_DEX_FRAGMENTS.items():
        target = root / relative
        if not target.is_file():
            errors.append(f"local Dex configuration is missing: {relative}")
            continue
        contents = " ".join(target.read_text(encoding="utf-8").split()).casefold()
        for fragment in fragments:
            if " ".join(fragment.split()).casefold() not in contents:
                errors.append(f"{relative} must preserve local Dex boundary: {fragment}")
    for target in production_files(root):
        contents = target.read_text(encoding="utf-8")
        if DEX_PRODUCTION_PATTERN.search(contents):
            relative = target.relative_to(root)
            errors.append(f"production surface must not reference local Dex: {relative}")
    return errors


def main() -> int:
    root = Path(__file__).resolve().parent.parent
    errors = validate(root)
    for error in errors:
        print(f"OIDC broker contract: {error}", file=sys.stderr)
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
