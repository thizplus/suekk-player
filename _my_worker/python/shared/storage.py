"""
S3 client for Python workers.
Uses boto3 for S3-compatible storage.
"""

import logging
import os
from pathlib import Path
from typing import BinaryIO, Callable, Optional

import boto3
from botocore.config import Config as BotoConfig

from .config import Config

logger = logging.getLogger(__name__)

# Type alias for progress callback
ProgressCallback = Callable[[int, int], None]


class S3Client:
    """S3-compatible storage client"""

    def __init__(self, config: Config):
        self.config = config
        self.bucket = config.s3_bucket

        # Create boto3 client with retry config
        self.client = boto3.client(
            "s3",
            endpoint_url=config.s3_endpoint,
            aws_access_key_id=config.s3_access_key,
            aws_secret_access_key=config.s3_secret_key,
            region_name=config.s3_region,
            config=BotoConfig(
                signature_version="s3v4",
                s3={"addressing_style": "path"},
                retries={
                    "max_attempts": 10,
                    "mode": "adaptive",  # adaptive backoff with rate limiting
                },
                connect_timeout=30,
                read_timeout=300,
            ),
        )

        logger.info(f"S3 client initialized: {config.s3_endpoint}")

    def download(
        self,
        remote_path: str,
        local_path: str,
        on_progress: Optional[ProgressCallback] = None,
    ) -> None:
        """
        Download a file from S3 to local path.

        Args:
            remote_path: S3 key
            local_path: Local file path
            on_progress: Optional callback(downloaded, total)
        """
        # Create local directory
        Path(local_path).parent.mkdir(parents=True, exist_ok=True)

        # Get file size for progress (with manual retry for 503)
        total_size = 0
        for attempt in range(3):
            try:
                head = self.client.head_object(Bucket=self.bucket, Key=remote_path)
                total_size = head.get("ContentLength", 0)
                logger.info(f"File exists: {remote_path} ({total_size / 1024 / 1024:.1f} MB)")
                break
            except self.client.exceptions.ClientError as e:
                error_code = e.response["Error"]["Code"]
                error_msg = e.response["Error"]["Message"]
                if error_code in ("503", "500", "429") and attempt < 2:
                    wait = (attempt + 1) * 10
                    logger.warning(f"HeadObject {error_code}, retrying in {wait}s (attempt {attempt+1}/3): {remote_path}")
                    import time
                    time.sleep(wait)
                    continue
                logger.error(f"HeadObject failed: {remote_path} → {error_code}: {error_msg}")
                raise Exception(f"File not accessible: {remote_path} ({error_code}: {error_msg})")
            except Exception as e:
                logger.error(f"HeadObject unexpected error: {remote_path} → {type(e).__name__}: {e}")
                raise

        # Download with progress tracking
        downloaded = 0

        def progress_callback(bytes_amount):
            nonlocal downloaded
            downloaded += bytes_amount
            if on_progress and total_size > 0:
                on_progress(downloaded, total_size)

        try:
            self.client.download_file(
                self.bucket,
                remote_path,
                local_path,
                Callback=progress_callback if on_progress else None,
            )
            logger.info(f"Downloaded: {remote_path} → {local_path}")
        except Exception as e:
            logger.error(f"Download failed: {remote_path} → {type(e).__name__}: {e}")
            raise

        logger.debug(f"Downloaded: {remote_path} -> {local_path}")

    def upload(
        self,
        remote_path: str,
        local_path: str,
        content_type: Optional[str] = None,
    ) -> None:
        """
        Upload a local file to S3.

        Args:
            remote_path: S3 key
            local_path: Local file path
            content_type: Optional content type
        """
        extra_args = {}
        if content_type:
            extra_args["ContentType"] = content_type
        else:
            # Guess content type from extension
            ext = Path(local_path).suffix.lower()
            content_types = {
                ".srt": "text/plain; charset=utf-8",
                ".vtt": "text/vtt",
                ".json": "application/json",
                ".wav": "audio/wav",
                ".mp3": "audio/mpeg",
                ".txt": "text/plain; charset=utf-8",
            }
            if ext in content_types:
                extra_args["ContentType"] = content_types[ext]

        self.client.upload_file(
            local_path,
            self.bucket,
            remote_path,
            ExtraArgs=extra_args if extra_args else None,
        )

        logger.debug(f"Uploaded: {local_path} -> {remote_path}")

    def upload_bytes(
        self,
        remote_path: str,
        data: bytes,
        content_type: str = "application/octet-stream",
    ) -> None:
        """
        Upload bytes directly to S3.

        Args:
            remote_path: S3 key
            data: Bytes to upload
            content_type: Content type
        """
        self.client.put_object(
            Bucket=self.bucket,
            Key=remote_path,
            Body=data,
            ContentType=content_type,
        )

        logger.debug(f"Uploaded bytes: {remote_path} ({len(data)} bytes)")

    def delete(self, remote_path: str) -> None:
        """
        Delete a file from S3.

        Args:
            remote_path: S3 key
        """
        self.client.delete_object(Bucket=self.bucket, Key=remote_path)
        logger.debug(f"Deleted: {remote_path}")

    def exists(self, remote_path: str) -> bool:
        """
        Check if a file exists in S3.

        Args:
            remote_path: S3 key

        Returns:
            True if file exists
        """
        try:
            self.client.head_object(Bucket=self.bucket, Key=remote_path)
            return True
        except Exception:
            return False

    def get_presigned_url(self, remote_path: str, expires_in: int = 3600) -> str:
        """
        Generate a presigned URL for reading.

        Args:
            remote_path: S3 key
            expires_in: URL expiration in seconds

        Returns:
            Presigned URL
        """
        url = self.client.generate_presigned_url(
            "get_object",
            Params={"Bucket": self.bucket, "Key": remote_path},
            ExpiresIn=expires_in,
        )
        return url

    def list_files(self, prefix: str) -> list[str]:
        """
        List files with a given prefix.

        Args:
            prefix: S3 key prefix

        Returns:
            List of S3 keys
        """
        files = []
        paginator = self.client.get_paginator("list_objects_v2")

        for page in paginator.paginate(Bucket=self.bucket, Prefix=prefix):
            for obj in page.get("Contents", []):
                files.append(obj["Key"])

        return files
