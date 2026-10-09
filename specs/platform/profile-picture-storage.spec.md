# Profile picture storage and processing

The [profile specification](../accounts/profile.spec.md) owns user behavior.

- Image parsing must be behind an injected processing port. Application and
  domain code must not import codec, HTTP, filesystem or persistence adapters.
- Requests must enforce the 10-megabyte decoded-upload limit before decoding.
  Processing must identify actual JPEG, PNG, WebP or HEIC content, reject other
  formats, bound decoded dimensions and processing resources, and fail without
  modifying the profile when processing cannot complete safely.
- Processing must apply supported orientation transforms before cropping. Crop
  coordinates must be validated against the normalized image dimensions. Output
  must be a square, application-encoded JPEG with no copied EXIF, XMP or other
  source metadata. Original uploads must not be stored or logged.
- Preview is an authenticated, rate-limited, audited operation. It must not write
  a saved picture. Picture writes must also have a feature-level abuse limit.
- Store the bounded normalized picture with its owner in PostgreSQL. Updating
  the current picture reference, revision, immutable owner-scoped retry record,
  and successful audit event must be one transaction. A failed transaction must
  preserve the old picture. Removal and replacement must remove prior image
  bytes. Deleting the account must cascade to picture bytes and retry records.
- Picture delivery must use an opaque, versioned application HTTPS media URL.
  It must return only the currently retained, normalized image for an active
  account, never an original upload, provider credential or arbitrary file.
  Missing, removed and deleted-account images must fail indistinguishably.
- Media responses must use the exact JPEG content type, nosniff and no-store.
  Delivery must not redirect to provider URLs or expose account identity in
  response metadata. Existing public-avatar clients make anonymous image
  requests; this narrow media delivery capability must not grant profile/API
  access and must retain request abuse controls.
- Native and Studio clients must use shared picture-editing orchestration with
  explicit picker/preview adapters. File bytes, temporary previews and crop
  drafts must stay within their owning account lifetime and be discarded on
  cancel, sign-out or account replacement.
- Dependency versions must be pinned and pass the existing age and security
  gates. Image-boundary verification must cover malformed and over-budget input,
  unsupported content, orientation, crop bounds, metadata removal, authorization,
  replay, audit rollback and account-deletion cleanup.
- Native selection must use the system photo picker with square crop editing,
  without requesting camera, microphone or broad photo-library access. Any
  app-cache file returned by the picker must be removed after its bytes are
  read; cancellation and account changes must not retain upload drafts.
- Development clients may resolve recognized application-picture identifiers
  against their explicitly configured development API origin. Production media
  transport must remain HTTPS.
