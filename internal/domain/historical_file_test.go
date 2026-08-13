package domain

import "testing"

func TestHistoricalFileValidate(t *testing.T) {
	tests := []struct {
		name    string
		file    HistoricalFile
		wantErr bool
	}{
		{
			name: "valid zip",
			file: HistoricalFile{Year: 2025, Data: []byte{0x50, 0x4B, 0x03, 0x04}},
		},
		{
			name:    "html instead of zip",
			file:    HistoricalFile{Year: 2025, Data: []byte("<html>")},
			wantErr: true,
		},
		{
			name:    "invalid year",
			file:    HistoricalFile{Year: 1980, Data: []byte{0x50, 0x4B, 0x03, 0x04}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.file.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
