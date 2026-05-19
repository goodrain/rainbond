package volume

import (
	"strings"

	kubevirtv1 "kubevirt.io/api/core/v1"
)

func VMVolumeDeviceType(volumePath string) string {
	switch {
	case volumePath == "/disk" || strings.HasPrefix(volumePath, "/disk-"):
		return "disk"
	case volumePath == "/lun" || strings.HasPrefix(volumePath, "/lun-"):
		return "lun"
	case volumePath == "/cdrom" || strings.HasPrefix(volumePath, "/cdrom-"):
		return "cdrom"
	default:
		return ""
	}
}

func VMVolumeDiskBus(volumePath string) kubevirtv1.DiskBus {
	switch {
	case volumePath == "/disk":
		return kubevirtv1.DiskBusSATA
	case strings.HasPrefix(volumePath, "/disk-"):
		return kubevirtv1.DiskBusSCSI
	case volumePath == "/lun" || strings.HasPrefix(volumePath, "/lun-"):
		return kubevirtv1.DiskBusSCSI
	case volumePath == "/cdrom" || strings.HasPrefix(volumePath, "/cdrom-"):
		return kubevirtv1.DiskBusSATA
	default:
		return kubevirtv1.DiskBusSATA
	}
}

func BuildVMDiskDevice(volumePath string) kubevirtv1.DiskDevice {
	bus := VMVolumeDiskBus(volumePath)
	switch VMVolumeDeviceType(volumePath) {
	case "disk":
		return kubevirtv1.DiskDevice{
			Disk: &kubevirtv1.DiskTarget{
				Bus: bus,
			},
		}
	case "lun":
		return kubevirtv1.DiskDevice{
			LUN: &kubevirtv1.LunTarget{
				Bus: bus,
			},
		}
	case "cdrom":
		return kubevirtv1.DiskDevice{
			CDRom: &kubevirtv1.CDRomTarget{
				Bus: bus,
			},
		}
	default:
		return kubevirtv1.DiskDevice{}
	}
}
