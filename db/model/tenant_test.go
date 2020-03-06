package model

import (
	"testing"
)

func TestTenantServices_IsState(t *testing.T) {
	type fields struct {
		ExtendMethod string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name:   "is state",
			fields: fields{ExtendMethod: ServiceTypeStateSingleton.String()},
			want:   true,
		},
		{
			name:   "is state",
			fields: fields{ExtendMethod: ServiceTypeStateMultiple.String()},
			want:   true,
		},
		{
			name:   "not state",
			fields: fields{ExtendMethod: ServiceTypeStatelessSingleton.String()},
			want:   false,
		},
		{
			name:   "not state",
			fields: fields{ExtendMethod: ServiceTypeStatelessMultiple.String()},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TenantServices{
				ExtendMethod: tt.fields.ExtendMethod,
			}
			if got := ts.IsState(); got != tt.want {
				t.Errorf("IsState() = %v, want %v", got, tt.want)
			}
		})
	}
}
