# Dameng image-swap MVP specification

Source of truth: [dameng-image-swap-mvp.yaml](dameng-image-swap-mvp.yaml).

This specification deliberately limits the first release to `rainbond` and
`rainbond-console`. It enables manual image replacement and component overrides
against the existing DM test instance. It does not modify `RainbondCluster`, the
Operator, ROI, Helm, or application database plugins.

The Console path also initializes its default Region through Django's ORM, so a
fresh DM database does not fall through to the legacy SQLite-only initializer.

The DM driver bundle is supplied only from the licensed DM installation during a
private image build. The bundle remains ignored by Git; credentials are supplied
at deployment time and never stored in source or build logs.
