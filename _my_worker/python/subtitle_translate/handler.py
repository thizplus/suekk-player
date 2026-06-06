"""
Handler for subtitle translation jobs.

Translates subtitles from source language to target languages.
Supports cluster-based translation with dynamic summary for better context.
"""

import json
import logging
import re
import shutil
import time
from pathlib import Path
from typing import List, Dict, Optional

from shared.config import Config
from shared.nats_consumer import Job, JobMeta
from shared.progress import ProgressPublisher
from shared.storage import S3Client
from shared.adapters import SubtitleLine, LanguageCode
from shared.adapters.llm_factory import create_llm
from shared.ports.llm_port import LLMPort

logger = logging.getLogger(__name__)

def get_llm() -> LLMPort:
    """Get LLM instance via Port/Adapter pattern — สร้างใหม่ทุกครั้งเพื่อโหลด key ล่าสุดจาก env"""
    import os
    provider = os.getenv("SUBTITLE_LLM_PROVIDER")
    llm = create_llm(provider)
    logger.info(f"LLM loaded: {llm.get_provider_name()} / {llm.get_model_name()}")
    return llm


# =============================================================================
# Stages
# =============================================================================

STAGE_INITIALIZING = "initializing"
STAGE_DOWNLOADING = "downloading"
STAGE_TRANSLATING = "translating"
STAGE_UPLOADING = "uploading"
STAGE_COMPLETED = "completed"
STAGE_FAILED = "failed"


# =============================================================================
# Utility Functions
# =============================================================================

