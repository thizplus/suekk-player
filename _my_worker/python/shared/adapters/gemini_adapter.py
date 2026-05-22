"""
GeminiAdapter - LLM for translation and refinement using Google Gemini
"""

import os
import logging
from typing import List, Dict, Tuple, Optional, Any

from .entities import SubtitleLine, LanguageCode

logger = logging.getLogger(__name__)


class GeminiAdapter:
    """
    Google Gemini LLM adapter for translation and refinement.

    Usage:
        adapter = GeminiAdapter(api_key="xxx", model="gemini-2.0-flash")
        translated = adapter.translate_batch(lines, source, target)
    """

    def __init__(
        self,
        api_key: Optional[str] = None,
        model: Optional[str] = None,
    ):
        """
        Initialize Gemini adapter.

        Args:
            api_key: Gemini API key (or from GEMINI_API_KEY env)
            model: Model name (or from GEMINI_MODEL env)
        """
        self._api_key = api_key or os.getenv("GEMINI_API_KEY", "")
        self._model_name = model or os.getenv("GEMINI_MODEL", "gemini-2.5-flash")
        self._client = None

        if not self._api_key:
            logger.warning("GEMINI_API_KEY not set")

    def _get_client(self):
        """Lazy load Gemini client"""
        if self._client is None:
            import google.generativeai as genai

            genai.configure(api_key=self._api_key)
            self._client = genai.GenerativeModel(self._model_name)
            logger.info(f"Gemini client initialized: {self._model_name}")

        return self._client

    def translate_batch(
        self,
        lines: List[SubtitleLine],
        source_lang: LanguageCode,
        target_lang: LanguageCode,
        context: str = "",
        batch_size: int = 30,
    ) -> List[SubtitleLine]:
        """
        Translate a batch of subtitles.

        Args:
            lines: Subtitles to translate
            source_lang: Source language
            target_lang: Target language
            context: Video context
            batch_size: Lines per API call

        Returns:
            Translated subtitles
        """
        if not lines:
            return []

        client = self._get_client()
        results = {}

        # Process in batches
        for i in range(0, len(lines), batch_size):
            batch = lines[i:i + batch_size]

            # Build prompt
            prompt = self._build_translate_prompt(
                batch, source_lang, target_lang, context
            )

            try:
                response = client.generate_content(prompt)
                batch_results = self._parse_translate_response(response.text, batch)
                results.update(batch_results)
            except Exception as e:
                logger.error(f"Translation batch failed: {e}")
                # Keep original text on error
                for line in batch:
                    results[line.index] = line.text

        # Build result list
        translated = []
        for line in lines:
            new_text = results.get(line.index, line.text)
            translated.append(line.with_text(new_text))

        return translated

    def refine_batch(
        self,
        lines: List[SubtitleLine],
        language: LanguageCode,
        context: str = "",
        batch_size: int = 30,
    ) -> List[SubtitleLine]:
        """
        Refine/fix transcription errors.

        Args:
            lines: Subtitles to refine
            language: Language code
            context: Video context
            batch_size: Lines per API call

        Returns:
            Refined subtitles
        """
        if not lines:
            return []

        client = self._get_client()
        results = {}

        for i in range(0, len(lines), batch_size):
            batch = lines[i:i + batch_size]

            prompt = self._build_refine_prompt(batch, language, context)

            try:
                response = client.generate_content(prompt)
                batch_results = self._parse_translate_response(response.text, batch)
                results.update(batch_results)
            except Exception as e:
                logger.error(f"Refine batch failed: {e}")
                for line in batch:
                    results[line.index] = line.text

        refined = []
        for line in lines:
            new_text = results.get(line.index, line.text)
            refined.append(line.with_text(new_text))

        return refined

    def translate_cluster(
        self,
        lines: List[SubtitleLine],
        previous_summary: Optional[str],
        source_lang: LanguageCode,
        target_lang: LanguageCode,
        context: str = "",
        is_scene_change: bool = False,
    ) -> Tuple[List[SubtitleLine], str]:
        """
        Translate speech cluster with dynamic summary.

        Args:
            lines: Subtitles in this cluster
            previous_summary: Summary from previous cluster
            source_lang: Source language
            target_lang: Target language
            context: Video context
            is_scene_change: True if new scene (reset summary)

        Returns:
            Tuple of (translated lines, new summary)
        """
        if not lines:
            return [], previous_summary or ""

        client = self._get_client()

        # Build prompt with cluster context
        prompt = self._build_cluster_prompt(
            lines, previous_summary, source_lang, target_lang, context, is_scene_change
        )

        try:
            response = client.generate_content(prompt)
            results, new_summary = self._parse_cluster_response(response.text, lines)

            translated = []
            for line in lines:
                new_text = results.get(line.index, line.text)
                translated.append(line.with_text(new_text))

            return translated, new_summary

        except Exception as e:
            logger.error(f"Cluster translation failed: {e}")
            return lines, previous_summary or ""

    def _build_translate_prompt(
        self,
        lines: List[SubtitleLine],
        source_lang: LanguageCode,
        target_lang: LanguageCode,
        context: str,
    ) -> str:
        """Build translation prompt"""
        lines_text = "\n".join([f"{l.index}|{l.text}" for l in lines])

        return f"""Translate the following subtitles from {source_lang.name} to {target_lang.name}.

Context: {context or 'Video subtitle'}

Rules:
- Keep the index number before each line
- Translate naturally, not literally
- Keep short and readable (max 45 chars per line)
- Output format: index|translated_text

Subtitles:
{lines_text}

Translation:"""

    def _build_refine_prompt(
        self,
        lines: List[SubtitleLine],
        language: LanguageCode,
        context: str,
    ) -> str:
        """Build refinement prompt"""
        lines_text = "\n".join([f"{l.index}|{l.text}" for l in lines])

        return f"""Fix transcription errors in these {language.name} subtitles.

Context: {context or 'Video subtitle'}

Rules:
- Fix obvious typos and errors
- Keep natural speech patterns
- Keep the index number before each line
- Output format: index|fixed_text

Subtitles:
{lines_text}

Fixed:"""

    def _build_cluster_prompt(
        self,
        lines: List[SubtitleLine],
        previous_summary: Optional[str],
        source_lang: LanguageCode,
        target_lang: LanguageCode,
        context: str,
        is_scene_change: bool,
    ) -> str:
        """Build cluster translation prompt with context"""
        lines_text = "\n".join([
            f"{l.index}|{l.text}|{l.gender or 'unknown'}|{l.speaker_id or ''}"
            for l in lines
        ])

        summary_text = ""
        if previous_summary and not is_scene_change:
            summary_text = f"\nPrevious context: {previous_summary}"

        return f"""Translate these {source_lang.name} subtitles to {target_lang.name}.

Video context: {context or 'Video subtitle'}{summary_text}

Rules:
- Translate naturally with context awareness
- Consider speaker gender for pronouns
- Output format: index|translated_text
- After translations, provide a 1-2 sentence summary

Lines (index|text|gender|speaker):
{lines_text}

Translation:
[translations here]

Summary:
[brief summary for next cluster]"""

    def _parse_translate_response(
        self,
        response: str,
        lines: List[SubtitleLine]
    ) -> Dict[int, str]:
        """Parse translation response"""
        results = {}

        for line in response.strip().split("\n"):
            line = line.strip()
            if "|" in line:
                parts = line.split("|", 1)
                if len(parts) == 2:
                    try:
                        idx = int(parts[0].strip())
                        text = parts[1].strip()
                        results[idx] = text
                    except ValueError:
                        continue

        return results

    def _parse_cluster_response(
        self,
        response: str,
        lines: List[SubtitleLine]
    ) -> Tuple[Dict[int, str], str]:
        """Parse cluster response with summary"""
        results = {}
        summary = ""

        in_summary = False
        for line in response.strip().split("\n"):
            line = line.strip()

            if "Summary:" in line or in_summary:
                in_summary = True
                if "Summary:" in line:
                    summary = line.replace("Summary:", "").strip()
                else:
                    summary += " " + line.strip()
                continue

            if "|" in line:
                parts = line.split("|", 1)
                if len(parts) == 2:
                    try:
                        idx = int(parts[0].strip())
                        text = parts[1].strip()
                        results[idx] = text
                    except ValueError:
                        continue

        return results, summary.strip()[:100]  # Limit summary length

    def is_healthy(self) -> bool:
        """Check if Gemini API is available"""
        try:
            client = self._get_client()
            response = client.generate_content("Say 'ok'")
            return response.text is not None
        except Exception as e:
            logger.warning(f"Gemini health check failed: {e}")
            return False

    def get_provider_name(self) -> str:
        """Get provider name"""
        return "gemini"
