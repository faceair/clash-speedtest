package main

import (
	"net/url"
	"os"
	"testing"
)

func TestDifferentTypesOfParsing(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test20230815114.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name()) // Clean up

	testCases := []struct {
		pv        string
		isUriType bool
		expectErr bool
	}{
		{
			pv:        "http://example.com",
			isUriType: true,
			expectErr: false,
		},
		{
			pv:        "thisisnotavaliduri",
			isUriType: true,
			expectErr: true,
		},
		{
			pv:        tempFile.Name(),
			isUriType: false,
			expectErr: false,
		},
		{
			pv:        "non_existent_file.txt",
			isUriType: false,
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		result := DifferentTypesOfParsing(tc.pv, tc.isUriType)
		if tc.expectErr && result != nil {
			t.Errorf("Expected error for pv %s, but got no error", tc.pv)
		}
		if !tc.isUriType && result != nil {
			_, ok := result.(os.File)
			if !ok {
				t.Errorf("Expected FileInfo type for pv %s", tc.pv)
			}
		}
		if tc.isUriType && result != nil {
			_, ok := result.(*url.URL)
			if !ok {
				t.Errorf("Expected URL type for pv %s", tc.pv)
			}
		}
	}
}
