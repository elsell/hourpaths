#!/usr/bin/env python3
import base64
import json
import sys
import uuid


def fail(message: str) -> int:
    print(message, file=sys.stderr)
    return 2


def main() -> int:
    if sys.argv[1:] == ["subject"]:
        token = sys.stdin.read()
        try:
            segments = token.split(".")
            if len(segments) != 3:
                return fail("invalid identity token")
            payload = segments[1] + "=" * (-len(segments[1]) % 4)
            subject = json.loads(base64.urlsafe_b64decode(payload))["sub"]
            if not isinstance(subject, str) or not subject:
                return fail("identity token subject is invalid")
        except (KeyError, TypeError, ValueError, json.JSONDecodeError):
            return fail("invalid identity token")
        print(subject)
        return 0
    if len(sys.argv) == 4 and sys.argv[1] == "user-id":
        print(uuid.uuid5(uuid.NAMESPACE_URL, sys.argv[2] + "\0" + sys.argv[3]))
        return 0
    return fail("usage: identity-fixture.py subject | user-id ISSUER SUBJECT")


if __name__ == "__main__":
    raise SystemExit(main())
