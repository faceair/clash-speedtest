package main

import (
	"testing"
)

func TestGenerateNodeName(t *testing.T) {
	nameCount := make(map[string]int)

	// Test first occurrence - no suffix
	name1 := generateNodeName("US", 10*1024*1024, nameCount)
	expected1 := "🇺🇸 US | ⬇️ 10.00 MB/s"
	if name1 != expected1 {
		t.Errorf("Expected %s, got %s", expected1, name1)
	}

	// Test second occurrence - should have -01 suffix
	name2 := generateNodeName("US", 10*1024*1024, nameCount)
	expected2 := "🇺🇸 US | ⬇️ 10.00 MB/s-01"
	if name2 != expected2 {
		t.Errorf("Expected %s, got %s", expected2, name2)
	}

	// Test third occurrence - should have -02 suffix
	name3 := generateNodeName("US", 10*1024*1024, nameCount)
	expected3 := "🇺🇸 US | ⬇️ 10.00 MB/s-02"
	if name3 != expected3 {
		t.Errorf("Expected %s, got %s", expected3, name3)
	}

	// Test different country - no suffix
	name4 := generateNodeName("HK", 5*1024*1024, nameCount)
	expected4 := "🇭🇰 HK | ⬇️ 5.00 MB/s"
	if name4 != expected4 {
		t.Errorf("Expected %s, got %s", expected4, name4)
	}

	// Test different speed - no suffix
	name5 := generateNodeName("US", 15*1024*1024, nameCount)
	expected5 := "🇺🇸 US | ⬇️ 15.00 MB/s"
	if name5 != expected5 {
		t.Errorf("Expected %s, got %s", expected5, name5)
	}
}

func TestGenerateNodeNameUnknownCountry(t *testing.T) {
	nameCount := make(map[string]int)

	name := generateNodeName("XX", 10*1024*1024, nameCount)
	expected := "🏳️ XX | ⬇️ 10.00 MB/s"
	if name != expected {
		t.Errorf("Expected %s, got %s", expected, name)
	}
}
