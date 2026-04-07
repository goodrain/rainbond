package multi

import "testing"

// capability_id: rainbond.multisvc.select-java-maven
func TestNewMultiServiceI_SupportsCompositeJavaMaven(t *testing.T) {
	tests := []struct {
		name string
		lang string
	}{
		{name: "plain java maven", lang: "Java-maven"},
		{name: "dockerfile first", lang: "dockerfile,Java-maven"},
		{name: "java maven first", lang: "Java-maven,dockerfile"},
		{name: "with spaces", lang: " dockerfile , Java-maven "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMultiServiceI(tt.lang); got == nil {
				t.Fatalf("NewMultiServiceI(%q) = nil, want maven parser", tt.lang)
			}
		})
	}
}

// capability_id: rainbond.multisvc.ignore-non-java
func TestNewMultiServiceI_IgnoresLanguagesWithoutJavaMaven(t *testing.T) {
	for _, lang := range []string{"dockerfile", "Node.js", "dockerfile,Node.js"} {
		t.Run(lang, func(t *testing.T) {
			if got := NewMultiServiceI(lang); got != nil {
				t.Fatalf("NewMultiServiceI(%q) = %T, want nil", lang, got)
			}
		})
	}
}
