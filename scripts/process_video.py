#!/usr/bin/env python3
"""
Unified video narration pipeline.

Processes a video through the complete narration workflow:
1. Validate input files (video.webm, narration.md)
2. Parse narration markdown to JSON segments
3. Generate TTS audio for each segment
4. Check for audio overlaps (warns but continues)
5. Build final MP4 with FFmpeg
6. Copy to build/{project}/staging/ (for easy uploads via yutu CLI)
7. (Optional) Upload to YouTube

Usage:
    python3 scripts/process_video.py --project <project> <video-name> [options]

Examples:
    python3 scripts/process_video.py --project proseforge admin-overview
    python3 scripts/process_video.py --project proseforge admin-overview --upload
    python3 scripts/process_video.py --project proseforge admin-overview --tts-engine gemini
    python3 scripts/process_video.py admin-overview --dry-run  # uses default project
"""

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path

# Project directories
SCRIPT_DIR = Path(__file__).parent
ROOT_DIR = SCRIPT_DIR.parent
DEFAULT_PROJECT = 'proseforge'


def get_project_dirs(project: str) -> tuple[Path, Path, Path]:
    """Get project-specific directories."""
    input_dir = ROOT_DIR / 'input' / project
    build_dir = ROOT_DIR / 'build' / project
    staging_dir = build_dir / 'staging'
    return input_dir, build_dir, staging_dir


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


def find_video_dir(video_name: str, input_dir: Path) -> Path | None:
    """Find input directory matching video name (handles full or short names)."""
    # First try exact match
    exact = input_dir / video_name
    if exact.exists():
        return exact

    # Try pattern match (e.g., "admin-overview" matches "admin-overview.showcase.ts-...")
    for d in input_dir.iterdir():
        if d.is_dir() and d.name.startswith(video_name):
            return d

    return None


def validate_input(video_name: str, input_dir: Path) -> tuple[Path, Path, Path]:
    """
    Validate input files exist. Returns (video_dir, video_file, narration_file).
    Raises SystemExit on validation failure.
    """
    video_dir = find_video_dir(video_name, input_dir)

    if not video_dir:
        log(f"Input directory not found for '{video_name}'", 'error')
        log(f"Available videos in {input_dir.name}:", 'info')
        if input_dir.exists():
            for d in sorted(input_dir.iterdir()):
                if d.is_dir() and (d / 'narration.md').exists():
                    print(f"  - {d.name}")
        else:
            log(f"Input directory does not exist: {input_dir}", 'error')
        sys.exit(1)

    video_file = video_dir / 'video.webm'
    narration_file = video_dir / 'narration.md'

    errors = []
    if not video_file.exists():
        errors.append(f"video.webm not found in {video_dir}")
    if not narration_file.exists():
        errors.append(f"narration.md not found in {video_dir}")

    if errors:
        for e in errors:
            log(e, 'error')
        sys.exit(1)

    return video_dir, video_file, narration_file


def run_step(name: str, cmd: list[str], dry_run: bool = False) -> bool:
    """Run a pipeline step. Returns True on success."""
    log(f"{name}", 'step')

    if dry_run:
        log(f"Would run: {' '.join(str(c) for c in cmd)}", 'info')
        return True

    result = subprocess.run(cmd, capture_output=False)

    if result.returncode != 0:
        log(f"Step failed: {name}", 'error')
        return False

    return True


