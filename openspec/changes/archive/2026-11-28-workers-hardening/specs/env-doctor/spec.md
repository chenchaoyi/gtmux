## ADDED Requirements

### Requirement: Atomic single-step release reads and cached verification keys

The public release download worker SHALL memoize imported RSA-3072 verification keys per public key and serve standard `GET` requests through one atomic R2 object read without a preceding `HEAD` check, preventing spurious `503` errors during concurrent `stable.json` promotion while caching only immutable versioned `200` `GET` responses at the edge.

#### Scenario: Concurrent stable manifest promotion during download reads

- **WHEN** `/gtmux/stable.json` is promoted while clients perform standard `GET` reads
- **THEN** each `GET` executes a single atomic `RELEASES.get` without `RELEASES.head` or spurious `503` errors, and `CryptoKey` import runs once per isolate per key

#### Scenario: Immutable release assets use edge cache while stable stays live

- **WHEN** unconditional `GET` requests read `/gtmux/vX.Y.Z/<asset>` versus `/gtmux/stable.json`
- **THEN** versioned `200` responses reuse the edge cache when available, `HEAD`/`Range`/`416`/conditional semantics hold, and `stable.json` always reflects the latest promoted manifest
