# Licensing & Provenance

Paper LMS is an independent reimplementation of the Canvas REST API surface and IMS
Common Cartridge / QTI / LTI specifications. No source code from Canvas LMS
(github.com/instructure/canvas-lms, AGPLv3) was used as a base or reference while
writing this codebase.

## Compatibility ≠ derivation

Paper LMS deliberately mirrors:

- **Canvas REST API field names and enum values** (e.g., `workflow_state`, `due_at`,
  `submission_types`, `must_view`, `TeacherEnrollment`) so existing Canvas-dependent
  tools and integrations work unchanged. Per *Lotus v. Borland* and *Google v. Oracle*,
  API contracts are functional/interop and not copyrightable as such.
- **IMS Global specifications** (Common Cartridge 1.3, QTI 1.2, LTI 1.3, LTI Advantage,
  OneRoster v1.2). These are public standards.

## Third-party software

Paper LMS bundles only permissively-licensed dependencies (MIT, Apache-2.0, BSD,
ISC, MPL-2.0 file-scope). Generated `THIRD-PARTY-NOTICES` ships with the binary.
No GPL, AGPL, SSPL, or EPL dependencies are present.

## Trademarks

"Canvas" is a trademark of Instructure, Inc. Paper LMS is not affiliated with,
endorsed by, or sponsored by Instructure.
