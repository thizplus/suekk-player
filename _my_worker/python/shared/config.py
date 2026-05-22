"""
Configuration loader for Python workers.
Loads from environment variables and .env files.
"""

import os
import socket
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional, List, Dict, Any


def _ensure_https(endpoint: str) -> str:
    """Add https:// prefix if missing."""
    if endpoint and not endpoint.startswith("http"):
        return f"https://{endpoint}"
    return endpoint

from dotenv import load_dotenv


# =============================================================================
# Whisper Model Configuration (from _subtitle/config/settings.py)
# =============================================================================

WHISPER_MODELS = {
    "large-v3": {
        "name": "large-v3",
        "compute_type": "int8_float16",
        "backend": "faster-whisper",
    },
    "turbo": {
        "name": "deepdml/faster-whisper-large-v3-turbo-ct2",
        "compute_type": "int8_float16",
        "backend": "faster-whisper",
    },
    "kotoba": {
        "name": "kotoba-tech/kotoba-whisper-v2.0-faster",
        "compute_type": "int8_float16",
        "backend": "faster-whisper",
        "word_timestamps": False,  # Crashes with alignment
    },
    "kotoba22": {
        "name": "Vinxscribe/kotoba-whisper-v2.2-faster",
        "compute_type": "int8_float16",
        "backend": "faster-whisper",
        "word_timestamps": False,
    },
    "anime": {
        "name": "litagin/anime-whisper",
        "compute_type": "float16",
        "backend": "transformers",
        "no_initial_prompt": True,
    },
}

# Hallucination patterns to filter
HALLUCINATION_PATTERNS = [
    r'ご視聴.*ありがとう',
    r'どうもありがとう.*ました',
    r'アダルトビデオ',
    r'音声.*含まれます',
    r'気持ちいい.*すごい.*あぁ.*んん.*イク',
    r'イク.*などの言葉',
    r'よろしくお願い',
    r'チャンネル登録',
    r'(XRXR){2,}',
]

# Supported languages
SUPPORTED_LANGUAGES = ["ja", "en", "zh", "ko", "th", "ru"]


@dataclass
class Config:
    """Configuration for Python subtitle workers"""

    # Worker identity
    worker_id: str = ""
    worker_type: str = ""  # subtitle_detect, subtitle_transcribe, subtitle_translate

    # NATS configuration
    nats_url: str = "nats://localhost:4222"
    stream_name: str = ""
    consumer_name: str = ""
    subject: str = ""

    # S3 configuration
    s3_endpoint: str = ""
    s3_access_key: str = ""
    s3_secret_key: str = ""
    s3_bucket: str = "suekk"
    s3_region: str = "us-east-1"

    # API configuration (for callbacks)
    api_base_url: str = "http://localhost:8080"
    api_internal_key: str = ""

    # Processing
    temp_dir: str = "/tmp/subtitle"
    log_dir: str = "/tmp/subtitle/logs"
    concurrency: int = 1

    # ==========================================================================
    # Whisper settings
    # ==========================================================================
    whisper_model: str = "turbo"  # Main transcription model
    whisper_device: str = "cuda"  # cuda or cpu
    whisper_compute_type: str = "int8_float16"

    # Gap re-transcription models (CRITICAL for quality)
    gap_model: str = "kotoba"  # For Japanese - less hallucination
    gap_model_fallback: str = "large-v3"  # For other languages

    # ==========================================================================
    # Gemini/LLM settings
    # ==========================================================================
    gemini_api_key: str = ""
    gemini_model: str = "gemini-2.0-flash"
    llm_provider: str = "gemini"  # gemini, openai, deepseek

    # Fallback LLM
    deepseek_api_key: str = ""
    openai_api_key: str = ""

    # ==========================================================================
    # Feature flags
    # ==========================================================================
    use_demucs: bool = True
    use_diarization: bool = True
    use_vad: bool = True
    use_refine: bool = True

    # ==========================================================================
    # Pipeline settings
    # ==========================================================================
    max_subtitle_duration: float = 7.0  # Cap subtitle to 7 seconds
    min_gap_duration: float = 0.5  # Minimum gap to consider
    gap_no_speech_threshold: float = 0.5  # Filter no-speech gaps

    # Cluster settings for translation
    cluster_silence_threshold: float = 5.0  # Gap to split clusters
    cluster_max_lines: int = 40  # Max lines per cluster
    cluster_scene_reset_gap: float = 60.0  # Reset summary after this gap

    # Speaker diarization
    speaker_min_speakers: int = 2
    speaker_max_speakers: int = 6

    def __post_init__(self):
        if not self.worker_id:
            hostname = socket.gethostname()
            self.worker_id = f"{self.worker_type}-{hostname}-{os.getpid()}"

    @property
    def whisper_options(self) -> Dict[str, Any]:
        """Default Whisper transcription options"""
        return {
            "vad_filter": True,
            "vad_parameters": {"min_silence_duration_ms": 500},
            "condition_on_previous_text": False,
            "word_timestamps": True,
            "no_speech_threshold": 0.3,
            "temperature": 0,
        }

    @property
    def supported_source_languages(self) -> List[str]:
        return SUPPORTED_LANGUAGES

    def get_translation_targets(self, source_language: str) -> List[str]:
        """Get target languages for translation"""
        if source_language == "th":
            return ["en"]
        return ["th"]


