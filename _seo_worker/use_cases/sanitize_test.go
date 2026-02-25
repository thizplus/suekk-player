package use_cases

import (
	"testing"

	"seo-worker/domain/models"
)

func TestSanitizeCastNames(t *testing.T) {
	// Mock cast data
	casts := []models.CastMetadata{
		{ID: "1", Name: "Zemba Mami", Slug: "zemba-mami"},
		{ID: "2", Name: "Yua Mikami", Slug: "yua-mikami"},
		{ID: "3", Name: "Aoi Satomi", Slug: "aoi-satomi"},
	}

	castNameMap := buildCastNameMap(casts)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Pattern: Thai + English
		{"Thai first + EN last", "เซ็นมะ Mami ในเรื่องนี้", "Zemba Mami ในเรื่องนี้"},
		{"Thai first + EN last 2", "ยัว Mikami แสดงได้ดี", "Yua Mikami แสดงได้ดี"},
		
		// Pattern: English + Thai  
		{"EN first + Thai last", "Zemba มามิ คือนักแสดง", "Zemba Mami คือนักแสดง"},
		{"EN first + Thai last 2", "Yua มิคามิ สวยมาก", "Yua Mikami สวยมาก"},
		
		// Already correct - should not change
		{"Already correct", "Zemba Mami is great", "Zemba Mami is great"},
		
		// Pure Thai - should not change (not mixed)
		{"Pure Thai", "เซมบะ มามิ แสดงดี", "เซมบะ มามิ แสดงดี"},
		
		// Multiple casts in one text
		{"Multiple casts", "เซ็นมะ Mami และ ยัว Mikami", "Zemba Mami และ Yua Mikami"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, count := sanitizeTextWithCastNames(tt.input, castNameMap)
			if result != tt.expected {
				t.Errorf("\nInput:    %q\nExpected: %q\nGot:      %q\nCount:    %d", 
					tt.input, tt.expected, result, count)
			}
		})
	}
}

func TestMixedNameRegex(t *testing.T) {
	tests := []struct {
		input   string
		matches []string
	}{
		{"เซ็นมะ Mami", []string{"เซ็นมะ Mami"}},
		{"Zemba มามิ", []string{"Zemba มามิ"}},
		{"hello world", nil},
		{"สวัสดี ครับ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := mixedNameRegex.FindAllString(tt.input, -1)
			if len(matches) != len(tt.matches) {
				t.Errorf("Input: %q\nExpected matches: %v\nGot: %v", 
					tt.input, tt.matches, matches)
			}
		})
	}
}
