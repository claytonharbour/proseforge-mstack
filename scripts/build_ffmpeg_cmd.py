#!/usr/bin/env python3
"""
Generate FFmpeg command from segments.json for video narration.

Usage: python build_ffmpeg_cmd.py segments.json video.webm output.mp4 [--execute]

This generates an FFmpeg command that:
1. Takes the input video
2. Overlays each audio segment at its specified timestamp using adelay
3. Mixes all audio tracks together
4. Outputs an MP4 with H.264 video and AAC audio
"""

import json
import sys
import subprocess
from pathlib import Path


def build_ffmpeg_command(segments_file: str, video_path: str, output_path: str) -> list[str]:
    """Build FFmpeg command as a list of arguments."""
    with open(segments_file) as f:
        segments = json.load(f)

    if not segments:
        raise ValueError("No segments found in segments.json")

    # Audio directory is next to segments.json
    audio_dir = Path(segments_file).parent / 'audio'

    # Start building command
    cmd = ['ffmpeg', '-y']  # -y to overwrite output

    # Add video input
    cmd.extend(['-i', video_path])

    # Add audio inputs
    for seg in segments:
        audio_path = audio_dir / seg['audio_file']
        cmd.extend(['-i', str(audio_path)])

    # Build filter_complex
    filter_parts = []
    mixer_inputs = []

    for i, seg in enumerate(segments):
        stream_idx = i + 1  # Video is [0], audio starts at [1]
        delay_ms = seg['timestamp_ms']
        # adelay format: delay_left|delay_right (stereo)
        filter_parts.append(f'[{stream_idx}]adelay={delay_ms}|{delay_ms}[a{stream_idx}]')
        mixer_inputs.append(f'[a{stream_idx}]')

    # Combine all delayed audio with amix
    # normalize=0 prevents volume ducking
    filter_complex = '; '.join(filter_parts)
    filter_complex += f'; {"".join(mixer_inputs)}amix=inputs={len(segments)}:duration=longest:normalize=0[aout]'

    cmd.extend(['-filter_complex', filter_complex])

    # Map video and mixed audio
    cmd.extend(['-map', '0:v', '-map', '[aout]'])

    # Output encoding settings - optimized for screen recordings
    cmd.extend([
        '-c:v', 'libx264',      # H.264 video codec
        '-preset', 'medium',    # Balance speed/quality
        '-crf', '18',           # Higher quality (lower = better, 18 is visually lossless)
        '-tune', 'animation',   # Optimized for flat areas/sharp edges in screen recordings
        '-c:a', 'aac',          # AAC audio codec
        '-b:a', '192k',         # Audio bitrate
        output_path
    ])

    return cmd


def format_command_for_display(cmd: list[str]) -> str:
    """Format command for readable display with line breaks."""
    # Group arguments for readability
    lines = ['ffmpeg -y \\']

    i = 1  # Skip 'ffmpeg' and '-y'
    while i < len(cmd):
        arg = cmd[i]
        if arg == '-i':
            lines.append(f'  -i "{cmd[i+1]}" \\')
            i += 2
        elif arg == '-filter_complex':
            # Split filter_complex for readability
            fc = cmd[i+1]
            lines.append(f'  -filter_complex "')
            # Split on semicolons for multiline
            parts = fc.split('; ')
            for j, part in enumerate(parts):
                if j < len(parts) - 1:
                    lines.append(f'    {part};')
                else:
                    lines.append(f'    {part}')
            lines.append('  " \\')
            i += 2
        elif arg == '-map':
            lines.append(f'  -map {cmd[i+1]} \\')
            i += 2
        elif arg.startswith('-'):
            if i + 1 < len(cmd) and not cmd[i+1].startswith('-'):
                lines.append(f'  {arg} {cmd[i+1]} \\')
                i += 2
            else:
                lines.append(f'  {arg} \\')
                i += 1
        else:
            # Last argument (output file)
            lines.append(f'  "{arg}"')
            i += 1

    return '\n'.join(lines)


def main():
    if len(sys.argv) < 4:
        print("Usage: build_ffmpeg_cmd.py <segments.json> <video.webm> <output.mp4> [--execute]", file=sys.stderr)
        print("\nOptions:", file=sys.stderr)
        print("  --execute    Run the command instead of just printing it", file=sys.stderr)
        sys.exit(1)

    segments_file = sys.argv[1]
    video_path = sys.argv[2]
    output_path = sys.argv[3]
    execute = '--execute' in sys.argv

    # Validate inputs
    if not Path(segments_file).exists():
        print(f"Error: segments.json not found: {segments_file}", file=sys.stderr)
        sys.exit(1)

    if not Path(video_path).exists():
        print(f"Error: Video not found: {video_path}", file=sys.stderr)
        sys.exit(1)

    # Check audio files exist
    audio_dir = Path(segments_file).parent / 'audio'
    with open(segments_file) as f:
        segments = json.load(f)

    missing = []
    for seg in segments:
        audio_path = audio_dir / seg['audio_file']
        if not audio_path.exists():
            missing.append(seg['audio_file'])

    if missing:
        print(f"Error: Missing audio files in {audio_dir}:", file=sys.stderr)
        for f in missing[:5]:
            print(f"  - {f}", file=sys.stderr)
        if len(missing) > 5:
            print(f"  ... and {len(missing) - 5} more", file=sys.stderr)
        sys.exit(1)

    # Build command
    cmd = build_ffmpeg_command(segments_file, video_path, output_path)

    if execute:
        print("Executing FFmpeg command...", file=sys.stderr)
        print(f"Output: {output_path}", file=sys.stderr)
        result = subprocess.run(cmd, capture_output=False)
        sys.exit(result.returncode)
    else:
        # Print formatted command
        print(format_command_for_display(cmd))


if __name__ == '__main__':
    main()
