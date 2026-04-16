#!/usr/bin/env python3
"""
Generate TTS audio using macOS say command.

Usage:
    python generate_tts_say.py segments.json --output-dir audio/ --voice Karen --rate 200
"""

import argparse
import json
import os
import subprocess


def main():
    parser = argparse.ArgumentParser(description="Generate TTS audio using macOS say")
    parser.add_argument("segments_file", help="Path to segments.json")
    parser.add_argument("--output-dir", required=True, help="Output directory for audio files")
    parser.add_argument("--voice", default="Karen", help="Voice name (default: Karen)")
    parser.add_argument("--rate", type=int, default=200, help="Speaking rate in wpm (default: 200)")
    parser.add_argument("--update-json", action="store_true", help="Update segments.json with .m4a extension")

    args = parser.parse_args()

    # Load segments
    with open(args.segments_file) as f:
        segments = json.load(f)

    # Create output directory
    os.makedirs(args.output_dir, exist_ok=True)

    # Generate audio for each segment
    for seg in segments:
        out = os.path.join(args.output_dir, f"segment_{seg['index']:03d}.m4a")
        text_preview = seg['text'][:50] + '...' if len(seg['text']) > 50 else seg['text']

        # Skip if file already exists
        if os.path.exists(out):
            print(f"  Segment {seg['index']:02d}: [exists] {text_preview}")
            continue

        print(f"  Segment {seg['index']:02d}: {text_preview}")
        subprocess.run([
            'say',
            '-v', args.voice,
            '-r', str(args.rate),
            '--file-format=mp4f',
            '-o', out,
            seg['text']
        ], check=True)

    print(f"Generated {len(segments)} audio files.")

    # Update segments.json if requested
    if args.update_json:
        for s in segments:
            s['audio_file'] = s['audio_file'].replace('.mp3', '.m4a').replace('.wav', '.m4a')
        with open(args.segments_file, 'w') as f:
            json.dump(segments, f, indent=2)
        print(f"Updated {args.segments_file}")


if __name__ == "__main__":
    main()
