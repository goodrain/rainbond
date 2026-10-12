package cleanup

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

const importHandoffField = "cleanup_version_handoff"

func importReceiptImages(key, value string) ([]string, string, bool) {
	images := map[string]bool{}
	valid := true
	complete := inspectPendingImportReceipt(key, value, func(image string) {
		if _, err := reference.ParseNormalizedNamed(image); err != nil {
			valid = false
		}
		images[image] = true
	})
	if !complete || !valid || len(images) == 0 {
		return nil, "", false
	}
	ordered := make([]string, 0, len(images))
	for image := range images {
		ordered = append(ordered, image)
	}
	sort.Strings(ordered)
	sum := sha256.Sum256([]byte(key + "\n" + strings.Join(ordered, "\n")))
	return ordered, hex.EncodeToString(sum[:]), true
}

// TransferImportedReferencesToVersions hands temporary protection to successful
// durable versions in the SAME reference mutation transaction. Receipts remain
// available to their original clients and duplicate task detection.
func TransferImportedReferencesToVersions(tx *gorm.DB) error {
	if _, ok := tx.CommonDB().(*sql.Tx); !ok {
		return ErrCoordinationChanged
	}
	var versions []model.VersionInfo
	current := tx
	if tx.Dialect().GetName() != "sqlite3" {
		current = tx.Set("gorm:query_option", "FOR UPDATE")
	}
	if err := current.Select("image_name, delivered_type, delivered_path").Where("final_status = ?", "success").Limit(20001).Find(&versions).Error; err != nil {
		return err
	}
	if len(versions) > 20000 {
		// Handoff is optional; keep receipts protective without failing a build.
		return nil
	}
	owned := map[string]bool{}
	for _, v := range versions {
		if v.ImageName != "" {
			owned[v.ImageName] = true
		}
		if v.DeliveredType == "image" && v.DeliveredPath != "" {
			owned[v.DeliveredPath] = true
		}
	}
	// Avoid loading unbounded receipt bodies; oversized documents retain protection.
	cursor := ""
	for seen := 0; ; {
		var receipts []model.KeyValue
		if err := tx.Select("k, CASE WHEN LENGTH(v) <= 1048576 THEN v ELSE '' END AS v").Where("(k LIKE ? OR k LIKE ?) AND k > ?", "/rainbond/tarload/%", "/servicecheck/%", cursor).Order("k").Limit(25).Find(&receipts).Error; err != nil {
			return err
		}
		if len(receipts) == 0 {
			return nil
		}
		seen += len(receipts)
		if seen > 20000 {
			return nil
		}
		cursor = receipts[len(receipts)-1].K
		for _, receipt := range receipts {
			images, fingerprint, valid := importReceiptImages(receipt.K, receipt.V)
			if !valid {
				continue
			}
			all := true
			for _, image := range images {
				if !owned[image] {
					all = false
					break
				}
			}
			if !all {
				continue
			}
			var content map[string]json.RawMessage
			if json.Unmarshal([]byte(receipt.V), &content) != nil {
				continue
			}
			var previous string
			_ = json.Unmarshal(content[importHandoffField], &previous)
			if previous == fingerprint {
				continue
			}
			content[importHandoffField], _ = json.Marshal(fingerprint)
			encoded, err := json.Marshal(content)
			if err != nil {
				return err
			}
			updated := tx.Model(&model.KeyValue{}).Where("k = ? AND v = ?", receipt.K, receipt.V).UpdateColumn("v", string(encoded))
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrCoordinationChanged
			}
		}
	}
}
