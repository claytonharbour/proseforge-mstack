# Marketing Stack - Video Narration Pipeline
# ==========================================

# Load environment variables
-include .env
export

# Default project (override: make process PROJECT=other-app VIDEO=xxx)
PROJECT ?= proseforge

# Default voice (can be overridden via .env or command line)
TTS_VOICE ?= Kore
TTS_SPEED ?= 1.0
GOBIN ?= $(HOME)/go/bin

.PHONY: help install install-deps install-mcp check-deps test build build-all clean

help: ## Show this help
	@echo "Marketing Stack - Video Narration Pipeline"
	@echo "==========================================="
	@echo ""
	@echo "Quick Start:"
	@echo "  make process VIDEO=<name>                    Full pipeline: parse → TTS → build"
	@echo "  make process VIDEO=<name> UPLOAD=1           Full pipeline with YouTube upload"
	@echo "  make process VIDEO=<name> PROJECT=proseforge Specify project (default: proseforge)"
	@echo "  make process-all                             Process all videos in input/"
	@echo ""
	@echo "Setup:"
	@echo "  install        Install all dependencies and configure MCP servers"
	@echo "  install-deps   Install CLI tools (mcp-tts, ffmpeg)"
	@echo "  install-mcp    Configure Claude Code MCP servers"
	@echo "  check-deps     Verify all dependencies are installed"
	@echo ""
	@echo "Development:"
	@echo "  test           Run all tests"
	@echo "  build          Build all binaries for current platform"
	@echo "  build-all      Build for all platforms (CI/CD use)"
	@echo "  clean          Clean all build artifacts (excluding input/)"
	@echo ""
	@echo "Configuration:"
	@echo "  PROJECT=$(PROJECT)  (default project)"
	@echo "  TTS_VOICE=$(TTS_VOICE)  (set in .env or override: make narrate TTS_VOICE=Aoede)"
	@echo "  TTS_SPEED=$(TTS_SPEED)"
	@echo ""

# =============================================================================
# Installation
# =============================================================================

install: install-deps install-mcp check-deps ## Full installation
	@echo ""
	@echo "Installation complete!"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Sample voices at: https://console.cloud.google.com/speech/text-to-speech"
	@echo "  2. Update TTS_VOICE in .env if desired (default: Kore)"
	@echo "  3. For YouTube uploads, run: mstack video youtube auth --client-secret=/path/to/client_secret.json"
	@echo ""

install-deps: ## Install CLI dependencies
	@echo "Installing dependencies..."
	@# Check for Go
	@command -v go >/dev/null 2>&1 || { echo "Go not found. Install with: brew install go"; exit 1; }
	@# Install mcp-tts
	@echo "Installing mcp-tts..."
	go install github.com/blacktop/mcp-tts@latest
	@# Install ffmpeg
	@command -v ffmpeg >/dev/null 2>&1 || { echo "Installing ffmpeg..."; brew install ffmpeg; }
	@# Install Bitwarden CLI
	@echo "Installing Bitwarden CLI..."
	brew install bitwarden-cli 2>/dev/null || brew upgrade bitwarden-cli 2>/dev/null || true
	@echo "Dependencies installed."

install-mcp: ## Configure Claude Code MCP servers
	@echo "Configuring Claude Code MCP servers..."
	@# Check for .env
	@test -f .env || { echo "Error: .env file not found. Copy .env.example to .env and add your API key."; exit 1; }
	@# Add mcp-tts server
	@echo "Adding mcp-tts server..."
	claude mcp add tts -e GOOGLE_AI_API_KEY=$(GOOGLE_AI_API_KEY) -- $(GOBIN)/mcp-tts || true
	@# Add mstack server
	@echo "Adding mstack server..."
	claude mcp add mstack -- $(BIN_DIR)/mstack-mcp || true
	@echo "MCP servers configured."
	@echo ""
	@echo "Restart Claude Code to load the new MCP servers."

check-deps: ## Verify all dependencies are installed
	@echo "Checking dependencies..."
	@echo -n "  mcp-tts: " && (command -v mcp-tts >/dev/null 2>&1 && echo "OK" || echo "MISSING")
	@echo -n "  ffmpeg: " && (command -v ffmpeg >/dev/null 2>&1 && echo "OK" || echo "MISSING")
	@echo -n "  bw (bitwarden): " && (command -v bw >/dev/null 2>&1 && echo "OK" || echo "MISSING")
	@echo -n "  mstack: " && (test -f $(BIN_DIR)/mstack && echo "OK" || echo "MISSING - run make build")
	@echo -n "  project .env: " && (test -f projects/$(PROJECT)/.env && echo "OK" || echo "MISSING - copy .env.example")
	@echo -n "  go: " && (command -v go >/dev/null 2>&1 && echo "OK" || echo "MISSING")

