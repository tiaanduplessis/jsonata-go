# Profile evidence

`make benchmark-profile` captures CPU, allocation, mutex, and blocking profiles
for the transform and large filter/reduce workloads. It also writes
`metadata.json` with the exact source revision and dirty status, machine,
timestamp, commands, and SHA-256 identity of every generated profile artifact.

Profile files without a matching `metadata.json` are stale and must not support
an optimization or performance claim. The checked-in profiles currently need a
fresh collection after the optimizer work and benchmark evidence revision are
final.

Hardware cache-counter collection is host-specific and must be recorded
separately when available. No cache-performance conclusion is implied by these
Go profiles.
