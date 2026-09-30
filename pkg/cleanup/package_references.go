package cleanup

import (
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

var uploadReferenceID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

func uploadReferenceEvent(value string) (string, error) {
	parts := strings.Split(value, "/")
	event := ""
	if len(parts) == 6 && parts[0] == "" && parts[1] == "grdata" && parts[2] == "package_build" && parts[3] == "temp" && parts[4] == "events" {
		event = parts[5]
	}
	if len(parts) == 7 && parts[0] == "" && parts[1] == "grdata" && parts[2] == "package_build" && parts[3] == "components" && uploadReferenceID.MatchString(parts[4]) && parts[5] == "events" {
		event = parts[6]
	}
	if !uploadReferenceID.MatchString(event) {
		return "", ErrCoordinationChanged
	}
	return event, nil
}

// ReadUploadPackageReferences returns positive protection evidence from retained
// versions and source checks. False does not prove absence of Console, queued
// task or other references and never authorizes deletion. Saved URLs are not
// exposed, and only package-prefixed paths are selected from the database.
func ReadUploadPackageReferences(database *gorm.DB, eventIDs []string) (map[string]bool, error) {
	if database == nil || len(eventIDs) == 0 || len(eventIDs) > 50 {
		return nil, ErrCoordinationChanged
	}
	refs := make(map[string]bool, len(eventIDs))
	for _, id := range eventIDs {
		if !uploadReferenceID.MatchString(id) {
			return nil, ErrCoordinationChanged
		}
		if _, ok := refs[id]; ok {
			return nil, ErrCoordinationChanged
		}
		refs[id] = false
	}
	queries := []struct {
		model  interface{}
		column string
	}{{&model.VersionInfo{}, "repo_url"}, {&model.CodeCheckResult{}, "git_url"}}
	for _, query := range queries {
		rows, err := database.Model(query.model).Select(query.column).Where(query.column+" LIKE ?", "/grdata/package_build/%").Limit(20001).Rows()
		if err != nil {
			return nil, err
		}
		count := 0
		for rows.Next() {
			count++
			if count > 20000 {
				rows.Close()
				return nil, ErrCoordinationUnavailable
			}
			var value string
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				return nil, err
			}
			event, err := uploadReferenceEvent(value)
			if err != nil {
				rows.Close()
				return nil, err
			}
			if _, ok := refs[event]; ok {
				refs[event] = true
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return refs, nil
}