def process_video(video_name: str, project: str = DEFAULT_PROJECT, tts_engine: str = 'say', upload: bool = False, dry_run: bool = False) -> bool:
    """
    Process a video through the full narration pipeline.
    Returns True on success, False on failure.
    """
    log(f"Processing video: {video_name} (project: {project})", 'info')

    # Get project-specific directories
    input_dir, build_dir, staging_dir = get_project_dirs(project)

    # Step 1: Validate input
    log("Validating input files...", 'step')
    video_dir, video_file, narration_file = validate_input(video_name, input_dir)
    log(f"Input directory: {video_dir.name}", 'ok')

    # Setup build directory
    build_video_dir = build_dir / video_dir.name
    build_video_dir.mkdir(parents=True, exist_ok=True)
    audio_dir = build_video_dir / 'audio'
    audio_dir.mkdir(exist_ok=True)

    segments_file = build_video_dir / 'segments.json'
    output_file = build_video_dir / f"{video_name}.mp4"

    # Step 2: Parse narration
    if not dry_run:
        log("Parsing narration.md to segments.json...", 'step')
        result = subprocess.run(
            ['python3', str(SCRIPT_DIR / 'parse_narration.py'), str(narration_file)],
            capture_output=True, text=True
        )
        if result.returncode != 0:
            log(f"Failed to parse narration: {result.stderr}", 'error')
            return False

        segments_file.write_text(result.stdout)
        segments = json.loads(result.stdout)
        log(f"Parsed {len(segments)} segments", 'ok')
    else:
        log("Would parse narration.md to segments.json", 'info')

    # Step 3: Generate TTS audio
    if tts_engine == 'say':
        tts_cmd = [
            'python3', str(SCRIPT_DIR / 'generate_tts_say.py'),
            str(segments_file),
            '--output-dir', str(audio_dir),
            '--voice', 'Karen',
            '--rate', '200',
            '--update-json'
        ]
    else:
        tts_cmd = [
            'python3', str(SCRIPT_DIR / 'generate_tts.py'),
            str(segments_file),
            '--output-dir', str(audio_dir),
            '--voice', 'Kore',
            '--update-json'
        ]

    if not run_step(f"Generating TTS audio ({tts_engine})...", tts_cmd, dry_run):
        return False
    log("TTS audio generated", 'ok')

    # Step 4: Analyze overlaps (warn only, don't fail)
    if not dry_run:
        log("Checking for audio overlaps...", 'step')
        result = subprocess.run(
            ['python3', str(SCRIPT_DIR / 'analyze_overlap.py'), video_dir.name],
            capture_output=True, text=True
        )
        if result.returncode != 0:
            log("Audio overlaps detected! Review output above and consider shortening text.", 'warn')
            print(result.stdout)
        else:
            log("No critical overlaps detected", 'ok')
    else:
        log("Would check for audio overlaps", 'info')

    # Step 5: Build final video
    build_cmd = [
        'python3', str(SCRIPT_DIR / 'build_ffmpeg_cmd.py'),
        str(segments_file),
        str(video_file),
        str(output_file),
        '--execute'
    ]

    if not run_step("Building final video with FFmpeg...", build_cmd, dry_run):
        return False
    log(f"Output: {output_file}", 'ok')

    # Step 6: Copy to staging directory (for easy uploads)
    if not dry_run:
        staging_dir.mkdir(parents=True, exist_ok=True)
        # Extract short name from video_name (e.g., "admin-overview" from "admin-overview.showcase.ts-...")
        short_name = video_name.split('.')[0] if '.' in video_name else video_name
        staged_file = staging_dir / f"{short_name}.mp4"
        shutil.copy2(output_file, staged_file)
        log(f"Staged: {staged_file}", 'ok')
    else:
        log(f"Would copy to {staging_dir}/", 'info')

    # Step 7: Upload to YouTube (optional)
    if upload:
        title = video_name.replace('-', ' ').title()
        short_name = video_name.split('.')[0] if '.' in video_name else video_name
        staged_file = staging_dir / f"{short_name}.mp4"
        upload_cmd = [
            'yutu', 'video', 'insert',
            f'--file={staged_file}',
            f'--title=ProseForge: {title}',
            f'--description=ProseForge tutorial - {title}. Learn more at proseforge.ai',
            '--privacy=public',
            '--categoryId=28'
        ]

        if not run_step("Uploading to YouTube...", upload_cmd, dry_run):
            return False
        log("Video uploaded to YouTube", 'ok')

    log(f"Pipeline complete for {video_name}!", 'ok')
    return True


def main():
    parser = argparse.ArgumentParser(
        description='Process a video through the narration pipeline',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  %(prog)s --project proseforge admin-overview           # Process with macOS TTS
  %(prog)s --project proseforge admin-overview --upload  # Process and upload
  %(prog)s admin-overview --tts-engine gemini            # Use default project
  %(prog)s admin-overview --dry-run                      # Preview without executing
        '''
    )
    parser.add_argument('video_name', help='Name of video to process')
    parser.add_argument('--project', '-p', default=DEFAULT_PROJECT,
                        help=f'Project name (default: {DEFAULT_PROJECT})')
    parser.add_argument('--tts-engine', choices=['say', 'gemini'], default='say',
                        help='TTS engine to use (default: say)')
    parser.add_argument('--upload', action='store_true',
                        help='Upload to YouTube after processing')
    parser.add_argument('--dry-run', action='store_true',
                        help='Preview steps without executing')

    args = parser.parse_args()

    success = process_video(
        args.video_name,
        project=args.project,
        tts_engine=args.tts_engine,
        upload=args.upload,
        dry_run=args.dry_run
    )

    sys.exit(0 if success else 1)


if __name__ == '__main__':
    main()