def load_config(worker_type: str) -> Config:
    """
    Load configuration from environment variables.

    Args:
        worker_type: Type of worker (subtitle_detect, subtitle_transcribe, subtitle_translate)

    Returns:
        Config object with all settings
    """
    # Load .env files (shared first, then worker-specific)
    env_paths = [
        Path(__file__).parent.parent.parent / ".env",  # _my_worker/.env
        Path(__file__).parent.parent / ".env",         # _my_worker/python/.env
        Path.cwd() / ".env",                           # Current directory
    ]

    for env_path in env_paths:
        if env_path.exists():
            load_dotenv(env_path)

    # Map worker type to NATS config
    stream_map = {
        "subtitle_detect": ("SUBTITLE_JOBS", "SUBTITLE_DETECT_WORKER", "jobs.subtitle.detect"),
        "subtitle_transcribe": ("SUBTITLE_JOBS", "SUBTITLE_TRANSCRIBE_WORKER", "jobs.subtitle.transcribe"),
        "subtitle_translate": ("SUBTITLE_JOBS", "SUBTITLE_TRANSLATE_WORKER", "jobs.subtitle.translate"),
    }

    stream_name, consumer_name, subject = stream_map.get(
        worker_type,
        ("SUBTITLE_JOBS", "SUBTITLE_WORKER", f"jobs.subtitle.{worker_type}")
    )

    return Config(
        worker_type=worker_type,
        worker_id=os.getenv("WORKER_ID", ""),

        # NATS
        nats_url=os.getenv("NATS_URL", "nats://localhost:4222"),
        stream_name=os.getenv(f"{worker_type.upper()}_STREAM_NAME", stream_name),
        consumer_name=os.getenv(f"{worker_type.upper()}_CONSUMER_NAME", consumer_name),
        subject=os.getenv(f"{worker_type.upper()}_SUBJECT", subject),

        # S3 (auto-add https:// if missing)
        s3_endpoint=_ensure_https(os.getenv("S3_ENDPOINT", "")),
        s3_access_key=os.getenv("S3_ACCESS_KEY", ""),
        s3_secret_key=os.getenv("S3_SECRET_KEY", ""),
        s3_bucket=os.getenv("S3_BUCKET", "suekk"),
        s3_region=os.getenv("S3_REGION", "us-east-1"),

        # API
        api_base_url=os.getenv("API_BASE_URL", "http://localhost:8080"),
        api_internal_key=os.getenv("API_INTERNAL_KEY", ""),

        # Processing
        temp_dir=os.getenv("TEMP_DIR", "/tmp/subtitle"),
        log_dir=os.getenv("LOG_DIR", "/tmp/subtitle/logs"),
        concurrency=int(os.getenv("SUBTITLE_CONCURRENCY", "1")),

        # Whisper
        whisper_model=os.getenv("WHISPER_MODEL", "turbo"),
        whisper_device=os.getenv("WHISPER_DEVICE", "cuda"),
        whisper_compute_type=os.getenv("WHISPER_COMPUTE_TYPE", "int8_float16"),
        gap_model=os.getenv("GAP_MODEL", "kotoba"),
        gap_model_fallback=os.getenv("GAP_MODEL_FALLBACK", "large-v3"),

        # LLM
        gemini_api_key=os.getenv("GEMINI_API_KEY", ""),
        gemini_model=os.getenv("GEMINI_MODEL", "gemini-2.0-flash"),
        llm_provider=os.getenv("LLM_PROVIDER", "gemini"),
        deepseek_api_key=os.getenv("DEEPSEEK_API_KEY", ""),
        openai_api_key=os.getenv("OPENAI_API_KEY", ""),

        # Feature flags
        use_demucs=os.getenv("USE_DEMUCS", "true").lower() == "true",
        use_diarization=os.getenv("USE_DIARIZATION", "true").lower() == "true",
        use_vad=os.getenv("USE_VAD", "true").lower() == "true",
        use_refine=os.getenv("USE_REFINE", "true").lower() == "true",

        # Pipeline settings
        max_subtitle_duration=float(os.getenv("MAX_SUBTITLE_DURATION", "7.0")),
        min_gap_duration=float(os.getenv("MIN_GAP_DURATION", "0.5")),
        gap_no_speech_threshold=float(os.getenv("GAP_NO_SPEECH_THRESHOLD", "0.5")),
    )


def validate_config(config: Config) -> list[str]:
    """
    Validate configuration and return list of errors.

    Returns:
        List of error messages (empty if valid)
    """
    errors = []

    if not config.s3_endpoint:
        errors.append("S3_ENDPOINT is required")
    if not config.s3_access_key:
        errors.append("S3_ACCESS_KEY is required")
    if not config.s3_secret_key:
        errors.append("S3_SECRET_KEY is required")

    # Validate Gemini for transcribe/translate
    if config.worker_type in ["subtitle_transcribe", "subtitle_translate"]:
        if not config.gemini_api_key:
            errors.append("GEMINI_API_KEY is required for transcribe/translate workers")

    return errors
