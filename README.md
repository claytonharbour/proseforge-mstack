# mstack

Video narration pipeline and marketing tools for content production. Generates TTS audio from timestamped markdown scripts, mixes narration onto silent screen recordings using FFmpeg, and uploads finished videos to YouTube.

Also includes tools for social media campaign management, Google Forms, and secrets management via Bitwarden.

## Installation

Download the latest release from [GitHub Releases](https://github.com/claytonharbour/proseforge-mstack/releases).

### macOS (Apple Silicon)

```bash
curl -L https://github.com/claytonharbour/proseforge-mstack/releases/latest/download/mstack_darwin_arm64.tar.gz | tar xz
chmod +x mstack
sudo mv mstack /usr/local/bin/
```

### MCP Server

The `mstack-mcp` binary provides MCP tools for AI assistants:

```bash
curl -L https://github.com/claytonharbour/proseforge-mstack/releases/latest/download/mstack-mcp_darwin_arm64.tar.gz | tar xz
chmod +x mstack-mcp
```

Configure in Claude Code:

```bash
claude mcp add mstack -- /path/to/mstack-mcp
```

## Quick Start

```bash
# Show all commands
mstack --help
mstack video --help

# Parse narration markdown to JSON
mstack video parse input/narration.md > segments.json

# Generate TTS audio (macOS say)
mstack video tts say segments.json --output-dir audio/

# Generate TTS audio (Google Gemini)
mstack video tts gemini segments.json --output-dir audio/

# Build final video with FFmpeg
mstack video build segments.json video.webm output.mp4 --execute

# Full pipeline
mstack video process --project myproject video-name

# Upload to YouTube
mstack video youtube upload --file=output.mp4 --title="My Video"
```

## Narration Format

Narration scripts use markdown tables with timestamps:

```markdown
| Time | Narration |
|------|-----------|
| 00:01 | Enter your email address to sign in |
| 00:05 | Then enter your password |
```

## Configuration

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

Key settings:
- `GOOGLE_AI_API_KEY` -- Google AI API key for Gemini TTS
- `TTS_VOICE` -- Voice name (Kore, Aoede, Puck, Charon)
- `TTS_SPEED` -- Speaking rate (0.25-4.0)
- `YOUTUBE_CHANNEL_ID` -- Your YouTube channel ID
- `YOUTUBE_PLAYLIST_ID` -- Target playlist for uploads

## Building from Source

Requires Go 1.22+.

```bash
cd src
go build -o ../bin/mstack ./cmd/mstack
go build -o ../bin/mstack-mcp ./cmd/mstack-mcp
```

## License

MIT
