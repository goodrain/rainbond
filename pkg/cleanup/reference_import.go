package cleanup

import (
	"encoding/json"
	"strings"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

// Import receipts remain conservative references until their owning workflow
// explicitly removes them. No inferred age or missing data expires a reference.
func inspectImportReferences(tx *gorm.DB, inspect func(string)) (bool, error) {
	rows, err := tx.Model(&model.KeyValue{}).Select("k, CASE WHEN LENGTH(v) <= 1048576 THEN v ELSE '' END").Where("k LIKE ? OR k LIKE ?", "/rainbond/tarload/%", "/servicecheck/%").Limit(20001).Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()
	complete := true
	count := 0
	for rows.Next() {
		count++
		if count > 20000 {
			return false, ErrCoordinationUnavailable
		}
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return false, err
		}
		if !inspectImportReceipt(key, value, inspect) {
			complete = false
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return complete, nil
}

func inspectImportReceipt(key, value string, inspect func(string)) bool {
	var marker struct {
		Handoff string `json:"cleanup_version_handoff"`
	}
	if json.Unmarshal([]byte(value), &marker) == nil && marker.Handoff != "" {
		_, fingerprint, valid := importReceiptImages(key, value)
		if valid && marker.Handoff == fingerprint {
			return true
		}
	}
	return inspectPendingImportReceipt(key, value, inspect)
}

func inspectPendingImportReceipt(key, value string, inspect func(string)) bool {
	if strings.HasPrefix(key, "/rainbond/tarload/") {
		var record struct {
			Status  string            `json:"status"`
			Images  []string          `json:"images"`
			Targets map[string]string `json:"target_images"`
		}
		if json.Unmarshal([]byte(value), &record) != nil {
			return false
		}
		complete := true
		for _, target := range record.Targets {
			if target == "" {
				complete = false
			} else {
				inspect(target)
			}
		}
		switch record.Status {
		case "success":
			return complete && len(record.Targets) > 0
		case "failure":
			return complete && len(record.Images) == 0 && len(record.Targets) == 0
		default:
			return false
		}
	}
	var record struct {
		Status   string `json:"check_status"`
		Services []struct {
			TarImages []struct {
				Name   string `json:"name"`
				Prefix string `json:"prefix"`
			} `json:"tar_images"`
		} `json:"service_info"`
	}
	if json.Unmarshal([]byte(value), &record) != nil {
		return false
	}
	if record.Status != "Success" && record.Status != "Failure" {
		return false
	}
	hasTar := false
	for _, service := range record.Services {
		for _, image := range service.TarImages {
			hasTar = true
			parts := strings.Split(image.Name, "/")
			// Match the legacy parser's one-, two- and three-segment destination
			// mapping. Unsupported shapes must not be guessed into a safe inventory.
			if image.Prefix == "" || len(parts) > 3 || parts[len(parts)-1] == "" {
				return false
			}
			inspect(strings.TrimSuffix(image.Prefix, "/") + "/" + parts[len(parts)-1])
		}
	}
	return !hasTar || record.Status == "Success"
}
