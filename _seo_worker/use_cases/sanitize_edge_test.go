package use_cases

import (
	"testing"

	"seo-worker/domain/models"
)

func TestSanitizeEdgeCases(t *testing.T) {
	casts := []models.CastMetadata{
		{ID: "1", Name: "Zemba Mami", Slug: "zemba-mami"},
	}

	castNameMap := buildCastNameMap(casts)

	tests := []struct {
		name     string
		input    string
		expected string
		shouldFix bool
	}{
		// No space between Thai and English
		{"No space Thai+EN", "เซ็นมะMami ในเรื่องนี้", "Zemba Mami ในเรื่องนี้", true},
		
		// Text with video code (should not break)
		{"With video code", "DLDSS-471 เซ็นมะ Mami แสดง", "DLDSS-471 Zemba Mami แสดง", true},
		
		// In middle of sentence
		{"In sentence", "การแสดงของ เซ็นมะ Mami ในฉากนี้ดีมาก", "การแสดงของ Zemba Mami ในฉากนี้ดีมาก", true},
		
		// Repeated name
		{"Repeated", "เซ็นมะ Mami และ เซ็นมะ Mami", "Zemba Mami และ Zemba Mami", true},
		
		// Unknown cast (should not change)
		{"Unknown cast", "ซากุระ Tanaka สวย", "ซากุระ Tanaka สวย", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, count := sanitizeTextWithCastNames(tt.input, castNameMap)
			
			if tt.shouldFix && result == tt.input {
				t.Errorf("Expected fix but got same:\nInput:  %q\nResult: %q", tt.input, result)
			}
			
			if result != tt.expected {
				t.Errorf("\nInput:    %q\nExpected: %q\nGot:      %q\nCount:    %d", 
					tt.input, tt.expected, result, count)
			}
		})
	}
}
