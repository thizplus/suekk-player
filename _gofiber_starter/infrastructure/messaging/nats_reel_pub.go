package messaging

import (
	"context"

	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/domain/services"
)

// NATSReelPublisher implements services.ReelJobPublisher using NATS JetStream
type NATSReelPublisher struct {
	publisher *natspkg.Publisher
}

// NewNATSReelPublisher สร้าง ReelJobPublisher adapter สำหรับ NATS
func NewNATSReelPublisher(publisher *natspkg.Publisher) services.ReelJobPublisher {
	return &NATSReelPublisher{
		publisher: publisher,
	}
}

// PublishReelExportJob ส่ง reel export job เข้า queue
func (p *NATSReelPublisher) PublishReelExportJob(ctx context.Context, job *services.ReelExportJob) error {
	// Convert from service type to NATS type
	natsJob := &natspkg.ReelExportJob{
		ReelID:       job.ReelID,
		VideoID:      job.VideoID,
		VideoCode:    job.VideoCode,
		HLSPath:      job.HLSPath,
		VideoQuality: job.VideoQuality,

		// Multi-segment support
		Segments: convertSegmentsToNATSFormat(job.Segments),

		// LEGACY: Single segment
		SegmentStart: job.SegmentStart,
		SegmentEnd:   job.SegmentEnd,
		CoverTime:    job.CoverTime,

		// Style-based fields
		Style:        job.Style,
		Title:        job.Title,
		Line1:        job.Line1,
		Line2:        job.Line2,
		ShowLogo:     job.ShowLogo,
		LogoPath:     job.LogoPath,
		GradientPath: job.GradientPath,
		CropX:        job.CropX,
		CropY:        job.CropY,

		// TTS
		TTSText: job.TTSText,

		// LEGACY: Layer-based fields
		OutputFormat: job.OutputFormat,
		VideoFit:     job.VideoFit,
		Layers:       convertLayersToNATSFormat(job.Layers),

		OutputPath: job.OutputPath,
	}

	return p.publisher.PublishReelExportJob(ctx, natsJob)
}

// convertSegmentsToNATSFormat แปลง segments จาก service type เป็น NATS type
func convertSegmentsToNATSFormat(segments []services.VideoSegmentJob) []natspkg.VideoSegmentJob {
	result := make([]natspkg.VideoSegmentJob, len(segments))
	for i, s := range segments {
		result[i] = natspkg.VideoSegmentJob{
			Start: s.Start,
			End:   s.End,
		}
	}
	return result
}

// convertLayersToNATSFormat แปลง layers จาก service type เป็น NATS type
func convertLayersToNATSFormat(layers []services.ReelLayerJob) []natspkg.ReelLayerJob {
	result := make([]natspkg.ReelLayerJob, len(layers))
	for i, l := range layers {
		result[i] = natspkg.ReelLayerJob{
			Type:       l.Type,
			Content:    l.Content,
			FontFamily: l.FontFamily,
			FontSize:   l.FontSize,
			FontColor:  l.FontColor,
			FontWeight: l.FontWeight,
			X:          l.X,
			Y:          l.Y,
			Width:      l.Width,
			Height:     l.Height,
			Opacity:    l.Opacity,
			ZIndex:     l.ZIndex,
			Style:      l.Style,
		}
	}
	return result
}
