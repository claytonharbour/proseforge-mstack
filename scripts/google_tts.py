#!/usr/bin/env python3
"""
Google Gemini TTS - Generate speech and save to audio file.

Usage:
    python google_tts.py "Hello world" output.wav
    python google_tts.py "Hello world" output.wav --voice Charon
"""

import argparse
import os
import re
import sys
import time
import wave

from google import genai
from google.genai import types
from google.genai.errors import ClientError


def wave_file(filename: str, pcm: bytes, channels: int = 1, rate: int = 24000, sample_width: int = 2):
    """Save PCM audio data to a WAV file."""
    with wave.open(filename, "wb") as wf:
        wf.setnchannels(channels)
        wf.setsampwidth(sample_width)
        wf.setframerate(rate)
        wf.writeframes(pcm)


def generate_speech(text: str, output_path: str, voice: str = "Charon", model: str = "gemini-2.5-flash-preview-tts", max_retries: int = 5):
    """Generate speech from text and save to file with retry logic."""
    api_key = os.environ.get("GOOGLE_AI_API_KEY")
    if not api_key:
        print("Error: GOOGLE_AI_API_KEY environment variable not set", file=sys.stderr)
        sys.exit(1)

    client = genai.Client(api_key=api_key)

    for attempt in range(max_retries):
        try:
            response = client.models.generate_content(
                model=model,
                contents=text,
                config=types.GenerateContentConfig(
                    response_modalities=["AUDIO"],
                    speech_config=types.SpeechConfig(
                        voice_config=types.VoiceConfig(
                            prebuilt_voice_config=types.PrebuiltVoiceConfig(
                                voice_name=voice,
                            )
                        )
                    ),
                )
            )

            # Extract audio data from response
            data = response.candidates[0].content.parts[0].inline_data.data
            wave_file(output_path, data)
            return output_path

        except ClientError as e:
            if "429" in str(e) or "RESOURCE_EXHAUSTED" in str(e):
                # Extract retry delay from error message if available
                match = re.search(r'retry in (\d+\.?\d*)s', str(e))
                wait_time = float(match.group(1)) + 1 if match else (2 ** attempt) * 10
                print(f"    Rate limited, waiting {wait_time:.1f}s (attempt {attempt + 1}/{max_retries})...")
                time.sleep(wait_time)
            else:
                raise

    raise Exception(f"Failed after {max_retries} retries")


def main():
    parser = argparse.ArgumentParser(description="Generate speech using Google Gemini TTS")
    parser.add_argument("text", help="Text to convert to speech")
    parser.add_argument("output", help="Output WAV file path")
    parser.add_argument("--voice", default="Charon",
                        help="Voice name (default: Charon). Options: Kore, Aoede, Puck, Charon, Zephyr, etc.")
    parser.add_argument("--model", default="gemini-2.5-flash-preview-tts",
                        help="TTS model to use")

    args = parser.parse_args()

    output = generate_speech(args.text, args.output, args.voice, args.model)
    print(f"Generated: {output}")


if __name__ == "__main__":
    main()
