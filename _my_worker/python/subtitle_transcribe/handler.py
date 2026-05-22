"""
Handler for subtitle transcription jobs.

Full 14-stage pipeline:
1. Download audio
2. Demucs voice separation (optional)
3. Speaker diarization (optional - not implemented yet)
4. Auto-detect language (skip if provided)
5. Whisper transcribe
6. Cap subtitle duration
7. VAD gap detection
8. Re-transcribe gaps (kotoba for ja, large-v3 for others)
9. Merge gaps
10. Smart fix
11. LLM refine (optional)
12. Generate SRT/VTT
13. Upload to storage
14. Complete
"""

import asyncio
import json
import logging
import shutil
import time
from pathlib import Path
from typing import List, Optional, Tuple, Callable

from shared.config import Config
from shared.nats_consumer import Job, JobMeta
from shared.progress import ProgressPublisher
from shared.storage import S3Client
from shared.adapters import WhisperAdapter, SileroVADAdapter, AudioAdapter, GeminiAdapter
from shared.adapters import SubtitleLine, LanguageCode

logger = logging.getLogger(__name__)

# =============================================================================
# Lazy Loaded Adapters (Singleton)
# =============================================================================

_whisper_turbo = None
_whisper_gap_ja = None
_whisper_gap_other = None
_vad_adapter = None
_audio_adapter = None
_gemini_adapter = None


def get_whisper_turbo(device: str = "cuda") -> WhisperAdapter:
    """Main transcription model (turbo - fast + quality)"""
    global _whisper_turbo
    if _whisper_turbo is None:
        logger.info("Loading Whisper turbo model")
        _whisper_turbo = WhisperAdapter(model="turbo", device=device)
    return _whisper_turbo


def get_whisper_gap_ja(device: str = "cuda") -> WhisperAdapter:
    """Japanese gap re-transcription (kotoba - less hallucination)"""
    global _whisper_gap_ja
    if _whisper_gap_ja is None:
        logger.info("Loading Whisper kotoba model for Japanese gaps")
        _whisper_gap_ja = WhisperAdapter(model="kotoba", device=device)
    return _whisper_gap_ja


def get_whisper_gap_other(device: str = "cuda") -> WhisperAdapter:
    """Non-Japanese gap re-transcription (large-v3)"""
    global _whisper_gap_other
    if _whisper_gap_other is None:
        logger.info("Loading Whisper large-v3 model for other language gaps")
        _whisper_gap_other = WhisperAdapter(model="large-v3", device=device)
    return _whisper_gap_other


def get_vad_adapter() -> SileroVADAdapter:
    """Voice Activity Detection"""
    global _vad_adapter
    if _vad_adapter is None:
        logger.info("Loading Silero VAD model")
        _vad_adapter = SileroVADAdapter()
    return _vad_adapter


def get_audio_adapter(temp_dir: Path) -> AudioAdapter:
    """Audio processing (Demucs, FFmpeg)"""
    global _audio_adapter
    if _audio_adapter is None:
        logger.info("Loading AudioAdapter")
        _audio_adapter = AudioAdapter(temp_dir=temp_dir)
    return _audio_adapter


def get_gemini_adapter() -> GeminiAdapter:
    """LLM for refinement"""
    global _gemini_adapter
    if _gemini_adapter is None:
        logger.info("Loading GeminiAdapter")
        _gemini_adapter = GeminiAdapter()
    return _gemini_adapter


# =============================================================================
# Stages
# =============================================================================

STAGE_INITIALIZING = "initializing"
STAGE_DOWNLOADING = "downloading"
STAGE_DEMUCS = "demucs"
STAGE_TRANSCRIBING = "transcribing"
STAGE_PROCESSING = "processing"
STAGE_VAD = "vad"
STAGE_GAPS = "gaps"
STAGE_MERGING = "merging"
STAGE_FIXING = "fixing"
STAGE_REFINING = "refining"
STAGE_GENERATING = "generating"
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

