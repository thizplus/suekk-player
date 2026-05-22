package ports

import (
	"context"

	"github.com/suekk/my_worker/go/transcode/domain"
)

// JobPublisherPort defines interface for publishing downstream jobs
type JobPublisherPort interface {
	// PublishGalleryJob publishes a job to gallery worker
	PublishGalleryJob(ctx context.Context, meta *domain.JobMeta, input *domain.GalleryJobInput) error

	// PublishWarmCacheJob publishes a job to warmcache worker
	PublishWarmCacheJob(ctx context.Context, meta *domain.JobMeta, input *domain.WarmCacheJobInput) error

	// PublishSubtitleDetectJob publishes a job to subtitle_detect worker
	PublishSubtitleDetectJob(ctx context.Context, meta *domain.JobMeta, input *domain.SubtitleDetectJobInput) error
}
