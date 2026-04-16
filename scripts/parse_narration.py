#!/usr/bin/env python3
"""
Parse narration markdown files and output JSON with timing info.

Usage: python parse_narration.py input/admin-overview/narration.md
Output: JSON to stdout

Example output:
[
  {
    "index": 1,
    "timestamp": "00:01",
    "timestamp_ms": 1000,
    "text": "Enter your email address to sign in",
    "audio_file": "segment_001.mp3"
  },
  ...
]
"""

import re
import json
import sys
from pathlib import Path


def parse_timestamp(ts: str) -> int:
    """Convert MM:SS to milliseconds."""
    parts = ts.strip().split(':')
    minutes = int(parts[0])
    seconds = int(parts[1])
    return (minutes * 60 + seconds) * 1000


def parse_narration_md(filepath: str) -> list[dict]:
    """
    Parse markdown table format:
    | Time | Narration |
    |------|-----------|
    | 00:01 | Text here |
    """
    segments = []
    with open(filepath, 'r') as f:
        content = f.read()

    # Match table rows: | 00:01 | Text here |
    # This pattern matches lines like: | 00:01 | Enter your email address to sign in |
    pattern = r'\|\s*(\d{2}:\d{2})\s*\|\s*(.+?)\s*\|'
    matches = re.findall(pattern, content)

    for i, (timestamp, text) in enumerate(matches):
        segments.append({
            'index': i + 1,
            'timestamp': timestamp,
            'timestamp_ms': parse_timestamp(timestamp),
            'text': text.strip(),
            'audio_file': f'segment_{i+1:03d}.mp3'
        })

    return segments


def main():
    if len(sys.argv) < 2:
        print("Usage: parse_narration.py <narration.md>", file=sys.stderr)
        print("Output: JSON to stdout", file=sys.stderr)
        sys.exit(1)

    filepath = sys.argv[1]

    if not Path(filepath).exists():
        print(f"Error: File not found: {filepath}", file=sys.stderr)
        sys.exit(1)

    segments = parse_narration_md(filepath)

    if not segments:
        print("Warning: No segments found in file", file=sys.stderr)

    print(json.dumps(segments, indent=2))


if __name__ == '__main__':
    main()
