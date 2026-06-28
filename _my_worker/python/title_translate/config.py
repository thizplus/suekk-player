"""
Title Translate Service configuration.
"""
from pathlib import Path
from pydantic_settings import BaseSettings

# หา .env จาก parent directories (shared กับ worker อื่น)
_THIS_DIR = Path(__file__).resolve().parent
_ENV_FILES = []
for parent in [_THIS_DIR.parent.parent, _THIS_DIR.parent, _THIS_DIR, Path.cwd()]:
    env_path = parent / ".env"
    if env_path.exists():
        _ENV_FILES.append(str(env_path))


class TitleTranslateConfig(BaseSettings):
    # Server
    host: str = "0.0.0.0"
    port: int = 8003
    debug: bool = False

    # Database (subth — separate from SUEKK)
    database_url: str = "postgresql://postgres:postgres@localhost:5433/subth"

    # LLM Provider (ใช้ shared Port/Adapter)
    llm_provider: str = "gemini"
    gemini_api_key: str = ""
    gemini_model: str = "gemini-2.5-flash"

    # Translation Settings
    batch_size: int = 50

    # Translations cache file
    translations_file: str = "data/translations_th.json"

    # API SUBTH (for cast management)
    api_subth_url: str = "https://api.subth.com/api/v1"
    api_email: str = ""
    api_password: str = ""

    # API SUEKK (for video management)
    api_suekk_url: str = "https://api.suekk.com/api/v1"
    api_suekk_email: str = ""
    api_suekk_password: str = ""

    class Config:
        env_file = tuple(_ENV_FILES) if _ENV_FILES else ".env"
        env_file_encoding = "utf-8"
        env_prefix = ""
        extra = "ignore"


_settings = None


def get_settings() -> TitleTranslateConfig:
    global _settings
    if _settings is None:
        _settings = TitleTranslateConfig()
    return _settings


# Proxy for lazy access
class _SettingsProxy:
    def __getattr__(self, name):
        return getattr(get_settings(), name)


settings = _SettingsProxy()
