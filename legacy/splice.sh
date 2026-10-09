#!/usr/bin/env bash

# SPLICER FOR TERMO
# Takes gif & video, dumps frames into a folder for termo to use.
#
# Usage:
#   ./splice.sh [options] <file|folder>...
#
# Each input becomes frames/<name>/frame_000001.jpg, frame_000002.jpg, ...
# A folder input splices every media file directly inside it.
#
# Options:
#   -o DIR        Frames root to write into (default: ./frames, then
#                 $frames_path, then frames/ next to this script)
#   -n NAME       Output folder name (single input only; default: file name)
#   -r FPS        Resample to FPS frames per second (default: keep all frames)
#   -w WIDTH      Scale frames to WIDTH px, keeping aspect ratio
#   -q N          JPEG quality 2-31, lower is better (default: 2)
#   -f            Overwrite an existing frames folder
#   -m            Move spliced originals into <their folder>/PROCESSED
#   -h            Show this help

set -euo pipefail

usage() {
    sed -n '3,21p' "$0" | sed 's/^# \{0,1\}//'
    exit "${1:-1}"
}

die() {
    echo "Error: $*" >&2
    exit 1
}

OUT_ROOT=""
NAME_OVERRIDE=""
OUT_FPS=""
OUT_WIDTH=""
QUALITY=2
FORCE=0
MOVE_PROCESSED=0

while getopts ":o:n:r:w:q:fmh" opt; do
    case "$opt" in
        o) OUT_ROOT="$OPTARG" ;;
        n) NAME_OVERRIDE="$OPTARG" ;;
        r) OUT_FPS="$OPTARG" ;;
        w) OUT_WIDTH="$OPTARG" ;;
        q) QUALITY="$OPTARG" ;;
        f) FORCE=1 ;;
        m) MOVE_PROCESSED=1 ;;
        h) usage 0 ;;
        :) die "option -$OPTARG needs a value" ;;
        *) die "unknown option: -$OPTARG (see -h)" ;;
    esac
done
shift $((OPTIND - 1))

[[ $# -gt 0 ]] || usage 1

[[ -z "$OUT_FPS"   || "$OUT_FPS"   =~ ^[0-9]+([./][0-9]+)?$ ]] || die "-r needs a number, got: $OUT_FPS"
[[ -z "$OUT_WIDTH" || "$OUT_WIDTH" =~ ^[1-9][0-9]*$ ]]         || die "-w needs a whole number, got: $OUT_WIDTH"
[[ "$QUALITY" =~ ^[0-9]+$ ]] && (( QUALITY >= 2 && QUALITY <= 31 )) || die "-q must be between 2 and 31"

command -v ffmpeg  >/dev/null || die "ffmpeg not found"
command -v ffprobe >/dev/null || die "ffprobe not found"

# Decide where "frames" live:
# 1) -o DIR
# 2) ./frames in current working dir
# 3) $frames_path (must be an existing dir)
# 4) frames/ next to this script (created if missing)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -n "$OUT_ROOT" ]]; then
    FRAMES_ROOT="$OUT_ROOT"
elif [[ -d "./frames" ]]; then
    FRAMES_ROOT="$PWD/frames"
elif [[ -n "${frames_path:-}" && -d "$frames_path" ]]; then
    FRAMES_ROOT="$frames_path"
else
    FRAMES_ROOT="$SCRIPT_DIR/frames"
fi
mkdir -p "$FRAMES_ROOT"
FRAMES_ROOT="$(cd "$FRAMES_ROOT" && pwd)"
FRAMES_MD="$FRAMES_ROOT/FRAMES.md"

sanitize() {
    local s="$1"
    s="${s// /_}"                                # spaces → _
    s="$(printf '%s' "$s" | tr -cd 'A-Za-z0-9._-')"  # keep only safe chars
    printf '%s' "$s"
}

is_media() {
    local ext="${1##*.}"
    case "${ext,,}" in
        mp4|m4v|mkv|webm|avi|mov|wmv|flv|mpg|mpeg|ts|ogv|3gp|gif|apng|webp) return 0 ;;
        *) return 1 ;;
    esac
}

# "50/3" → "16.667"
frac_to_float() {
    awk -v r="$1" 'BEGIN{n=split(r,a,"/"); if (n==1) a[2]=1; if (a[1]+0==0 || a[2]+0==0) print "N/A"; else printf "%.3f", a[1]/a[2]}'
}

probe() {
    ffprobe -v error -select_streams v:0 -show_entries "stream=$1" -of csv=p=0 "$2" 2>/dev/null | head -n1
}

# Older ffmpeg only knows -vsync, newer only -fps_mode
if [[ "$(ffmpeg -hide_banner -h full 2>/dev/null || true)" == *$'\n'-fps_mode* ]]; then
    PASSTHROUGH=(-fps_mode passthrough)
