#!/usr/bin/env python3
"""
Batch process all videos in the input directory.

Finds all input directories containing both video.webm and narration.md,
then processes each sequentially. Stops on first failure.

Usage:
    python3 scripts/process_all.py [options]

Examples:
    python3 scripts/process_all.py               # Process all videos
    python3 scripts/process_all.py --upload      # Process and upload all
    python3 scripts/process_all.py --dry-run     # Preview what would run
"""

import argparse
import subprocess
import sys
from pathlib import Path

# Project directories
SCRIPT_DIR = Path(__file__).parent
PROJECT_DIR = SCRIPT_DIR.parent
INPUT_DIR = PROJECT_DIR / 'input'


def log(msg: str, level: str = 'info'):
    """Print formatted log message."""
    prefix = {
        'info': '\033[34m[INFO]\033[0m',
        'ok': '\033[32m[OK]\033[0m',
        'warn': '\033[33m[WARN]\033[0m',
        'error': '\033[31m[ERROR]\033[0m',
        'step': '\033[36m[STEP]\033[0m',
    }.get(level, '')
    print(f"{prefix} {msg}")


def find_all_videos() -> list[str]:
    """Find all input directories with both video.webm and narration.md."""
    videos = []

    if not INPUT_DIR.exists():
        return videos

    for d in sorted(INPUT_DIR.iterdir()):
        if not d.is_dir():
            continue

        has_video = (d / 'video.webm').exists()
        has_narration = (d / 'narration.md').exists()

        if has_video and has_narration:
            videos.append(d.name)

    return videos


def process_all(tts_engine: str = 'say', upload: bool = False, dry_run: bool = False) -> bool:
    """
    Process all videos. Returns True if all succeed, False on first failure.
    """
    videos = find_all_videos()

    if not videos:
        log("No videos found in input/ directory", 'warn')
        log("Ensure each video has both video.webm and narration.md", 'info')
        return False

    log(f"Found {len(videos)} video(s) to process:", 'info')
    for v in videos:
        print(f"  - {v}")
    print()

    if dry_run:
        log("Dry run mode - previewing steps only", 'info')
        print()

    processed = 0
    failed = None

    for i, video_name in enumerate(videos, 1):
        log(f"Processing video {i}/{len(videos)}: {video_name}", 'step')
        print('=' * 60)

        cmd = [
            'python3', str(SCRIPT_DIR / 'process_video.py'),
            video_name,
            '--tts-engine', tts_engine,
        ]
        if upload:
            cmd.append('--upload')
        if dry_run:
            cmd.append('--dry-run')

        result = subprocess.run(cmd)

        if result.returncode != 0:
            log(f"Failed processing: {video_name}", 'error')
            failed = video_name
            break

        processed += 1
        print()

    # Summary
    print('=' * 60)
    log("SUMMARY", 'info')
    print('=' * 60)
    print(f"  Total videos: {len(videos)}")
    print(f"  Processed: {processed}")
    print(f"  Failed: {1 if failed else 0}")

    if failed:
        log(f"Pipeline stopped at: {failed}", 'error')
        return False

    log(f"All {len(videos)} videos processed successfully!", 'ok')
    return True


def main():
    parser = argparse.ArgumentParser(
        description='Batch process all videos in input/',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s                      # Process all videos
  %(prog)s --upload             # Process and upload all to YouTube
  %(prog)s --tts-engine gemini  # Use Google Gemini TTS
  %(prog)s --dry-run            # Preview without executing
        '''
    )
    parser.add_argument('--tts-engine', choices=['say', 'gemini'], default='say',
                        help='TTS engine to use (default: say)')
    parser.add_argument('--upload', action='store_true',
                        help='Upload each video to YouTube after processing')
    parser.add_argument('--dry-run', action='store_true',
                        help='Preview steps without executing')

    args = parser.parse_args()

    success = process_all(
        tts_engine=args.tts_engine,
        upload=args.upload,
        dry_run=args.dry_run
    )

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
