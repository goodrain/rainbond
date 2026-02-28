package code

import "testing"

func TestGetCNBVersions(t *testing.T) {
	tests := []struct {
		name      string
		lang      string
		wantCount int // 0 means empty
	}{
		// Single languages
		{"nodejs returns versions", "nodejs", len(cnbNodeVersions)},
		{"Node.js returns versions", "Node.js", len(cnbNodeVersions)},
		{"node returns versions", "node", len(cnbNodeVersions)},
		{"NODEJS returns versions (case-insensitive)", "NODEJS", len(cnbNodeVersions)},
		{"python returns empty", "python", 0},
		{"dockerfile returns empty", "dockerfile", 0},
		{"empty string returns empty", "", 0},
		// Composite languages (comma-separated)
		{"dockerfile,Node.js returns versions", "dockerfile,Node.js", len(cnbNodeVersions)},
		{"Node.js,dockerfile returns versions", "Node.js,dockerfile", len(cnbNodeVersions)},
		{"dockerfile,nodejs returns versions", "dockerfile,nodejs", len(cnbNodeVersions)},
		{"dockerfile,static returns empty", "dockerfile,static", 0},
		{"Dockerfile,Node.js (mixed case)", "Dockerfile,Node.js", len(cnbNodeVersions)},
		// Whitespace around parts
		{"dockerfile, Node.js (space after comma)", "dockerfile, Node.js", len(cnbNodeVersions)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCNBVersions(tt.lang)
			if len(got) != tt.wantCount {
				t.Errorf("GetCNBVersions(%q) returned %d versions, want %d", tt.lang, len(got), tt.wantCount)
			}
		})
	}
}

func TestMatchCNBVersion_CompositeLanguage(t *testing.T) {
	tests := []struct {
		name        string
		lang        string
		versionSpec string
		want        string
	}{
		// Composite language should resolve version correctly
		{"dockerfile,Node.js with major 20", "dockerfile,Node.js", "20", "20.20.0"},
		{"dockerfile,Node.js with major 22", "dockerfile,Node.js", "22", "22.22.0"},
		{"dockerfile,Node.js with exact", "dockerfile,Node.js", "20.19.6", "20.19.6"},
		{"dockerfile,Node.js empty spec returns default", "dockerfile,Node.js", "", "24.13.0"},
		// Single language still works
		{"nodejs with major 20", "nodejs", "20", "20.20.0"},
		{"Node.js with fuzzy 20.x", "Node.js", "20.x", "20.20.0"},
		{"Node.js with >=22", "Node.js", ">=22", "22.22.0"},
		// Unsupported language returns empty
		{"python returns empty", "python", "3.11", ""},
		{"dockerfile alone returns empty", "dockerfile", "20", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchCNBVersion(tt.lang, tt.versionSpec)
			if got != tt.want {
				t.Errorf("MatchCNBVersion(%q, %q) = %q, want %q", tt.lang, tt.versionSpec, got, tt.want)
			}
		})
	}
}

func TestExtractMajorFromSpec(t *testing.T) {
	tests := []struct {
		spec string
		want int
	}{
		{"20", 20},
		{"20.x", 20},
		{">=20.0", 20},
		{"^22.0.0", 22},
		{"~18.10", 18},
		{"v24", 24},
		{"=20.0.0", 20},
		{"", 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got := extractMajorFromSpec(tt.spec)
			if got != tt.want {
				t.Errorf("extractMajorFromSpec(%q) = %d, want %d", tt.spec, got, tt.want)
			}
		})
	}
}
