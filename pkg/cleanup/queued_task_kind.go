package cleanup

// QueuedTaskKind maps actual native consumers to their admission identity.
// Tar import and service checks use body-owned IDs and reserve in their API
// handlers, so they must not receive a second MQ-ID reservation here.
func QueuedTaskKind(taskType string) string {
	switch taskType {
	case "build_from_image":
		return "image"
	case "build_from_source_code":
		return "source"
	case "build_from_vm":
		return "vm"
	case "plugin_image_build":
		return "plugin-image"
	case "plugin_dockerfile_build":
		return "plugin-dockerfile"
	case "share-image":
		return "image-share"
	case "import_app":
		return "import_app"
	case "backup_apps_restore":
		return "backup_apps_restore"
	}
	return ""
}

func queuedNativeKind(kind string) bool {
	switch kind {
	case "tar-image", "service-check", "image", "source", "vm", "plugin-image", "plugin-dockerfile", "image-share", "import_app", "backup_apps_restore":
		return true
	}
	return false
}