else
    PASSTHROUGH=(-vsync 0)
fi

TMP_DIR=""
trap '[[ -n "$TMP_DIR" && -d "$TMP_DIR" ]] && rm -rf "$TMP_DIR"' EXIT

splice() {
    local file="$1" name="$2"
    local dest="$FRAMES_ROOT/$name"

    [[ -n "$name" ]] || { echo "Skipping (empty name after sanitizing): $file" >&2; return 1; }

    if [[ -e "$dest" ]]; then
        if (( FORCE == 0 )); then
            echo "Skipping $file: $dest already exists (use -f to overwrite)" >&2
            return 1
        fi
    fi

    [[ -n "$(probe codec_name "$file")" ]] || { echo "Skipping $file: no video stream found" >&2; return 1; }

    local filters=()
    [[ -n "$OUT_FPS"   ]] && filters+=("fps=$OUT_FPS")
    [[ -n "$OUT_WIDTH" ]] && filters+=("scale=$OUT_WIDTH:-2:flags=lanczos")
    local vf=()
    if (( ${#filters[@]} > 0 )); then
        vf=(-vf "$(IFS=,; echo "${filters[*]}")")
    fi

    # Extract into a temp dir first, so a failed run never leaves half a folder
    TMP_DIR="$(mktemp -d "$FRAMES_ROOT/.splice.XXXXXX")"
    if ! ffmpeg -hide_banner -loglevel error -nostdin -y \
            -i "$file" -map 0:v:0 "${vf[@]}" "${PASSTHROUGH[@]}" \
            -q:v "$QUALITY" "$TMP_DIR/frame_%06d.jpg"; then
        rm -rf "$TMP_DIR"; TMP_DIR=""
        echo "Failed: $file" >&2
        return 1
    fi

    local frame_count
    frame_count="$(find "$TMP_DIR" -maxdepth 1 -type f -name 'frame_*.jpg' | wc -l | tr -d ' ')"
    if (( frame_count == 0 )); then
        rm -rf "$TMP_DIR"; TMP_DIR=""
        echo "Failed: no frames extracted from $file" >&2
        return 1
    fi

    # FPS of the frames on disk (avg_frame_rate; r_frame_rate lies for gifs)
    local fps
    if [[ -n "$OUT_FPS" ]]; then
        fps="$(frac_to_float "$OUT_FPS")"
    else
        fps="$(frac_to_float "$(probe avg_frame_rate "$file")")"
        [[ "$fps" != "N/A" ]] || fps="$(frac_to_float "$(probe r_frame_rate "$file")")"
    fi

    local res
    res="$(ffprobe -v error -show_entries frame=width,height -of csv=s=x:p=0 "$TMP_DIR/frame_000001.jpg" 2>/dev/null | head -n1)"
    res="${res/x/ x }"  # "1920x1080" → "1920 x 1080"

    rm -rf "$dest"
    mv "$TMP_DIR" "$dest"
    TMP_DIR=""
    chmod 755 "$dest"

    # Index in FRAMES.md (one line per folder)
    touch "$FRAMES_MD"
    local entries
    entries="$(grep -vF -- "[$name] - " "$FRAMES_MD" || true)"
    { [[ -z "$entries" ]] || printf '%s\n' "$entries"; echo "[$name] - [$fps] - [$frame_count] - [$res]"; } > "$FRAMES_MD"

    if (( MOVE_PROCESSED == 1 )); then
        local processed
        processed="$(dirname "$file")/PROCESSED"
        mkdir -p "$processed"
        mv "$file" "$processed/"
    fi

    echo "Done: $file → $dest/ ($frame_count frames, $fps fps, $res)"
}

# Expand folders into the media files directly inside them
FILES=()
for input in "$@"; do
    if [[ -d "$input" ]]; then
        found=0
        for f in "$input"/*; do
            [[ -f "$f" ]] && is_media "$f" || continue
            FILES+=("$f")
            found=1
        done
        (( found == 1 )) || echo "No media files in folder: $input" >&2
    elif [[ -f "$input" ]]; then
        FILES+=("$input")
    else
        echo "No such file or folder: $input" >&2
    fi
done

(( ${#FILES[@]} > 0 )) || die "nothing to splice"
[[ -z "$NAME_OVERRIDE" || ${#FILES[@]} -eq 1 ]] || die "-n works with a single input, got ${#FILES[@]}"

failed=0
for file in "${FILES[@]}"; do
    if [[ -n "$NAME_OVERRIDE" ]]; then
        name="$(sanitize "$NAME_OVERRIDE")"
    else
        base="$(basename "$file")"
        name="$(sanitize "${base%.*}")"
    fi
    splice "$file" "$name" || failed=1
done

exit "$failed"
