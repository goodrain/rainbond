# Dameng ISO bundle specification

Source of truth: [dameng-iso-bundle.yaml](dameng-iso-bundle.yaml).

The local DM8 ISO is a one-time, licensed source input. The implementation must
extract only Go archives, Python source, and DPI files into a small private OCI
build context. Normal Rainbond image actions use that context automatically;
platform deployment remains `DB_TYPE=dm` plus the ordinary database connection
configuration.