def format_srt_timestamp(seconds: float) -> str:
    """Format seconds to SRT timestamp (HH:MM:SS,mmm)"""
    hours = int(seconds // 3600)
    minutes = int((seconds % 3600) // 60)
    secs = int(seconds % 60)
    millis = int((seconds % 1) * 1000)
    return f"{hours:02d}:{minutes:02d}:{secs:02d},{millis:03d}"


def format_vtt_timestamp(seconds: float) -> str:
    """Format seconds to VTT timestamp (HH:MM:SS.mmm)"""
    hours = int(seconds // 3600)
    minutes = int((seconds % 3600) // 60)
    secs = int(seconds % 60)
    millis = int((seconds % 1) * 1000)
    return f"{hours:02d}:{minutes:02d}:{secs:02d}.{millis:03d}"


def generate_srt(segments: List[SubtitleLine]) -> str:
    """Generate SRT content from segments"""
    lines = []
    for i, seg in enumerate(segments, 1):
        start = format_srt_timestamp(seg.start_sec)
        end = format_srt_timestamp(seg.end_sec)
        text = seg.text.strip() if seg.text else ""

        lines.append(str(i))
        lines.append(f"{start} --> {end}")
        lines.append(text)
        lines.append("")

    return "\n".join(lines)


def generate_vtt(segments: List[SubtitleLine]) -> str:
    """Generate VTT content from segments"""
    lines = ["WEBVTT", ""]
    for seg in segments:
        start = format_vtt_timestamp(seg.start_sec)
        end = format_vtt_timestamp(seg.end_sec)
        text = seg.text.strip() if seg.text else ""

        lines.append(f"{start} --> {end}")
        lines.append(text)
        lines.append("")

    return "\n".join(lines)


def fix_segment_overlaps(segments: List[SubtitleLine], min_gap_sec: float = 0.001) -> List[SubtitleLine]:
    """Fix overlapping timecodes in segments"""
    if not segments:
        return segments

    sorted_segs = sorted(segments, key=lambda s: s.start_sec)
    result = []
    fixed_count = 0

    for i in range(len(sorted_segs)):
        current = sorted_segs[i]

        if i == len(sorted_segs) - 1:
            result.append(current)
            continue

        next_seg = sorted_segs[i + 1]

        if current.end_sec >= next_seg.start_sec:
            new_end = next_seg.start_sec - min_gap_sec
            if new_end <= current.start_sec:
                new_end = current.start_sec + 0.05

            new_end_ms = int(new_end * 1000)
            current = current.with_timing(current.start_ms, new_end_ms)
            fixed_count += 1

        result.append(current)

    if fixed_count > 0:
        logger.info(f"Fixed {fixed_count} overlapping timecodes")

    return result


# =============================================================================
# Handler
# =============================================================================

class SubtitleTranslateHandler:
    """
    Handles subtitle translation jobs.

    Uses cluster-based translation with dynamic summary for better context.
    """

    def __init__(
        self,
        config: Config,
        storage: S3Client,
        progress: ProgressPublisher,
    ):
        self.config = config
        self.storage = storage
        self.progress = progress

        # Cluster settings
        self.cluster_size = getattr(config, 'cluster_size', 30)
        self.cluster_gap_sec = getattr(config, 'cluster_gap_sec', 3.0)

    async def handle(self, job: Job) -> dict:
        """
        Process a subtitle translation job.

        Input:
            srt_path: S3 path to source SRT file
            source_language: Source language code
            target_languages: List of target language codes
            output_path: S3 path prefix for output
            speaker_info_path: Optional S3 path to speakers.json

        Output:
            translations: Dict of {language: srt_path}
            vtt_paths: Dict of {language: vtt_path}
        """
        start_time = time.time()
        meta = job.meta
        input_data = job.input

        srt_path = input_data.get("source_srt_path", "") or input_data.get("srt_path", "")
        source_lang = input_data.get("source_language", "ja")
        target_langs = input_data.get("target_languages", [])
        output_path = input_data.get("output_path", "").rstrip("/")
        speaker_info_path = input_data.get("speaker_info_path", "")
        context = input_data.get("context", "Video subtitle")
        # Store video_id and subtitle_ids for progress routing
        self._video_id = input_data.get("video_id", "")
        self._video_code = input_data.get("video_code", "")
        self._subtitle_ids = input_data.get("subtitle_ids", [])

        # Validate source language
        valid_languages = ["ja", "en", "zh", "ko", "th"]
        if source_lang not in valid_languages:
            logger.warning(f"Invalid source_language '{source_lang}', using 'ja'")
            source_lang = "ja"

        logger.info(f"Starting translation: {meta.job_id}")
        logger.info(f"  Source: {srt_path} ({source_lang})")
        logger.info(f"  Targets: {target_langs}")

        job_dir = Path(self.config.temp_dir) / meta.job_id
        job_dir.mkdir(parents=True, exist_ok=True)

        temp_files = []

        try:
            # =================================================================
            # Stage 1: Initialize
            # =================================================================
            await self._publish_progress(meta, STAGE_INITIALIZING, 0, "เริ่มแปลซับไตเติ้ล")

            # =================================================================
            # Stage 2: Download source SRT (5%)
            # =================================================================
            await self._publish_progress(meta, STAGE_DOWNLOADING, 5, "กำลังดาวน์โหลดซับไตเติ้ลต้นฉบับ")

            local_srt = job_dir / "source.srt"
            self.storage.download(srt_path, str(local_srt))
            temp_files.append(local_srt)

            source_content = local_srt.read_text(encoding="utf-8")
            segments = self._parse_srt(source_content)

            logger.info(f"Loaded {len(segments)} segments")

            # =================================================================
            # Stage 3: Load speaker info (optional)
            # =================================================================
            speaker_info = None
            if speaker_info_path:
                try:
                    local_speaker = job_dir / "speakers.json"
                    self.storage.download(speaker_info_path, str(local_speaker))
                    temp_files.append(local_speaker)

                    speaker_data = json.loads(local_speaker.read_text(encoding="utf-8"))
                    speaker_info = speaker_data
                    logger.info(f"Loaded speaker info: {len(speaker_data.get('speakers', {}))} speakers")

                    # Tag segments with speaker info
                    segments = self._tag_segments_with_speakers(segments, speaker_info)
                except Exception as e:
                    logger.warning(f"Could not load speaker info: {e}")

            # =================================================================
            # Stage 4: Translate to each target language
            # =================================================================
            translations = {}
            vtt_paths = {}
            total_targets = len(target_langs)

            llm = get_llm()

            for i, target_lang in enumerate(target_langs):
                progress_base = 10 + int((i / total_targets) * 70)
                target = LanguageCode(target_lang)

                await self._publish_progress(
                    meta, STAGE_TRANSLATING, progress_base,
                    f"กำลังแปลเป็นภาษา{target.name}"
                )

                logger.info(f"Translating to {target_lang}")

                # Cluster-based translation for better context
                translated = await self._translate_with_clusters(
                    segments=segments,
                    source_lang=LanguageCode(source_lang),
                    target_lang=target,
                    context=context,
                    llm=llm,
                )

                logger.info(f"Translated {len(translated)} segments to {target_lang}")

                # Fix overlaps
                translated = fix_segment_overlaps(translated)

                # Generate SRT/VTT
                srt_content = generate_srt(translated)
                vtt_content = generate_vtt(translated)

                # Save locally
                local_translated_srt = job_dir / f"{target_lang}.srt"
                local_translated_vtt = job_dir / f"{target_lang}.vtt"

                local_translated_srt.write_text(srt_content, encoding="utf-8")
                local_translated_vtt.write_text(vtt_content, encoding="utf-8")

                temp_files.extend([local_translated_srt, local_translated_vtt])

                # Upload
                await self._publish_progress(
                    meta, STAGE_UPLOADING, progress_base + 10,
                    f"กำลังอัพโหลดซับไตเติ้ลภาษา{target.name}"
                )

                remote_srt = f"{output_path}/{target_lang}.srt"
                remote_vtt = f"{output_path}/{target_lang}.vtt"

                self.storage.upload(remote_srt, str(local_translated_srt))
                self.storage.upload(remote_vtt, str(local_translated_vtt))

                translations[target_lang] = remote_srt
                vtt_paths[target_lang] = remote_vtt

                logger.info(f"Uploaded: {remote_srt}")

            # =================================================================
            # Stage 5: Complete
            # =================================================================
            duration_sec = time.time() - start_time
            output = {
                "translations": translations,
                "vtt_paths": vtt_paths,
                "source_language": source_lang,
                "segments_count": len(segments),
                "video_id": self._video_id,
            }

            # Send per-language completion with subtitle_id for each target
            # ต้อง override entity_type/entity_id เป็น subtitle เพื่อให้ API route ถูก
            from shared.progress import ProgressUpdate
            for i, target_lang in enumerate(translations.keys()):
                sub_id = self._subtitle_ids[i] if i < len(self._subtitle_ids) else ""
                srt_path = translations.get(target_lang, "")
                completed_update = ProgressUpdate(
                    job_id=meta.job_id,
                    job_type=meta.job_type,
                    entity_type="subtitle",
                    entity_id=sub_id or meta.entity_id,
                    entity_code=meta.entity_code,
                    worker_id=self.progress.worker_id,
                    status="completed",
                    progress=100.0,
                    stage="completed",
                    message="สำเร็จ",
                    duration_sec=duration_sec,
                    output={
                        "translations": translations,
                        "srt_path": srt_path,
                        "language": target_lang,
                        "video_id": self._video_id,
                    },
                    video_id=self._video_id,
                    video_code=self._video_code,
                    subtitle_id=sub_id,
                    current_language=target_lang,
                )
                self.progress.throttler.cleanup(meta.job_id)
                await self.progress.publish(completed_update)

            logger.info(f"Translation completed in {duration_sec:.1f}s")
            return output

        except Exception as e:
            logger.error(f"Translation failed: {e}")

            await self.progress.publish_failed(
                job_id=meta.job_id,
                job_type=meta.job_type,
                entity_type=meta.entity_type,
                entity_id=meta.entity_id,
                entity_code=meta.entity_code,
                error=str(e),
                stage=STAGE_FAILED,
                error_code="SUBTITLE_TRANSLATE_FAILED",
            )

            raise

        finally:
            self._cleanup(job_dir, temp_files)

    # =========================================================================
    # Translation Methods
    # =========================================================================

    async def _translate_with_clusters(
        self,
        segments: List[SubtitleLine],
        source_lang: LanguageCode,
        target_lang: LanguageCode,
        context: str,
        llm: LLMPort,
    ) -> List[SubtitleLine]:
        """
        Translate using cluster-based approach with dynamic summary.

        Groups nearby segments and translates them together for better context.
        Passes summary to next cluster for continuity.
        Uses LLMPort so any provider (Gemini, OpenAI, etc.) can be used.
        """
        if not segments:
            return []

        from subtitle_translate.prompts import translate_cluster

        # Split into clusters
        clusters = self._split_into_clusters(segments)
        logger.info(f"Split into {len(clusters)} clusters")

        translated = []
        previous_summary = None
        total_clusters = len(clusters)

        for ci, cluster in enumerate(clusters):
            # Check if scene change (large time gap)
            is_scene_change = False
            if translated and cluster:
                last_end = translated[-1].end_sec
                first_start = cluster[0].start_sec
                if first_start - last_end > 10.0:  # 10 sec gap = scene change
                    is_scene_change = True
                    previous_summary = None

            # Translate cluster via Port (any LLM provider)
            logger.info(f"Translating cluster {ci+1}/{total_clusters} ({len(cluster)} lines)")
            cluster_result, new_summary = translate_cluster(
                llm=llm,
                lines=cluster,
                previous_summary=previous_summary,
                source_lang=source_lang,
                target_lang=target_lang,
                context=context,
                is_scene_change=is_scene_change,
            )

            translated.extend(cluster_result)
            previous_summary = new_summary
            logger.info(f"Cluster {ci+1}/{total_clusters} done, translated {len(cluster_result)} lines")

        return translated

    def _split_into_clusters(self, segments: List[SubtitleLine]) -> List[List[SubtitleLine]]:
        """Split segments into clusters based on time gaps, then merge small ones"""
        if not segments:
            return []

        # Step 1: Split by time gap or max size
        raw_clusters = []
        current_cluster = []

        for seg in segments:
            if not current_cluster:
                current_cluster.append(seg)
            else:
                last = current_cluster[-1]
                gap = seg.start_sec - last.end_sec

                if gap > self.cluster_gap_sec or len(current_cluster) >= self.cluster_size:
                    raw_clusters.append(current_cluster)
                    current_cluster = [seg]
                else:
                    current_cluster.append(seg)

        if current_cluster:
            raw_clusters.append(current_cluster)

        # Step 2: Merge small clusters (< min_size) with neighbors
        # ไม่ merge ข้าม scene change (gap > 10s)
        min_cluster_size = 5
        scene_change_gap = 10.0
        merged = []

        for cluster in raw_clusters:
            if not merged:
                merged.append(cluster)
                continue

            prev = merged[-1]
            gap_between = cluster[0].start_sec - prev[-1].end_sec
            combined_size = len(prev) + len(cluster)

            # Merge ถ้า: cluster ก่อนหน้าเล็ก + รวมกันไม่เกิน max + ไม่ใช่ scene change
            if len(prev) < min_cluster_size and combined_size <= self.cluster_size and gap_between < scene_change_gap:
                merged[-1] = prev + cluster
            # Merge ถ้า: cluster ปัจจุบันเล็ก + รวมกันไม่เกิน max + ไม่ใช่ scene change
            elif len(cluster) < min_cluster_size and combined_size <= self.cluster_size and gap_between < scene_change_gap:
                merged[-1] = prev + cluster
            else:
                merged.append(cluster)

        logger.info(f"Clustering: {len(raw_clusters)} raw → {len(merged)} merged (min_size={min_cluster_size})")
        return merged

    # =========================================================================
    # Helper Methods
    # =========================================================================

    def _parse_srt(self, content: str) -> List[SubtitleLine]:
        """Parse SRT content to SubtitleLine list"""
        segments = []
        blocks = re.split(r"\n\n+", content.strip())

        for i, block in enumerate(blocks):
            lines = block.strip().split("\n")
            if len(lines) >= 3:
                timestamp_match = re.match(
                    r"(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})",
                    lines[1],
                )
                if timestamp_match:
                    start = self._parse_timestamp(timestamp_match.group(1))
                    end = self._parse_timestamp(timestamp_match.group(2))
                    text = "\n".join(lines[2:])

                    segments.append(SubtitleLine(
                        index=i,
                        text=text,
                        start_ms=int(start * 1000),
                        end_ms=int(end * 1000),
                    ))

        return segments

    def _parse_timestamp(self, timestamp: str) -> float:
        """Parse SRT timestamp to seconds"""
        match = re.match(r"(\d{2}):(\d{2}):(\d{2}),(\d{3})", timestamp)
        if match:
            hours, minutes, seconds, millis = map(int, match.groups())
            return hours * 3600 + minutes * 60 + seconds + millis / 1000
        return 0.0

    def _tag_segments_with_speakers(
        self,
        segments: List[SubtitleLine],
        speaker_info: dict
    ) -> List[SubtitleLine]:
        """Tag segments with speaker info based on timestamp overlap"""
        if not speaker_info:
            return segments

        speaker_segments = speaker_info.get('segments', [])
        speakers = speaker_info.get('speakers', {})

        result = []
        for seg in segments:
            # Find overlapping speaker segment
            speaker_id = None
            gender = None

            for sp_seg in speaker_segments:
                sp_start = sp_seg.get('start', 0)
                sp_end = sp_seg.get('end', 0)

                # Check overlap
                if sp_start <= seg.end_sec and sp_end >= seg.start_sec:
                    speaker_id = sp_seg.get('speaker', '')
                    # Get gender from speakers dict
                    if speaker_id and speaker_id in speakers:
                        gender = speakers[speaker_id].get('gender', 'unknown')
                    else:
                        gender = sp_seg.get('gender', 'unknown')
                    break

            # Create new segment with speaker info
            result.append(SubtitleLine(
                index=seg.index,
                text=seg.text,
                start_ms=seg.start_ms,
                end_ms=seg.end_ms,
                speaker_id=speaker_id,
                gender=gender,
                confidence=seg.confidence,
                metadata=seg.metadata,
            ))

        return result

    async def _publish_progress(self, meta: JobMeta, stage: str, progress: float, message: str):
        """Publish progress update with video_id for API routing"""
        from shared.progress import ProgressUpdate
        update = ProgressUpdate(
            job_id=meta.job_id,
            job_type=meta.job_type,
            entity_type=meta.entity_type,
            entity_id=meta.entity_id,
            entity_code=meta.entity_code,
            worker_id=self.progress.worker_id,
            status="processing",
            progress=progress,
            stage=stage,
            message=message,
            video_id=getattr(self, '_video_id', None),
            video_code=getattr(self, '_video_code', None),
        )
        if self.progress.throttler.should_send(meta.job_id, progress, "processing"):
            await self.progress.publish(update)

    def _cleanup(self, job_dir: Path, temp_files: List[Path]):
        """Remove temporary files"""
        for path in temp_files:
            try:
                if path and path.exists():
                    path.unlink()
            except Exception as e:
                logger.warning(f"Cleanup file failed: {e}")

        try:
            if job_dir.exists():
                shutil.rmtree(job_dir)
        except Exception as e:
            logger.warning(f"Cleanup job dir failed: {e}")
