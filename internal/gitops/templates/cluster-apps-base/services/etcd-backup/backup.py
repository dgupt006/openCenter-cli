#!/usr/bin/env python3
"""Upload an etcd snapshot to the configured S3-compatible bucket."""

import os
import sys
from pathlib import Path
from typing import List

import boto3
from botocore.client import Config
from botocore.exceptions import BotoCoreError, ClientError


def required_env(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise ValueError(f"{name} must be set")
    return value


def upload_snapshot(snapshot: Path) -> None:
    access_key = required_env("ACCESS_KEY")
    secret_key = required_env("SECRET_KEY")
    endpoint = required_env("S3_HOST")
    region = required_env("S3_REGION")
    bucket = required_env("S3_BUCKET_NAME")

    if not snapshot.is_file():
        raise ValueError(f"snapshot does not exist: {snapshot}")

    client = boto3.client(
        "s3",
        endpoint_url=endpoint,
        region_name=region,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        config=Config(signature_version="s3v4"),
    )
    client.head_bucket(Bucket=bucket)
    client.upload_file(str(snapshot), bucket, snapshot.name)


def main(argv: List[str]) -> int:
    if len(argv) != 2:
        print(f"usage: {argv[0]} SNAPSHOT", file=sys.stderr)
        return 2

    try:
        upload_snapshot(Path(argv[1]))
    except (BotoCoreError, ClientError, OSError, ValueError) as error:
        print(f"etcd snapshot upload failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
