package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildShellPodGenerateNameLowercasesRegionName(t *testing.T) {
	generateName := buildShellPodGenerateName("GXZY-K8S")

	assert.Equal(t, "shell-gxzy-k8s-", generateName)
}

func TestValidatePEMCertificates_EmptyInputs(t *testing.T) {
	tests := []struct {
		name    string
		caPEM   []byte
		certPEM []byte
		keyPEM  []byte
		wantErr string
	}{
		{
			name:    "empty ca",
			caPEM:   []byte{},
			certPEM: []byte("data"),
			keyPEM:  []byte("data"),
			wantErr: "ca.pem is empty",
		},
		{
			name:    "empty cert",
			caPEM:   []byte("data"),
			certPEM: []byte{},
			keyPEM:  []byte("data"),
			wantErr: "server.pem is empty",
		},
		{
			name:    "empty key",
			caPEM:   []byte("data"),
			certPEM: []byte("data"),
			keyPEM:  []byte{},
			wantErr: "server.key.pem is empty",
		},
		{
			name:    "nil ca",
			caPEM:   nil,
			certPEM: []byte("data"),
			keyPEM:  []byte("data"),
			wantErr: "ca.pem is empty",
		},
		{
			name:    "invalid pem block",
			caPEM:   []byte("not-a-pem-block"),
			certPEM: []byte("not-a-pem-block"),
			keyPEM:  []byte("not-a-pem-block"),
			wantErr: "no valid PEM block found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePEMCertificates(tt.caPEM, tt.certPEM, tt.keyPEM)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
