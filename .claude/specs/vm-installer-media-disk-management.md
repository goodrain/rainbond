# VM Installer Media Disk Management Spec

- Design doc: [2026-05-11-vm-installer-media-disk-management-design.md](/Users/zhangqihang/MyWork/workrc/rainbond/docs/plans/2026-05-11-vm-installer-media-disk-management-design.md:1)
- YAML spec: [vm-installer-media-disk-management.yaml](/Users/zhangqihang/MyWork/workrc/rainbond/.claude/specs/vm-installer-media-disk-management.yaml:1)

## Goal

Expose ISO installer media as a visible VM disk item, allow disk ordering and installer removal from the component storage page, and make the saved layout take effect on the next restart.

## Commit Groups

1. `feat: add vm disk layout APIs`
   - Add console VM disk list/save APIs
   - Switch UI VM storage page to use full VM disk list semantics

2. `feat: honor installer media in vm disk layout`
   - Update rainbond worker VM layout parsing so installer media is controlled by `vm_disk_layout`
   - Preserve imported-disk and HTTP import behavior

3. `test: verify vm installer media disk management flow`
   - Run cross-repo verification gates
