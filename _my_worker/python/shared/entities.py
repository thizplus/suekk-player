"""
Domain entities for subtitle processing.
"""

from dataclasses import dataclass, field
from typing import Optional, Dict, Any, List


@dataclass
class SubtitleLine:
    """A single subtitle line with timing"""
    index: int
    text: str
    start_ms: int
    end_ms: int
    speaker_id: Optional[str] = None
    gender: Optional[str] = None
    confidence: Optional[float] = None
    metadata: Optional[Dict[str, Any]] = None

    @property
    def start_sec(self) -> float:
        return self.start_ms / 1000.0

    @property
    def end_sec(self) -> float:
        return self.end_ms / 1000.0

    @property
    def duration_sec(self) -> float:
        return (self.end_ms - self.start_ms) / 1000.0

    def with_text(self, new_text: str) -> "SubtitleLine":
        """Return new SubtitleLine with updated text"""
        return SubtitleLine(
            index=self.index,
            text=new_text,
            start_ms=self.start_ms,
            end_ms=self.end_ms,
            speaker_id=self.speaker_id,
            gender=self.gender,
            confidence=self.confidence,
            metadata=self.metadata,
        )

    def with_timing(self, start_ms: int, end_ms: int) -> "SubtitleLine":
        """Return new SubtitleLine with updated timing"""
        return SubtitleLine(
            index=self.index,
            text=self.text,
            start_ms=start_ms,
            end_ms=end_ms,
            speaker_id=self.speaker_id,
            gender=self.gender,
            confidence=self.confidence,
            metadata=self.metadata,
        )


@dataclass
class LanguageCode:
    """Language code wrapper with utilities"""
    code: str

    # Language display names (Thai)
    NAMES_TH = {
        "ja": "ญี่ปุ่น",
        "en": "อังกฤษ",
        "zh": "จีน",
        "ko": "เกาหลี",
        "th": "ไทย",
        "ru": "รัสเซีย",
        "fr": "ฝรั่งเศส",
        "de": "เยอรมัน",
        "es": "สเปน",
        "it": "อิตาลี",
        "pt": "โปรตุเกส",
    }

    SUPPORTED = ["ja", "en", "zh", "ko", "th", "ru"]

    @property
    def name(self) -> str:
        """Get Thai display name"""
        return self.NAMES_TH.get(self.code, self.code)

    def is_supported(self) -> bool:
        """Check if language is supported"""
        return self.code in self.SUPPORTED

    @classmethod
    def detect_or_default(cls, code: str, default: str = "ja") -> "LanguageCode":
        """Create LanguageCode with fallback to default"""
        if code and code in cls.SUPPORTED:
            return cls(code)
        return cls(default)


@dataclass
class VoiceSegment:
    """Voice activity segment"""
    start_sec: float
    end_sec: float
    confidence: float = 1.0

    @property
    def duration_sec(self) -> float:
        return self.end_sec - self.start_sec


@dataclass
class TranscriptionResult:
    """Result of transcription"""
    subtitles: List[SubtitleLine]
    language: LanguageCode
    duration_sec: float
    model_name: str = ""


@dataclass
class SpeakerSegment:
    """Speaker diarization segment"""
    start: float
    end: float
    speaker_id: str
    gender: str = "unknown"


@dataclass
class DiarizationResult:
    """Speaker diarization result"""
    speakers: Dict[str, dict]
    segments: List[SpeakerSegment]

    @classmethod
    def from_dict(cls, data: dict) -> "DiarizationResult":
        """Create from dict"""
        segments = [
            SpeakerSegment(
                start=s.get("start", 0),
                end=s.get("end", 0),
                speaker_id=s.get("speaker", ""),
                gender=s.get("gender", "unknown"),
            )
            for s in data.get("segments", [])
        ]
        return cls(
            speakers=data.get("speakers", {}),
            segments=segments,
        )
