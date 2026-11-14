package conversion

import "testing"

// capability_id: rainbond.vm-restore.worker-startup-filter
func TestRestoredRuntimeDoesNotReuseBLoginInputsOrMutateSavedExtensions(t *testing.T) {
	saved := map[string]string{"vm_ssh_username": "B_USER_SENTINEL", "vm_login_password_hash": "B_PRIVATE_SENTINEL", "vm_ssh_authorized_key": "B_KEY_SENTINEL", "vm_login_password": "B_PRIVATE_SENTINEL", "vm_boot_mode": "uefi", "vm_fixed_ip": "192.0.2.4"}
	filtered := vmRuntimeExtensionsForRestoredDisk(saved)
	for _, key := range []string{"vm_ssh_username", "vm_login_password_hash", "vm_ssh_authorized_key", "vm_login_password"} {
		if _, exists := filtered[key]; exists {
			t.Fatal("B login input survived filter", key)
		}
		if saved[key] == "" {
			t.Fatal("saved platform input was modified", key)
		}
	}
	if filtered["vm_boot_mode"] != "uefi" || filtered["vm_fixed_ip"] != saved["vm_fixed_ip"] {
		t.Fatal("unrelated runtime settings lost")
	}
}