class SubtitleTranscribeHandler:
    """
    Handles subtitle transcription jobs.

    Full 14-stage pipeline matching the original _subtitle/use_cases/transcribe_flow.py
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

        # Feature flags (จาก config หรือ hardcode)
        self.use_demucs = getattr(config, 'use_demucs', True)
        self.use_vad = getattr(config, 'use_vad', True)
        self.use_refine = getattr(config, 'use_refine', True)
        self.max_subtitle_duration = getattr(config, 'max_subtitle_duration', 7.0)

    async def handle(self, job: Job) -> dict:
        """
        Process a subtitle transcription job.

        Input:
            audio_path: S3 path to audio.wav
            language: Detected language code
            output_path: S3 path prefix for subtitles

        Output:
            srt_path: S3 path to generated SRT file
            vtt_path: S3 path to generated VTT file
            segments: Number of subtitle segments
            duration: Audio duration in seconds
        """
        start_time = time.time()
        meta = job.meta
        input_data = job.input

        audio_path = input_data.get("audio_path", "")
        language = input_data.get("language", "ja")
        output_path = input_data.get("output_path", "").rstrip("/")

        logger.info(f"Starting transcription: {meta.job_id}")
        logger.info(f"  Audio: {audio_path}")
        logger.info(f"  Language: {language}")
        logger.info(f"  Output: {output_path}")

        # Create temp directory
        job_dir = Path(self.config.temp_dir) / meta.job_id
        job_dir.mkdir(parents=True, exist_ok=True)

        # Track temp files for cleanup
        temp_files = []
        temp_dirs = []

        try:
            # =================================================================
            # Stage 1: Initialize
            # =================================================================
            await self._publish_progress(meta, STAGE_INITIALIZING, 0, "เริ่มถอดเสียง")

            # =================================================================
            # Stage 2: Download audio (5%)
            # =================================================================
            await self._publish_progress(meta, STAGE_DOWNLOADING, 5, "กำลังดาวน์โหลดเสียง...")

            local_audio = job_dir / "audio.wav"
            self.storage.download(audio_path, str(local_audio))
            temp_files.append(local_audio)

            logger.info(f"Audio downloaded: {local_audio}")

            # =================================================================
            # Stage 3: Demucs voice separation (10%)
            # =================================================================
            audio_for_whisper = local_audio

            if self.use_demucs:
                await self._publish_progress(meta, STAGE_DEMUCS, 10, "กำลังแยกเสียงพูด (Demucs)")

                local_audio_clean = job_dir / "audio_clean.wav"
                temp_files.append(local_audio_clean)

                try:
                    audio_adapter = get_audio_adapter(job_dir)
                    success = audio_adapter.separate_vocals(local_audio, local_audio_clean)

                    if success and local_audio_clean.exists():
                        audio_for_whisper = local_audio_clean
                        logger.info("Demucs completed, using clean audio")
                    else:
                        logger.warning("Demucs failed, using original audio")
                except Exception as e:
                    logger.warning(f"Demucs error: {e}, using original audio")

            # =================================================================
            # Stage 4: Whisper transcribe (20-50%)
            # =================================================================
            whisper = get_whisper_turbo(self.config.whisper_device)

            # Auto-detect language if "auto" (กรณี API ส่งมาโดยไม่ผ่าน detect worker)
            if language == "auto":
                await self._publish_progress(meta, STAGE_TRANSCRIBING, 15, "กำลังตรวจจับภาษา...")
                detected_lang, confidence = whisper.detect_language(str(audio_for_whisper))
                language = detected_lang
                logger.info(f"Auto-detected language: {language} ({confidence:.2f})")
                # อัพเดท output path ให้ตรงกับภาษาจริง
                if output_path.endswith("/auto.srt"):
                    output_path = output_path.replace("/auto.srt", f"/{language}.srt")

            await self._publish_progress(meta, STAGE_TRANSCRIBING, 20, f"กำลังถอดเสียง ({language})...")

            lang_code = LanguageCode(language)

            segments = whisper.transcribe(audio_for_whisper, lang_code)
            logger.info(f"Whisper completed: {len(segments)} segments")

            await self._publish_progress(meta, STAGE_TRANSCRIBING, 50, f"ถอดเสียงได้ {len(segments)} บรรทัด")

            # =================================================================
            # Stage 5: Cap subtitle duration (55%)
            # =================================================================
            await self._publish_progress(meta, STAGE_PROCESSING, 55, "กำลังตัดซับไตเติ้ลยาว")

            segments = self._cap_subtitle_duration(segments, self.max_subtitle_duration)
            logger.info(f"Cap duration: {len(segments)} segments")

            # =================================================================
            # Stage 6-8: VAD gap detection and re-transcription (60-75%)
            # =================================================================
            if self.use_vad:
                segments = await self._process_vad_gaps(
                    segments, audio_for_whisper, lang_code, job_dir, meta
                )
                temp_dirs.append(job_dir / "gaps")

            # =================================================================
            # Stage 9: Smart fix (78%)
            # =================================================================
            await self._publish_progress(meta, STAGE_FIXING, 78, "กำลังแก้ไขซับไตเติ้ล")
            segments = self._smart_fix(segments)

            # =================================================================
            # Stage 10: LLM refine (82%)
            # =================================================================
            if self.use_refine:
                await self._publish_progress(meta, STAGE_REFINING, 82, "กำลังปรับปรุงข้อความ (LLM)")

                try:
                    gemini = get_gemini_adapter()
                    segments = gemini.refine_batch(segments, lang_code, context="Video subtitle")
                    logger.info(f"Refinement completed: {len(segments)} segments")
                except Exception as e:
                    logger.warning(f"Refinement failed: {e}, using original text")

            # =================================================================
            # Stage 11: Generate SRT/VTT (88%)
            # =================================================================
            await self._publish_progress(meta, STAGE_GENERATING, 88, "กำลังสร้างไฟล์ SRT/VTT")

            # Fix overlaps
            segments = fix_segment_overlaps(segments)

            # Re-index segments
            for i, seg in enumerate(segments):
                segments[i] = SubtitleLine(
                    index=i,
                    text=seg.text,
                    start_ms=seg.start_ms,
                    end_ms=seg.end_ms,
                    speaker_id=seg.speaker_id,
                    gender=seg.gender,
                    confidence=seg.confidence,
                    metadata=seg.metadata,
                )

            # Generate content
            srt_content = generate_srt(segments)
            vtt_content = generate_vtt(segments)

            # Save locally
            local_srt = job_dir / f"{language}.srt"
            local_vtt = job_dir / f"{language}.vtt"

            local_srt.write_text(srt_content, encoding="utf-8")
            local_vtt.write_text(vtt_content, encoding="utf-8")

            temp_files.extend([local_srt, local_vtt])

            logger.info(f"Generated SRT/VTT: {len(segments)} subtitles")

            # =================================================================
            # Stage 12: Upload (95%)
            # =================================================================
            await self._publish_progress(meta, STAGE_UPLOADING, 95, "กำลังอัพโหลด...")

            # output_path อาจเป็น "subtitles/code/ja.srt" (full path) หรือ "subtitles/code" (directory)
            # ตัด filename ออกถ้ามี .srt/.vtt ต่อท้าย
            base_path = output_path
            if base_path.endswith(".srt") or base_path.endswith(".vtt"):
                base_path = base_path.rsplit("/", 1)[0]  # ใช้ string split ไม่ใช่ Path (Windows ใส่ \ ให้)
            remote_srt = f"{base_path}/{language}.srt"
            remote_vtt = f"{base_path}/{language}.vtt"

            self.storage.upload(remote_srt, str(local_srt))
            self.storage.upload(remote_vtt, str(local_vtt))

            logger.info(f"Uploaded SRT: {remote_srt}")
            logger.info(f"Uploaded VTT: {remote_vtt}")

            # =================================================================
            # Stage 13: Complete
            # =================================================================
            duration_sec = time.time() - start_time
            audio_duration = self._get_audio_duration(local_audio)

            output = {
                "srt_path": remote_srt,
                "vtt_path": remote_vtt,
                "segments": len(segments),
                "duration": audio_duration,
                "language": language,
            }

            await self.progress.publish_completed(
                job_id=meta.job_id,
                job_type=meta.job_type,
                entity_type=meta.entity_type,
                entity_id=meta.entity_id,
                entity_code=meta.entity_code,
                duration_sec=duration_sec,
                output=output,
            )

            logger.info(f"Transcription completed in {duration_sec:.1f}s")
            return output

        except Exception as e:
            logger.error(f"Transcription failed: {e}")

            await self.progress.publish_failed(
                job_id=meta.job_id,
                job_type=meta.job_type,
                entity_type=meta.entity_type,
                entity_id=meta.entity_id,
                entity_code=meta.entity_code,
                error=str(e),
                stage=STAGE_FAILED,
                error_code="SUBTITLE_TRANSCRIBE_FAILED",
            )

            raise

        finally:
            self._cleanup(job_dir, temp_files, temp_dirs)

    # =========================================================================
    # VAD Gap Processing
    # =========================================================================

    async def _process_vad_gaps(
        self,
        segments: List[SubtitleLine],
        audio_path: Path,
        language: LanguageCode,
        job_dir: Path,
        meta: JobMeta,
    ) -> List[SubtitleLine]:
        """Process VAD gap detection and re-transcription"""
        try:
            # Stage 6: VAD detection
            await self._publish_progress(meta, STAGE_VAD, 60, "กำลังตรวจหาเสียงที่หายไป (VAD)")

            vad = get_vad_adapter()
            vad_segments = vad.detect_voice(audio_path)

            # Find gaps (speech in VAD but not in subtitles)
            gaps = vad.find_gaps(vad_segments, segments)
            logger.info(f"Found {len(gaps)} gaps")

            if not gaps:
                return segments

            # Stage 7: Cut and re-transcribe gaps
            await self._publish_progress(meta, STAGE_GAPS, 65, f"กำลังประมวลผล {len(gaps)} ช่องว่าง")

            gaps_dir = job_dir / "gaps"
            gaps_dir.mkdir(parents=True, exist_ok=True)

            audio_adapter = get_audio_adapter(job_dir)

            for i, gap in enumerate(gaps):
                gap_audio = gaps_dir / f"gap_{i}.wav"
                success = audio_adapter.cut_segment(
                    audio_path, gap_audio,
                    gap['start'], gap['end'],
                )
                if success:
                    gap['audio_path'] = gap_audio

            await self._publish_progress(meta, STAGE_GAPS, 70, "กำลังถอดเสียงช่องว่าง")

            # Select gap model based on language
            # kotoba for Japanese (less hallucination), large-v3 for others
            if language.code == "ja":
                gap_whisper = get_whisper_gap_ja(self.config.whisper_device)
            else:
                gap_whisper = get_whisper_gap_other(self.config.whisper_device)

            logger.info(f"Gap re-transcription using: {gap_whisper.get_model_name()}")

            for gap in gaps:
                if 'audio_path' in gap and gap['audio_path'].exists():
                    try:
                        text, is_valid = gap_whisper.transcribe_segment(
                            gap['audio_path'],
                            language,
                        )
                        if is_valid and text:
                            gap['text'] = text
                    except Exception as e:
                        logger.warning(f"Gap re-transcription failed: {e}")

            speech_gaps = len([g for g in gaps if g.get('text')])
            logger.info(f"Re-transcribed gaps: {speech_gaps} with speech")

            # Stage 8: Merge gaps
            await self._publish_progress(meta, STAGE_MERGING, 75, "กำลังรวมซับไตเติ้ล")
            segments = self._merge_gaps(segments, gaps)
            logger.info(f"Merged: {len(segments)} total segments")

            return segments

        except Exception as e:
            logger.warning(f"VAD/Gap processing failed: {e}")
            return segments

    # =========================================================================
    # Helper Methods
    # =========================================================================

    def _cap_subtitle_duration(
        self,
        segments: List[SubtitleLine],
        max_duration: float = 7.0
    ) -> List[SubtitleLine]:
        """Cap subtitle duration to max_duration seconds"""
        result = []
        for seg in segments:
            duration = seg.duration_sec
            if duration > max_duration:
                new_end_ms = int((seg.start_sec + max_duration) * 1000)
                result.append(seg.with_timing(seg.start_ms, new_end_ms))
            else:
                result.append(seg)
        return result

    def _merge_gaps(
        self,
        segments: List[SubtitleLine],
        gaps: List[dict]
    ) -> List[SubtitleLine]:
        """Merge gap transcriptions into segments"""
        result = list(segments)

        for gap in gaps:
            if gap.get('text'):
                # Create new segment for gap
                new_seg = SubtitleLine(
                    index=len(result),
                    text=gap['text'],
                    start_ms=int(gap['start'] * 1000),
                    end_ms=int(gap['end'] * 1000),
                )
                result.append(new_seg)

        # Sort by start time
        result.sort(key=lambda s: s.start_sec)
        return result

    def _smart_fix(self, segments: List[SubtitleLine]) -> List[SubtitleLine]:
        """Apply smart fixes to segments - filter empty/noise"""
        result = []
        for seg in segments:
            text = seg.text.strip() if seg.text else ""
            # Filter out empty or very short segments
            if text and len(text) > 1:
                result.append(seg)
        return result

    def _get_audio_duration(self, audio_path: Path) -> float:
        """Get audio duration in seconds"""
        try:
            audio_adapter = get_audio_adapter(audio_path.parent)
            duration = audio_adapter.get_duration(audio_path)
            return duration or 0.0
        except Exception:
            return 0.0

    async def _publish_progress(
        self,
        meta: JobMeta,
        stage: str,
        progress: float,
        message: str,
    ):
        """Publish progress update"""
        await self.progress.publish_processing(
            job_id=meta.job_id,
            job_type=meta.job_type,
            entity_type=meta.entity_type,
            entity_id=meta.entity_id,
            entity_code=meta.entity_code,
            stage=stage,
            progress=progress,
            message=message,
        )

    def _cleanup(self, job_dir: Path, temp_files: List[Path], temp_dirs: List[Path]):
        """Remove temporary files and directories"""
        for path in temp_files:
            try:
                if path and path.exists():
                    path.unlink()
            except Exception as e:
                logger.warning(f"Cleanup file failed: {e}")

        for path in temp_dirs:
            try:
                if path and path.exists():
                    shutil.rmtree(path)
            except Exception as e:
                logger.warning(f"Cleanup dir failed: {e}")

        try:
            if job_dir.exists():
                shutil.rmtree(job_dir)
        except Exception as e:
            logger.warning(f"Cleanup job dir failed: {e}")
