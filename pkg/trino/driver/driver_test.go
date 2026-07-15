package driver

import (
	"reflect"
	"testing"
)

func TestParseRoles(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{name: "empty", input: "", want: map[string]string{}},
		{name: "blank", input: "   ", want: map[string]string{}},
		{
			name:  "single",
			input: "system:admin",
			want:  map[string]string{"system": "admin"},
		},
		{
			name:  "multiple",
			input: "system:admin;catalog1:roleA;catalog2:roleB",
			want:  map[string]string{"system": "admin", "catalog1": "roleA", "catalog2": "roleB"},
		},
		{
			name:  "trims whitespace",
			input: " system : admin ; catalog1 : roleA ",
			want:  map[string]string{"system": "admin", "catalog1": "roleA"},
		},
		{
			name:    "missing colon",
			input:   "system-admin",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRoles(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
