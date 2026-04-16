#!/usr/bin/env python3
"""Analyze audio segment timing to detect overlaps."""

import json
import subprocess
import sys
from pathlib import Path

def get_audio_duration(audio_path: str) -> float:
    """Get duration of audio file in milliseconds."""
    result = subprocess.run(
        ['ffprobe', '-v', 'error', '-show_entries', 'format=duration',
         '-of', 'csv=p=0', audio_path],
        capture_output=True, text=True
    )
    return float(result.stdout.strip()) * 1000  # Convert to ms

def analyze_segments(segments_path: str, audio_dir: str) -> dict:
    """Analyze segments for timing issues."""
    with open(segments_path) as f:
        segments = json.load(f)

    results = {
        'total_segments': len(segments),
        'overlaps': [],
        'tight_fits': [],  # < 500ms gap
        'good_fits': [],
        'summary': {}
    }

    for i, seg in enumerate(segments):
        audio_file = Path(audio_dir) / seg['audio_file']
        if not audio_file.exists():
            continue

        duration_ms = get_audio_duration(str(audio_file))
        start_ms = seg['timestamp_ms']
        end_ms = start_ms + duration_ms

        # Check against next segment
        if i < len(segments) - 1:
            next_start = segments[i + 1]['timestamp_ms']
            gap = next_start - end_ms

            seg_info = {
                'segment': seg['index'],
                'text': seg['text'][:50] + '...' if len(seg['text']) > 50 else seg['text'],
                'start_ms': start_ms,
                'duration_ms': round(duration_ms),
                'end_ms': round(end_ms),
                'next_start_ms': next_start,
                'gap_ms': round(gap)
            }

            if gap < 0:
                results['overlaps'].append(seg_info)
            elif gap < 500:
                results['tight_fits'].append(seg_info)
            else:
                results['good_fits'].append(seg_info)

    results['summary'] = {
        'overlaps': len(results['overlaps']),
        'tight_fits': len(results['tight_fits']),
        'good_fits': len(results['good_fits'])
    }

    return results

def print_report(results: dict, video_name: str):
    """Print a formatted report."""
    print(f"\n{'='*60}")
    print(f"AUDIO TIMING ANALYSIS: {video_name}")
    print(f"{'='*60}")
    print(f"Total segments: {results['total_segments']}")
    print(f"Overlaps: {results['summary']['overlaps']}")
    print(f"Tight fits (<500ms gap): {results['summary']['tight_fits']}")
    print(f"Good fits (>500ms gap): {results['summary']['good_fits']}")

    if results['overlaps']:
        print(f"\n{'─'*60}")
        print("OVERLAPPING SEGMENTS (audio extends into next segment):")
        print(f"{'─'*60}")
        for seg in results['overlaps']:
            print(f"  Seg {seg['segment']:02d}: overlap by {abs(seg['gap_ms'])}ms")
            print(f"         \"{seg['text']}\"")

    if results['tight_fits']:
        print(f"\n{'─'*60}")
        print("TIGHT FITS (<500ms gap - may sound rushed):")
        print(f"{'─'*60}")
        for seg in results['tight_fits']:
            print(f"  Seg {seg['segment']:02d}: {seg['gap_ms']}ms gap")

    if not results['overlaps'] and not results['tight_fits']:
        print(f"\n✓ All segments have comfortable timing!")

    return len(results['overlaps']) == 0

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: analyze_overlap.py <video-name>")
        print("Example: analyze_overlap.py admin-overview")
        sys.exit(1)

    video_name = sys.argv[1]
    build_dir = Path(__file__).parent.parent / 'build' / video_name
    segments_path = build_dir / 'segments.json'
    audio_dir = build_dir / 'audio'

    if not segments_path.exists():
        print(f"Error: {segments_path} not found")
        sys.exit(1)

    results = analyze_segments(str(segments_path), str(audio_dir))
    ok = print_report(results, video_name)
    sys.exit(0 if ok else 1)