# =============================================================================
# Testing and Building
# =============================================================================

SRC_DIR := $(CURDIR)/src

test: ## Run all tests
	@echo "Running tests..."
	@cd $(SRC_DIR) && go test ./... -v
	@echo "✅ Tests complete"

test-functional: build ## Run functional tests only
	@echo "Running functional tests..."
	@cd $(SRC_DIR) && go test -v ./tests/functional/...
	@echo "✅ Functional tests complete"

build: ## Build mstack + mstack-mcp for current platform
	@echo "Building mstack + mstack-mcp..."
	@mkdir -p $(BIN_DIR)
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack-mcp ./cmd/mstack-mcp
	@echo "✅ Build complete"

build-all: ## Build for all platforms (CI/CD use)
	@echo "Building for all platforms..."
	@mkdir -p $(BIN_DIR)
	@for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} $(MAKE) _build-platform GOOS=$${platform%/*} GOARCH=$${platform#*/}; \
	done
	@echo "✅ Multi-platform build complete"

_build-platform: ## Internal: Build mstack for GOOS/GOARCH
	@SUFFIX=$$([ "$(GOOS)" = "windows" ] && echo ".exe" || echo ""); \
	ARCH_SUFFIX=$$(echo "$(GOARCH)" | sed 's/amd64/x86_64/;s/arm64/arm64/'); \
	echo "Building $(GOOS)/$(GOARCH) executable..."; \
	cd $(SRC_DIR) && GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BIN_DIR)/mstack-$(GOOS)-$$ARCH_SUFFIX$$SUFFIX ./cmd/mstack

clean: ## Clean all build artifacts (excluding input/)
	@echo "Cleaning build artifacts..."
	@rm -rf $(CURDIR)/build
	@echo "✅ Clean complete (input/ folder preserved)"

# =============================================================================
# Video Narration Pipeline
# =============================================================================

SCRIPTS_DIR := $(CURDIR)/scripts
BUILD_DIR := $(CURDIR)/build/$(PROJECT)
INPUT_DIR := $(CURDIR)/input/$(PROJECT)
PROJECTS_DIR := $(CURDIR)/projects/$(PROJECT)
BIN_DIR := $(CURDIR)/bin

.PHONY: list-videos process process-all narrate-parse narrate-tts narrate-tts-say narrate-build narrate-all clean-audio validate validate-all

