# Releasing

The module tag is the publication event. Verify the exact candidate before
creating that tag.

1. Choose the commit to publish and run **Pre-release verification** from the
   repository Actions tab. Supply that exact branch, commit, or existing tag
   as `ref`, and the intended `vX.Y.Z` value as `version`.
2. Wait for the workflow to pass. It runs the source, conformance,
   differential, security, documentation, oracle, module, and deterministic
   benchmark-evidence integrity gates. It validates committed benchmark
   correctness and provenance without collecting new performance timings on a
   hosted runner. A hardware-dependent fastest-library result is not a
   release requirement; any performance claim remains valid only when the
   committed report's scoped claim gate says it is met.
3. From a trusted checkout at the verified commit, create and inspect a
   signed annotated tag, then push it:

   ```sh
   git checkout --detach <verified-commit>
   git tag -s -a vX.Y.Z <verified-commit> -m "Release vX.Y.Z"
   git tag -v vX.Y.Z
   git push origin vX.Y.Z
   ```

   The tag must point to the commit verified in step 2. Do not move or reuse
   a published version tag. Repository tag protection and the maintainer's
   signature-verification policy are part of this handoff.

The tag-triggered workflow runs after the tag has been published. It
revalidates the tag, checks the signed annotated tag shape, packages the
module, validates `proxy.golang.org`, signs checksums with Sigstore, and
attests release archives. It cannot prevent a bad tag from already being
published, so it is not a substitute for the pre-release workflow.
