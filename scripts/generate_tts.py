#!/usr/bin/env python3
"""
Generate TTS audio for all segments in a video.

Usage:
    python generate_tts.py segments.json --output-dir audio/ --voice Charon
"""

import argparse
import json
import os
import sys

# Add scripts directory to path for google_tts import
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from google_tts import generate_speech


def main():
    parser = argparse.ArgumentParser(description="Generate TTS audio for video segments")
    parser.add_argument("segments_file", help="Path to segments.json")
    parser.add_argument("--output-dir", required=True, help="Output directory for audio files")
    parser.add_argument("--voice", default="Charon", help="Voice name (default: Charon)")
    parser.add_argument("--update-json", action="store_true", help="Update segments.json with .wav extension")

    args = parser.parse_args()

    # Load segments
    with open(args.segments_file) as f:
        segments = json.load(f)

    # Create output directory
    os.makedirs(args.output_dir, exist_ok=True)

    # Generate audio for each segment (skip existing)
    for seg in segments:
        out = os.path.join(args.output_dir, f"segment_{seg['index']:03d}.wav")
        text_preview = seg['text'][:50] + '...' if len(seg['text']) > 50 else seg['text']

        # Skip if file already exists
        if os.path.exists(out):
            print(f"  Segment {seg['index']:02d}: [exists] {text_preview}")
            continue

        print(f"  Segment {seg['index']:02d}: {text_preview}")
        generate_speech(seg['text'], out, voice=args.voice)

    print(f"Generated {len(segments)} audio files.")

    # Update segments.json if requested
    if args.update_json:
        for s in segments:
            s['audio_file'] = s['audio_file'].replace('.mp3', '.wav').replace('.m4a', '.wav')
        with open(args.segments_file, 'w') as f:
            json.dump(segments, f, indent=2)
        print(f"Updated {args.segments_file}")


if __name__ == "__main__":
    main()
