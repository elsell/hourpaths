import tempfile
import unittest
from pathlib import Path

import check_oidc_broker_contract


class OIDCBrokerContractTest(unittest.TestCase):
    def write_contract(self, root: Path) -> None:
        for relative, fragments in check_oidc_broker_contract.REQUIRED_FRAGMENTS.items():
            target = root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text("\n".join(fragments), encoding="utf-8")
        for relative, fragments in check_oidc_broker_contract.LOCAL_DEX_FRAGMENTS.items():
            target = root / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            existing = target.read_text(encoding="utf-8") if target.exists() else ""
            target.write_text(existing + "\n" + "\n".join(fragments), encoding="utf-8")

    def test_complete_provider_neutral_contract_passes(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.write_contract(root)
            self.assertEqual(check_oidc_broker_contract.validate(root), [])

    def test_missing_google_or_apple_contract_fails_closed(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.write_contract(root)
            guide = root / "docs/oidc.md"
            guide.write_text(
                guide.read_text(encoding="utf-8").replace("Google and Apple are upstream identity providers", "approved upstreams are identity providers"),
                encoding="utf-8",
            )
            self.assertTrue(any("Google and Apple" in error for error in check_oidc_broker_contract.validate(root)))

    def test_every_required_contract_clause_is_enforced(self) -> None:
        for relative, fragments in check_oidc_broker_contract.REQUIRED_FRAGMENTS.items():
            for fragment in fragments:
                with self.subTest(file=relative, fragment=fragment), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    self.write_contract(root)
                    target = root / relative
                    target.write_text(
                        target.read_text(encoding="utf-8").replace(fragment, "contract clause removed"),
                        encoding="utf-8",
                    )
                    errors = check_oidc_broker_contract.validate(root)
                    self.assertTrue(any(relative in error and fragment in error for error in errors))

    def test_local_dex_cannot_become_the_production_authority(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.write_contract(root)
            config = root / "deploy/dex/config.yaml"
            config.write_text(
                config.read_text(encoding="utf-8").replace(
                    "issuer: http://localhost:5556/dex",
                    "issuer: https://identity.hourpaths.example",
                ),
                encoding="utf-8",
            )
            self.assertTrue(any("local Dex" in error for error in check_oidc_broker_contract.validate(root)))

    def test_production_surface_cannot_route_to_dex(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.write_contract(root)
            workflow = root / ".github/workflows/release.yml"
            workflow.parent.mkdir(parents=True, exist_ok=True)
            workflow.write_text("env:\n  OIDC_ISSUER: http://dex:5556/dex\n", encoding="utf-8")
            errors = check_oidc_broker_contract.validate(root)
            self.assertTrue(any("production surface" in error and "release.yml" in error for error in errors))

    def test_dex_local_configuration_is_excluded_from_production_scan(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.write_contract(root)
            self.assertEqual(check_oidc_broker_contract.validate(root), [])


if __name__ == "__main__":
    unittest.main()
