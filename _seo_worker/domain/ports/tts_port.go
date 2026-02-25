package ports

import "context"

// TTSPort - Interface สำหรับ Text-to-Speech (ElevenLabs)
type TTSPort interface {
	// GenerateAudio สร้างไฟล์เสียงจาก text
	GenerateAudio(ctx context.Context, text string, voiceID string) (*TTSResult, error)
}

// TTSResult - ผลลัพธ์จาก TTS
type TTSResult struct {
	AudioData []byte // MP3 data
	Duration  int    // seconds
	CharCount int    // characters used (for logging)
}

// ExtractTTSScript สกัดใจความสำคัญจาก summary + highlights
// เหลือ ~500 ตัวอักษร เพื่อความกระชับ
func ExtractTTSScript(summary string, highlights []string) string {
	// ใช้ 2 ประโยคแรกจาก summary
	script := extractFirstSentences(summary, 2)

	// เพิ่ม highlights 3 อันแรก
	for i, h := range highlights {
		if i >= 3 {
			break
		}
		script += " " + h
	}

	// ตัดให้ไม่เกิน 500 ตัวอักษร
	if len([]rune(script)) > 500 {
		runes := []rune(script)
		script = string(runes[:500])
	}

	return script
}

func extractFirstSentences(text string, count int) string {
	runes := []rune(text)
	sentenceEnd := []rune{'.', '!', '?', '。'}
	sentences := 0
	endIdx := len(runes)

	for i, r := range runes {
		for _, end := range sentenceEnd {
			if r == end {
				sentences++
				if sentences >= count {
					endIdx = i + 1
					break
				}
			}
		}
		if sentences >= count {
			break
		}
	}

	return string(runes[:endIdx])
}
