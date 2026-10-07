# Security Principles

This document records the security and privacy constraints for implementation.
The complete normative design is in [design-research.md](design-research.md).

## Least privilege

Azerlay must observe only the qualified Azeron interface04 hidraw node. Production code must not:

- require root;
- require permanent membership in the `input` group;
- grab, remap, or inject input;
- use `uinput`; or
- monitor general keyboards.

Device access uses read-only, nonblocking, close-on-exec descriptors. Only passive
reports and OS-held identity/report-descriptor getters are permitted. No HID
writes, Feature/Output requests, GET_INPUT requests, onboard profile access or
evdev fallback are allowed, including in diagnostics.

Native packaging supplies `71-azerlay.rules`, restricted to hidraw, USB
`16d0:12f7:0111`, interface04. The Arch package installs it; manual installation
requires an explicit administrator action. Application commands do not install
rules or repair permissions.
A uaccess ACL does not itself enforce read-only opens; that is the application's
contract. Existing broad host grants do not prove least-privilege packaging.

## Untrusted profile input

Imported profile data is untrusted. Future import work must use bounded,
stage-specific parsing, reject malformed lengths and unsupported structures,
and avoid persisting partial results. Raw data remains behind versioned adapters.

## Local boundaries

Normal operation makes no external network connections and sends no telemetry.
The planned control socket is local to the current user and must use restrictive
directory and socket permissions.

## Privacy

Default logs must not contain raw exports, complete input-event streams, macro
contents, full user labels, or local Azeron database contents. Public fixtures
must be synthetic and must exclude private profile data.

## License status

Azerlay-owned material is MIT-licensed. Third-party licenses remain applicable;
see [third-party notices](../THIRD_PARTY_NOTICES.md). Public-release readiness
remains subject to the [release-safety decision](decisions/public-release-safety.md).
