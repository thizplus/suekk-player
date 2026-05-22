package models

// ═══════════════════════════════════════════════════════════════════════════════
// Worker Job Types & Status
// ═══════════════════════════════════════════════════════════════════════════════

// WorkerJobType represents the type of worker job
type WorkerJobType string

const (
	// Video Processing
	WorkerJobTypeTranscode WorkerJobType = "transcode"
	WorkerJobTypeGallery   WorkerJobType = "gallery"
	WorkerJobTypeWarmCache WorkerJobType = "warmcache"

	// Subtitle Processing (3 sub-types)
	WorkerJobTypeSubtitleDetect    WorkerJobType = "subtitle_detect"
	WorkerJobTypeSubtitleTranscribe WorkerJobType = "subtitle_transcribe"
	WorkerJobTypeSubtitleTranslate  WorkerJobType = "subtitle_translate"

	// Reel Export
	WorkerJobTypeReel WorkerJobType = "reel"
)

// WorkerJobStatus represents the status of a worker job
type WorkerJobStatus string

const (
	WorkerJobStatusPending    WorkerJobStatus = "pending"     // รอ publish
	WorkerJobStatusQueued     WorkerJobStatus = "queued"      // อยู่ใน NATS queue
	WorkerJobStatusProcessing WorkerJobStatus = "processing"  // worker กำลังทำ
	WorkerJobStatusCompleted  WorkerJobStatus = "completed"   // สำเร็จ
	WorkerJobStatusFailed     WorkerJobStatus = "failed"      // ล้มเหลว (หมด retry)
	WorkerJobStatusCancelled  WorkerJobStatus = "cancelled"   // ยกเลิก
	WorkerJobStatusDeadLetter WorkerJobStatus = "dead_letter" // poison pill
)

// EntityType represents the type of entity being processed
type WorkerEntityType string

const (
	WorkerEntityTypeVideo    WorkerEntityType = "video"
	WorkerEntityTypeSubtitle WorkerEntityType = "subtitle"
	WorkerEntityTypeReel     WorkerEntityType = "reel"
)

// IsValidJobType checks if the job type is valid
func (t WorkerJobType) IsValid() bool {
	switch t {
	case WorkerJobTypeTranscode, WorkerJobTypeGallery, WorkerJobTypeWarmCache,
		WorkerJobTypeSubtitleDetect, WorkerJobTypeSubtitleTranscribe, WorkerJobTypeSubtitleTranslate,
		WorkerJobTypeReel:
		return true
	}
	return false
}

// IsValidStatus checks if the status is valid
func (s WorkerJobStatus) IsValid() bool {
	switch s {
	case WorkerJobStatusPending, WorkerJobStatusQueued, WorkerJobStatusProcessing,
		WorkerJobStatusCompleted, WorkerJobStatusFailed, WorkerJobStatusCancelled,
		WorkerJobStatusDeadLetter:
		return true
	}
	return false
}

// IsTerminal returns true if status is terminal (no more transitions)
func (s WorkerJobStatus) IsTerminal() bool {
	switch s {
	case WorkerJobStatusCompleted, WorkerJobStatusFailed, WorkerJobStatusCancelled, WorkerJobStatusDeadLetter:
		return true
	}
	return false
}
