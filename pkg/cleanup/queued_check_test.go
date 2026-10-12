package cleanup

import "testing"

// capability_id: rainbond.cleanup.package-check-queue-classification
func TestPackageChecksUseSharedQueueClassification(t *testing.T) {
	for _, test := range []struct {
		body      string
		protected bool
	}{
		{`{"uuid":"id","source_type":"package_build","source_body":"{}"}`, true},
		{`{"uuid":"id","source_type":"sourcecode","source_body":"{\"server_type\":\"pkg\"}"}`, true},
		{`{"uuid":"id","source_type":"sourcecode","source_body":"{\"server_type\":\"git\"}"}`, false},
		{`{"uuid":"id","source_type":"docker-compose","event_id":"owned"}`, true},
		{`{"uuid":"id","source_type":"docker-run","source_body":"event owned"}`, true},
		{`{"uuid":"id","source_type":"docker-run","source_body":"docker run nginx"}`, false},
	} {
		id, protected, err := PackageCheckIdentity([]byte(test.body))
		if err != nil || id != "id" || protected != test.protected {
			t.Fatal("incorrect package classification", protected, err)
		}
	}
	if _, _, err := PackageCheckIdentity([]byte(`{"uuid":"../escape"}`)); err == nil {
		t.Fatal("invalid identity accepted")
	}
}