list-videos: ## List available videos to narrate
	@echo "Available videos for project '$(PROJECT)':"
	@ls -1 $(INPUT_DIR)/*/narration.md 2>/dev/null | sed 's|$(INPUT_DIR)/||;s|/narration.md||' | sort

process: ## Full pipeline: validate → parse → TTS → build (make process VIDEO=admin-overview)
ifndef VIDEO
	$(error VIDEO is required. Usage: make process VIDEO=admin-overview [PROJECT=proseforge])
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack video process --project $(PROJECT) $(VIDEO) $(if $(UPLOAD),--upload,) $(if $(TTS_ENGINE),--tts-engine $(TTS_ENGINE),)

process-all: ## Process all videos in input/ (make process-all)
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack video process-all --project $(PROJECT) $(if $(UPLOAD),--upload,) $(if $(TTS_ENGINE),--tts-engine $(TTS_ENGINE),)

narrate-parse: ## Parse narration to JSON (make narrate-parse VIDEO=admin-overview)
ifndef VIDEO
	$(error VIDEO is required. Usage: make narrate-parse VIDEO=admin-overview)
endif
	@mkdir -p $(BUILD_DIR)/$(VIDEO)/audio
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack video parse $(INPUT_DIR)/$(VIDEO)/narration.md > $(BUILD_DIR)/$(VIDEO)/segments.json
	@echo "Parsed narration to $(BUILD_DIR)/$(VIDEO)/segments.json"
	@echo "Segments: $$($(BIN_DIR)/mstack video parse $(BUILD_DIR)/$(VIDEO)/segments.json 2>/dev/null | grep -c '\"index\"' || echo 0)"

narrate-tts: ## Generate TTS audio for all segments (make narrate-tts VIDEO=admin-overview)
ifndef VIDEO
	$(error VIDEO is required. Usage: make narrate-tts VIDEO=admin-overview)
endif
	@mkdir -p $(BUILD_DIR)/$(VIDEO)/audio
	@echo "Generating TTS audio for $(VIDEO) with Google Gemini (voice: $(TTS_VOICE))..."
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@GOOGLE_AI_API_KEY=$(GOOGLE_AI_API_KEY) $(BIN_DIR)/mstack video tts gemini \
		--output-dir $(BUILD_DIR)/$(VIDEO)/audio \
		--voice $(TTS_VOICE) \
		--update-json \
		$(BUILD_DIR)/$(VIDEO)/segments.json

narrate-tts-say: ## Generate TTS audio using macOS say (fallback)
ifndef VIDEO
	$(error VIDEO is required. Usage: make narrate-tts-say VIDEO=admin-overview)
endif
	@mkdir -p $(BUILD_DIR)/$(VIDEO)/audio
	@echo "Generating TTS audio for $(VIDEO) with macOS say (voice: Karen @ 200 wpm)..."
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack video tts say \
		--output-dir $(BUILD_DIR)/$(VIDEO)/audio \
		--voice Karen \
		--rate 200 \
		--update-json \
		$(BUILD_DIR)/$(VIDEO)/segments.json

narrate-build: ## Build final video with FFmpeg (make narrate-build VIDEO=admin-overview)
ifndef VIDEO
	$(error VIDEO is required. Usage: make narrate-build VIDEO=admin-overview)
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@VIDEO_DIR=$$(find $(INPUT_DIR) -type d -name "$(VIDEO).*" | head -1); \
	if [ -z "$$VIDEO_DIR" ]; then echo "Error: Video directory not found for $(VIDEO)"; exit 1; fi; \
	$(BIN_DIR)/mstack video build \
		$(BUILD_DIR)/$(VIDEO)/segments.json \
		"$$VIDEO_DIR/video.webm" \
		$(BUILD_DIR)/$(VIDEO)/$(VIDEO).mp4 \
		--execute
	@echo ""
	@echo "Output: $(BUILD_DIR)/$(VIDEO)/$(VIDEO).mp4"

narrate-all: narrate-parse narrate-tts narrate-build ## Full pipeline: parse, TTS, build (make narrate-all VIDEO=admin-overview)
	@echo ""
	@echo "Video narration complete: $(BUILD_DIR)/$(VIDEO)/$(VIDEO).mp4"

clean-audio: ## Clean generated audio files (make clean-audio VIDEO=admin-overview)
ifndef VIDEO
	$(error VIDEO is required. Usage: make clean-audio VIDEO=admin-overview)
endif
	rm -rf $(BUILD_DIR)/$(VIDEO)/audio
	@echo "Cleaned audio files for $(VIDEO)"

validate: ## Validate video content matches narration using OCR (make validate VIDEO=story-forge)
ifndef VIDEO
	$(error VIDEO is required. Usage: make validate VIDEO=story-forge)
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack video validate --project $(PROJECT) $(VIDEO) $(if $(JSON),--json,)

validate-all: ## Validate all processed videos
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@for video in $$(ls -1 $(BUILD_DIR)/*/segments.json 2>/dev/null | xargs -I{} dirname {} | xargs -I{} basename {}); do \
		echo "=== Validating $$video ==="; \
		$(BIN_DIR)/mstack video validate --project $(PROJECT) $$video || true; \
		echo ""; \
	done

# =============================================================================
# YouTube Publishing
# =============================================================================

YOUTUBE_CHANNEL_ID ?=
YOUTUBE_PLAYLIST_ID ?=

.PHONY: publish youtube-upload youtube-list youtube-delete

publish: narrate-all youtube-upload ## Full pipeline: narrate + upload (make publish VIDEO=admin-overview)
	@echo ""
	@echo "Published $(VIDEO) to YouTube!"

youtube-upload: ## Upload video to YouTube (make youtube-upload VIDEO=admin-overview)
ifndef VIDEO
	$(error VIDEO is required. Usage: make youtube-upload VIDEO=admin-overview)
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@echo "Uploading $(VIDEO) to YouTube..."
	@TITLE=$$(echo '$(VIDEO)' | sed 's/-/ /g' | awk '{for(i=1;i<=NF;i++)sub(/./,toupper(substr($$i,1,1)),$$i)}1'); \
	$(BIN_DIR)/mstack video youtube upload \
		--file="$(BUILD_DIR)/$(VIDEO)/$(VIDEO).mp4" \
		--title="ProseForge: $$TITLE" \
		--description="ProseForge tutorial - $$TITLE. Learn more at proseforge.ai" \
		--privacy=unlisted \
		--playlist=$(YOUTUBE_PLAYLIST_ID) \
		--category=28

