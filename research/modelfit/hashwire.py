"""Length-prefixed little-endian hashing matching forecast/feature_tape.go."""

from __future__ import annotations

import hashlib
import struct


def new_sha() -> "hashlib._Hash":
    return hashlib.sha256()


def put_u8(h, v: int) -> None:
    h.update(bytes([v & 0xFF]))


def put_u32(h, v: int) -> None:
    h.update(struct.pack("<I", v & 0xFFFFFFFF))


def put_u64(h, v: int) -> None:
    h.update(struct.pack("<Q", v & 0xFFFFFFFFFFFFFFFF))


def put_i64(h, v: int) -> None:
    put_u64(h, v)


def put_bytes(h, p: bytes) -> None:
    put_u32(h, len(p))
    h.update(p)


def put_string(h, s: str) -> None:
    put_bytes(h, s.encode("utf-8"))


def put_digest(h, d: bytes) -> None:
    if len(d) != 32:
        raise ValueError("digest must be 32 bytes")
    h.update(d)


def put_f64(h, v: float) -> None:
    put_u64(h, struct.unpack("<Q", struct.pack("<d", float(v)))[0])


def parse_digest_hex(s: str) -> bytes:
    if len(s) != 64:
        raise ValueError("digest must be 64 hex characters")
    return bytes.fromhex(s)


def digest_hex(d: bytes) -> str:
    return d.hex()
