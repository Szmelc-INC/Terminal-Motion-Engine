#!/usr/bin/env bash

# SPLICER FOR TERMO
# Takes gif & video, dumps frames into a folder for termo to use.

set -e

INPUT_DIR="$1"

if [[ -z "$INPUT_DIR" || ! -d "$INPUT_DIR" ]]; then
    echo "Usage: $0 <folder_with_media>"
    exit 1
fi

# Decide where "frames" live:
# 1) Prefer ./frames in current working dir
# 2) Otherwise use $frames_path (must be an existing dir)
if [[ -d "./frames" ]]; then
    FRAMES_ROOT="$PWD/frames"
elif [[ -n "${frames_path:-}" && -d "$frames_path" ]]; then
    FRAMES_ROOT="$frames_path"
else
    echo "Error: frames directory not found."
    echo "Create ./frames or export frames_path=/path/to/frames"
    exit 1
fi

FRAMES_MD="$FRAMES_ROOT/FRAMES.md"
# Ensure FRAMES.md exists
if [[ ! -f "$FRAMES_MD" ]]; then
    touch "$FRAMES_MD"
fi

PROCESSED_DIR="$INPUT_DIR/PROCESSED"
mkdir -p "$PROCESSED_DIR"

sanitize() {
    local s="$1"
    s="${s// /_}"                             # spaces → _
    s="$(echo "$s" | tr -cd 'A-Za-z0-9._-')"  # keep only safe chars
    echo "$s"
}

for FILE in "$INPUT_DIR"/*; do
    [[ -f "$FILE" ]] || continue

    # Only process common video/image formats (ffmpeg will handle them)
    ext="${FILE##*.}"
    shopt -s nocasematch
    case "$ext" in
        mp4|mkv|webm|avi|mov|wmv|flv|mpg|mpeg|gif)
            ;;
        *)
            shopt -u nocasematch
            echo "Skipping non-media file: $FILE"
            continue
            ;;
    esac
    shopt -u nocasematch

    ORIGINAL_BASENAME="$(basename "$FILE")"
    SANITIZED_BASENAME="$(sanitize "$ORIGINAL_BASENAME")"

    if [[ "$SANITIZED_BASENAME" != "$ORIGINAL_BASENAME" ]]; then
        mv "$FILE" "$INPUT_DIR/$SANITIZED_BASENAME"
        FILE="$INPUT_DIR/$SANITIZED_BASENAME"
    fi

    NAME="${SANITIZED_BASENAME%.*}"
    SRC_FRAMES_DIR="$INPUT_DIR/$NAME"

    mkdir -p "$SRC_FRAMES_DIR"

    # Extract frames
    ffmpeg -hide_banner -loglevel error \
        -i "$FILE" -vsync 0 "$SRC_FRAMES_DIR/frame_%06d.jpg"

    # Move original media to PROCESSED
    mv "$FILE" "$PROCESSED_DIR/"

    # Analyze FPS (convert fraction to float)
    RAW_FPS="$(ffprobe -v 0 -of csv=p=0 -select_streams v:0 -show_entries stream=r_frame_rate "$PROCESSED_DIR/$SANITIZED_BASENAME" 2>/dev/null || echo "0/0")"
    if [[ "$RAW_FPS" == */* ]]; then
        FPS="$(awk -v r="$RAW_FPS" 'BEGIN{split(r,a,"/"); if (a[2]==0) {print "N/A"} else printf "%.3f", a[1]/a[2]}')"
    else
        FPS="$RAW_FPS"
    fi

    # Analyze resolution
    RAW_RES="$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=s=x:p=0 "$PROCESSED_DIR/$SANITIZED_BASENAME" 2>/dev/null || echo "x")"
    RES="${RAW_RES/x/ x }"  # "1920x1080" → "1920 x 1080"

    # Count frames
    FRAME_COUNT="$(find "$SRC_FRAMES_DIR" -maxdepth 1 -type f -name 'frame_*.jpg' | wc -l | tr -d ' ')"

    # Move frames folder into frames root
    DEST_FRAMES_DIR="$FRAMES_ROOT/$NAME"
    mv "$SRC_FRAMES_DIR" "$DEST_FRAMES_DIR"

    # Append info to FRAMES.md
    echo "[$NAME] - [$FPS] - [$FRAME_COUNT] - [$RES]" >> "$FRAMES_MD"

    echo "Done: $SANITIZED_BASENAME → $DEST_FRAMES_DIR/ → moved to PROCESSED/ and indexed in FRAMES.md"
done
