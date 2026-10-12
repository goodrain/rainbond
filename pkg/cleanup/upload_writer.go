package cleanup

import "github.com/jinzhu/gorm"

// UploadWriterProtocol proves that a native API process implements upload-session
// mutation admission and retirement checks. Older registry proofs do not suffice.
const UploadWriterProtocol = "package-upload-v1"

// UploadWriterCoverageRegistered verifies every API from a trusted complete live
// coverage inspection. It does not itself list Pods or prove the set is complete.
func UploadWriterCoverageRegistered(database *gorm.DB, writers []ReferenceWriter) (bool, error) {
	if database == nil {
		return false, ErrCoordinationUnavailable
	}
	if len(writers) == 0 || len(writers) > 256 {
		return false, nil
	}
	namespace := ""
	seen := map[string]bool{}
	for _, w := range writers {
		if w.Role != "api" {
			continue
		}
		if namespace == "" {
			namespace = w.Namespace
		}
		if w.Namespace != namespace {
			return false, ErrCoordinationChanged
		}
		w.Protocol = UploadWriterProtocol
		id, _, err := w.identity()
		if err != nil || seen[id] {
			return false, ErrCoordinationChanged
		}
		seen[id] = true
		registered, err := ReferenceWriterRegistered(database, w)
		if err != nil || !registered {
			return false, err
		}
	}
	return len(seen) > 0, nil
}
