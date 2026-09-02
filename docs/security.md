# Security Principles

This document records the security and privacy constraints for implementation.
The complete normative design is in [design-research.md](design-research.md).

## Least privilege

Azerlay must observe only supported Azeron event nodes. Production code must not:

- require root;
- require permanent membership in the `input` group;
- grab, remap, or inject input;
- use `uinput`; or
- monitor general keyboards.

Device access will use narrowly targeted udev `uaccess` rules after supported
device identifiers are verified.

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

The project license is not finalized. MPL-2.0 remains a recommendation pending
the public-release dependency-license, NOTICE, and SBOM review.
