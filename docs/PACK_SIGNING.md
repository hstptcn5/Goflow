# Goflow Pack Signing Foundation v1

Status: offline development foundation; no production keys or trust service.

Checksums and `PACK_INFO.json` prove that bundle bytes match an inventory. They
do not identify a publisher. A signature proves only that a holder of a trusted
private key approved one exact bundle identity and inventory. Trust still
depends on how the operator obtained, reviewed, rotated, or revoked that key.

## Container

A signed ZIP adds exactly one root member named `PACK_SIGNATURE.json`. It is not
listed in `PACK_INFO.files`; listing it would make the signature depend on
itself. Unsigned bundles omit the member. More than one member, a directory,
symlink, duplicate/case-collision, compressed bomb, or metadata larger than
16 KiB is invalid.

Schema v1 fields are strict and bounded:

```json
{
  "schema_version": 1,
  "algorithm": "ed25519",
  "key_id": "publisher.example.dev-2026",
  "pack_id": "example.pack",
  "pack_version": "1.2.3",
  "target": "windows-amd64",
  "required_capabilities": ["goflow.pack.v1"],
  "pack_info_sha256": "64 lowercase hex characters",
  "signature": "base64 Ed25519 signature"
}
```

`key_id` is a label selected by the publisher and matched against the explicit
trusted key input. It is not a certificate, authority claim, URL, or embedded
public key. Unknown schema/algorithm/fields and duplicate capability entries
fail closed. Exact Pack version and target are signed; downgrade acceptance is
therefore a separate operator policy and is not automatic.

## Canonical Signed Bytes

The signature is Ed25519 over UTF-8 bytes assembled without JSON canonicalizer
ambiguity. Every string is length-prefixed as unsigned 64-bit big-endian bytes.
Capabilities retain manifest order and are encoded as a length-prefixed count
followed by length-prefixed entries.

```text
"goflow-pack-signature-v1\n"
u64(schema_version)
string(algorithm)
string(key_id)
string(pack_id)
string(pack_version)
string(target)
u64(capability_count)
string(capability_0) ... string(capability_n)
bytes32(decoded pack_info_sha256)
```

The `PACK_INFO.json` digest covers its exact bytes. `PACK_INFO.files` in turn
covers the runtime, runtime manifest, workflow, assets, and plugin resources.
This avoids ZIP ordering/compression/timestamp ambiguity while binding the full
verified payload. The canonical payload is deterministic for equal metadata.

## Signing And Verification Order

1. Build and verify the unsigned ZIP using existing bounded inventory checks.
2. Refuse an existing signature member.
3. Read bounded `PACK_INFO.json`; bind its exact SHA-256 and identity fields.
4. Read an Ed25519 private key only from an explicit local file or stdin; never
   print, copy, persist, inventory, or include it in output.
5. Write a new ZIP atomically with the existing members plus one deterministic
   `PACK_SIGNATURE.json`, then re-run integrity and signature verification.
6. Verification first checks ZIP inventory/limits, then strict signature
   metadata and identity, then matches `key_id` to an explicit trusted Ed25519
   public key and verifies canonical bytes.

The verifier never trusts a key contained in a Pack, downloads a key, performs
trust-on-first-use, contacts a registry, or changes a trust store. Key rotation
means operators explicitly replace/add a reviewed key ID. Revocation and
publisher governance remain external prerequisites for production acceptance.

## Unsigned Policy

Existing `pack verify` remains an integrity command and reports unsigned state.
Signature-required verification must be explicit and fails unsigned bundles.
Current alpha/beta appliance execution may continue only under its documented
unsigned development policy and must retain an `UNSIGNED-*` warning. No signed
release or authenticity claim is created by this foundation.