youtube-list: ## List uploaded videos
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack video youtube list

youtube-delete: ## Delete a video (make youtube-delete VIDEO_ID=xxx)
ifndef VIDEO_ID
	$(error VIDEO_ID is required. Usage: make youtube-delete VIDEO_ID=xxx)
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	$(BIN_DIR)/mstack video youtube delete $(VIDEO_ID)

# =============================================================================
# Secrets Management (Bitwarden)
# =============================================================================

.PHONY: secrets-push secrets-pull secrets-list secrets-diff

secrets-push: ## Push .env secrets to Bitwarden (make secrets-push PROJECT=proseforge)
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack secrets push --project $(PROJECT) $(if $(DRY_RUN),--dry-run,)

secrets-pull: ## Pull secrets from Bitwarden to .env (make secrets-pull PROJECT=proseforge)
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack secrets pull --project $(PROJECT) $(if $(DRY_RUN),--dry-run,)

secrets-list: ## List secrets in Bitwarden (make secrets-list PROJECT=proseforge)
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack secrets list --project $(PROJECT)

secrets-diff: ## Compare .env vs Bitwarden (make secrets-diff PROJECT=proseforge)
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack secrets diff --project $(PROJECT)

# =============================================================================
# Social Media Management
# =============================================================================

.PHONY: x-show x-update fb-show fb-update ig-show

x-show: ## Show X/Twitter profile
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social x show --project $(PROJECT)

x-update: ## Update X/Twitter profile
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social x update --project $(PROJECT)

fb-show: ## Show Facebook Page info
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social fb show --project $(PROJECT)

fb-update: ## Update Facebook Page info
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social fb update --project $(PROJECT)

ig-show: ## Show Instagram profile info
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social ig show --project $(PROJECT)

# =============================================================================
# Campaign Management
# =============================================================================

CAMPAIGN ?= launch

.PHONY: prelaunch-check campaign-list campaign-post campaign-retract x-post fb-post

prelaunch-check: ## Run pre-launch checklist (make prelaunch-check [CAMPAIGN=launch])
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social check --project $(PROJECT) $(if $(CAMPAIGN),--campaign $(CAMPAIGN),) $(if $(SITE_URL),--site $(SITE_URL),)

campaign-list: ## List posts in a campaign (make campaign-list [CAMPAIGN=launch])
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social campaign list --project $(PROJECT) --campaign $(CAMPAIGN)

campaign-post: ## Post a campaign item by UUID (make campaign-post ID=xxx)
ifndef ID
	$(error ID is required. Usage: make campaign-post ID=uuid [CAMPAIGN=launch])
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social campaign post --project $(PROJECT) --campaign $(CAMPAIGN) $(ID) $(if $(DRY_RUN),--dry-run,)

campaign-retract: ## Retract a campaign item by UUID (make campaign-retract ID=xxx)
ifndef ID
	$(error ID is required. Usage: make campaign-retract ID=uuid [CAMPAIGN=launch])
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social campaign retract --project $(PROJECT) --campaign $(CAMPAIGN) $(ID) $(if $(DRY_RUN),--dry-run,)

x-post: ## Post a tweet (make x-post MSG="Hello world!")
ifndef MSG
	$(error MSG is required. Usage: make x-post MSG="Hello world!")
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social x post --project $(PROJECT) "$(MSG)" $(if $(DRY_RUN),--dry-run,)

fb-post: ## Post to Facebook Page (make fb-post MSG="Hello world!")
ifndef MSG
	$(error MSG is required. Usage: make fb-post MSG="Hello world!")
endif
	@cd $(SRC_DIR) && go build -o $(BIN_DIR)/mstack ./cmd/mstack
	@$(BIN_DIR)/mstack social fb post --project $(PROJECT) "$(MSG)" $(if $(LINK),--link $(LINK),) $(if $(DRY_RUN),--dry-run,)

# =============================================================================
# GitHub Sync (Public Release Mirror)
# =============================================================================

.PHONY: sync-github sync-github-dry sync-check

sync-github: ## Sync public content to GitHub (stage, verify, push)
	scripts/sync-github.sh

sync-github-dry: ## Dry run — stage and verify, but don't push
	scripts/sync-github.sh --dry-run

sync-check: ## Run guardrail checks only (no staging)
	scripts/sync-github.sh --check
