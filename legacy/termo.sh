#!/usr/bin/env bash

# Usage:
#   ./player.sh [options] <folder>
#
# Options:
#   -f FPS        Set frames per second (default: 30)
#   -e            Edges only              (jp2a: --edges-only)
#                 If -e is used without -et, --edge-threshold=0.1 is applied.
#   -et N         Edge threshold N        (jp2a: --edge-threshold=N)
#   -i            Invert brightness       (jp2a: --invert)
#   -c            Enable color            (jp2a: --colors)
#   -nc           Disable color           (default: no --colors)
#   -cd N         Color depth N           (jp2a: --color-depth=N)
#   -C "chars"    Characters palette      (jp2a: --chars=chars)
#   -I            Interactive mode
#   --jp2a "opts" Extra raw jp2a opts, e.g. "--contrast"
#   --            End of options
#
# Interactive keys:
#   1             Toggle color on/off
#   2             Toggle edges-only on/off
#   3             Toggle invert on/off
#   4             Cycle color-depth: none → 4 → 8 → 24 → none
#   5             Toggle border on/off              (--border)
#   6             Toggle flipx on/off               (--flipx)
#   7             Toggle flipy on/off               (--flipy)
#   8             Toggle term-fit on/off            (--term-fit)
#   9             Toggle term-center on/off         (--term-center)
#   0             Toggle term-zoom on/off           (--term-zoom)
#   x             Toggle grayscale on/off           (--grayscale)
#   y             Cycle background: none → dark → light → none  (--background=...)
#   f             Toggle fill on/off                (--fill)
#   r / R         Red   weight −/+ 0.01 (0.00–1.00) (--red)
#   g / G         Green weight −/+ 0.01 (0.00–1.00) (--green)
#   b / B         Blue  weight −/+ 0.01 (0.00–1.00) (--blue)
#   w / W         Width  +/− 2 (start 80)           (--width / --size)
#   h / H         Height +/− 1 (start 24)           (--height / --size)
#   s             Toggle size mode (use --size=WxH if both set)
#   p             Toggle HUD (show/hide bottom controls)
#   ← / →         Decrease / increase FPS (±1, min 1)
#   ↑ / ↓         Decrease / increase edge-threshold (±0.01, 0.00–1.00)
#   q             Quit

usage() {
    echo "Usage: $0 [options] <folder>"
    echo "Example: $0 -I -f 30 -c -e frames"
    exit 1
}

FPS=30
EDGES_ONLY=0
EDGE_THRESHOLD=""   # float, 0.00–1.00
INVERT=0
USE_COLORS=0        # default: no --colors
COLOR_DEPTH=""
CHARS=""
EXTRA_JP2A_OPTS=""
INTERACTIVE=0
STTY_ORIG=""

# Extra jp2a feature flags
BORDER=0
FLIPX=0
FLIPY=0
TERM_FIT=0
TERM_CENTER=0
TERM_ZOOM=0
GRAYSCALE=0
BACKGROUND=""       # "", "dark", "light"
FILL=0

# RGB weights
RED_WEIGHT=""
GREEN_WEIGHT=""
BLUE_WEIGHT=""

# Size / width / height
WIDTH=""
HEIGHT=""
USE_SIZE=0          # 0: use width/height; 1: use size=WxH if both set

# HUD visibility
SHOW_HUD=1

# ---- Parse options ----
while [[ $# -gt 0 ]]; do
    case "$1" in
        -f)
            [[ -n "$2" ]] || usage
            FPS="$2"
            shift 2
            ;;
        -e)
            EDGES_ONLY=1
            shift
            ;;
        -et)
            [[ -n "$2" ]] || usage
            EDGE_THRESHOLD="$2"
            shift 2
            ;;
        -i)
            INVERT=1
            shift
            ;;
        -c)
            USE_COLORS=1
            shift
            ;;
        -nc)
            USE_COLORS=0
            shift
            ;;
        -cd)
            [[ -n "$2" ]] || usage
            COLOR_DEPTH="$2"
            shift 2
            ;;
        -C)
            [[ -n "$2" ]] || usage
            CHARS="$2"
            shift 2
            ;;
        -I)
            INTERACTIVE=1
            shift
            ;;
        --jp2a)
            [[ -n "$2" ]] || usage
            EXTRA_JP2A_OPTS+=" $2"
            shift 2
            ;;
        --)
            shift
            break
            ;;
        -*)
            echo "Unknown option: $1"
            usage
            ;;
        *)
            break
            ;;
    esac
done

DIR="$1"
[[ -z "$DIR" ]] && usage
[[ -d "$DIR" ]] || { echo "No such directory: $DIR"; exit 1; }
shift  # drop DIR

# ---- Float helpers ----
inc_edge_threshold() {
    local step="$1"
    local v="${EDGE_THRESHOLD:-0}"
    EDGE_THRESHOLD=$(awk -v v="$v" -v s="$step" 'BEGIN {
        v += s;
        if (v < 0) v = 0;
        if (v > 1) v = 1;
        printf "%.2f", v;
    }')
}

adjust_weight() {
    local name="$1"
    local step="$2"
    local v="${!name:-0}"
    v=$(awk -v v="$v" -v s="$step" 'BEGIN {
        v += s;
        if (v < 0) v = 0;
        if (v > 1) v = 1;
        printf "%.2f", v;
    }')
    printf -v "$name" '%s' "$v"
}

# ---- Build jp2a args from current flags ----
build_jp2a_args() {
    JP2A_ARGS=()

    if (( EDGES_ONLY == 1 )); then
        JP2A_ARGS+=(--edges-only)
        if [[ -n "$EDGE_THRESHOLD" ]]; then
            JP2A_ARGS+=(--edge-threshold="$EDGE_THRESHOLD")
        else
            JP2A_ARGS+=(--edge-threshold=0.10)
        fi
    elif [[ -n "$EDGE_THRESHOLD" ]]; then
        JP2A_ARGS+=(--edge-threshold="$EDGE_THRESHOLD")
    fi

    (( INVERT == 1 ))     && JP2A_ARGS+=(--invert)
    (( USE_COLORS == 1 )) && JP2A_ARGS+=(--colors)
    [[ -n "$COLOR_DEPTH" ]] && JP2A_ARGS+=(--color-depth="$COLOR_DEPTH")
    [[ -n "$CHARS" ]]       && JP2A_ARGS+=(--chars="$CHARS")

    (( BORDER == 1 ))       && JP2A_ARGS+=(--border)
    (( FLIPX == 1 ))        && JP2A_ARGS+=(--flipx)
    (( FLIPY == 1 ))        && JP2A_ARGS+=(--flipy)
    (( TERM_FIT == 1 ))     && JP2A_ARGS+=(--term-fit)
    (( TERM_CENTER == 1 ))  && JP2A_ARGS+=(--term-center)
    (( TERM_ZOOM == 1 ))    && JP2A_ARGS+=(--term-zoom)
    (( GRAYSCALE == 1 ))    && JP2A_ARGS+=(--grayscale)
    if [[ -n "$BACKGROUND" ]]; then
        JP2A_ARGS+=(--background="$BACKGROUND")
    fi
    (( FILL == 1 ))         && JP2A_ARGS+=(--fill)

    [[ -n "$RED_WEIGHT"   ]] && JP2A_ARGS+=(--red="$RED_WEIGHT")
    [[ -n "$GREEN_WEIGHT" ]] && JP2A_ARGS+=(--green="$GREEN_WEIGHT")
    [[ -n "$BLUE_WEIGHT"  ]] && JP2A_ARGS+=(--blue="$BLUE_WEIGHT")

    if (( USE_SIZE == 1 )) && [[ -n "$WIDTH" ]] && [[ -n "$HEIGHT" ]]; then
        JP2A_ARGS+=(--size="${WIDTH}x${HEIGHT}")
    else
        [[ -n "$WIDTH"  ]] && JP2A_ARGS+=(--width="$WIDTH")
        [[ -n "$HEIGHT" ]] && JP2A_ARGS+=(--height="$HEIGHT")
    fi

    if [[ -n "$EXTRA_JP2A_OPTS" ]]; then
        # shellcheck disable=SC2206
        local extra=($EXTRA_JP2A_OPTS)
        JP2A_ARGS+=("${extra[@]}")
    fi
}

# ---- Cleanup ----
cleanup() {
    printf '\033[?25h'
    if (( INTERACTIVE == 1 )) && [[ -n "$STTY_ORIG" ]]; then
        stty "$STTY_ORIG"
    fi
    exit "${1:-0}"
}

# Pre-expand file list
set -- "$DIR"/*
[[ "$1" == "$DIR/*" ]] && { echo "No files in $DIR"; cleanup 1; }

INTERVAL=$(awk "BEGIN {print 1/$FPS}")

# Hide cursor
printf '\033[?25l'

if (( INTERACTIVE == 1 )); then
    STTY_ORIG=$(stty -g)
    stty -echo
fi

trap 'cleanup 0' INT TERM

# ---- Key handling ----
handle_keys() {
    (( INTERACTIVE == 1 )) || return

    while IFS= read -rsn1 -t 0.01 key; do
        case "$key" in
            1) (( USE_COLORS = 1 - USE_COLORS )) ;;
            2)
                if (( EDGES_ONLY == 1 )); then
                    EDGES_ONLY=0
                else
                    EDGES_ONLY=1
                    [[ -z "$EDGE_THRESHOLD" ]] && EDGE_THRESHOLD=0.10
                fi
                ;;
            3) (( INVERT = 1 - INVERT )) ;;
            4)
                case "$COLOR_DEPTH" in
                    "")   COLOR_DEPTH=4 ;;
                    4)    COLOR_DEPTH=8 ;;
                    8)    COLOR_DEPTH=24 ;;
                    24)   COLOR_DEPTH="" ;;
                    *)    COLOR_DEPTH="" ;;
                esac
                ;;
            5) (( BORDER      = 1 - BORDER      )) ;;
            6) (( FLIPX       = 1 - FLIPX       )) ;;
            7) (( FLIPY       = 1 - FLIPY       )) ;;
            8) (( TERM_FIT    = 1 - TERM_FIT    )) ;;
            9) (( TERM_CENTER = 1 - TERM_CENTER )) ;;
            0) (( TERM_ZOOM   = 1 - TERM_ZOOM   )) ;;
            x|X) (( GRAYSCALE = 1 - GRAYSCALE )) ;;
            y|Y)
                case "$BACKGROUND" in
                    "")     BACKGROUND="dark" ;;
                    dark)   BACKGROUND="light" ;;
                    light*) BACKGROUND="" ;;
                    *)      BACKGROUND="" ;;
                esac
                ;;
            f|F) (( FILL = 1 - FILL )) ;;
            p|P)
                if (( SHOW_HUD == 1 )); then
                    SHOW_HUD=0
                else
                    SHOW_HUD=1
                fi
                ;;
            r) adjust_weight RED_WEIGHT   -0.01 ;;
            R) adjust_weight RED_WEIGHT    0.01 ;;
            g) adjust_weight GREEN_WEIGHT -0.01 ;;
            G) adjust_weight GREEN_WEIGHT  0.01 ;;
            b) adjust_weight BLUE_WEIGHT  -0.01 ;;
            B) adjust_weight BLUE_WEIGHT   0.01 ;;
            w)
                [[ -z "$WIDTH" ]] && WIDTH=80
                WIDTH=$((WIDTH + 2))
                (( WIDTH < 4 )) && WIDTH=4
                ;;
            W)
                [[ -z "$WIDTH" ]] && WIDTH=80
                WIDTH=$((WIDTH - 2))
                (( WIDTH < 4 )) && WIDTH=4
                ;;
            h)
                [[ -z "$HEIGHT" ]] && HEIGHT=24
                HEIGHT=$((HEIGHT + 1))
                (( HEIGHT < 4 )) && HEIGHT=4
                ;;
            H)
                [[ -z "$HEIGHT" ]] && HEIGHT=24
                HEIGHT=$((HEIGHT - 1))
                (( HEIGHT < 4 )) && HEIGHT=4
                ;;
            s|S)
                if (( USE_SIZE == 1 )); then
                    USE_SIZE=0
                else
                    USE_SIZE=1
                fi
                ;;
            q|Q)
                cleanup 0
                ;;
            $'\033')
                if IFS= read -rsn2 -t 0.01 seq; then
                    case "$seq" in
                        "[C") # right → FPS up
                            FPS=$((FPS + 1))
                            (( FPS < 1 )) && FPS=1
                            INTERVAL=$(awk "BEGIN {print 1/$FPS}")
                            ;;
                        "[D") # left → FPS down
                            FPS=$((FPS - 1))
                            (( FPS < 1 )) && FPS=1
                            INTERVAL=$(awk "BEGIN {print 1/$FPS}")
                            ;;
                        "[A") # up → edge-threshold up
                            [[ -z "$EDGE_THRESHOLD" ]] && EDGE_THRESHOLD=0.10
                            inc_edge_threshold 0.01
                            ;;
                        "[B") # down → edge-threshold down
                            [[ -z "$EDGE_THRESHOLD" ]] && EDGE_THRESHOLD=0.00
                            inc_edge_threshold -0.01
                            ;;
                    esac
                fi
                ;;
        esac
    done
}

# ---- Main loop ----
while :; do
    for img; do
        [[ -f "$img" ]] || continue

        handle_keys
        build_jp2a_args

        # Clear screen & move cursor home
        printf '\033[H\033[2J'

        jp2a "${JP2A_ARGS[@]}" "$img"

        if (( INTERACTIVE == 1 )) && (( SHOW_HUD == 1 )); then
            printf '\n1:color=%s  2:edges=%s(th=%s)  3:invert=%s  4:depth=%s  ←/→ FPS=%s  ↑/↓ edge-th  q:quit  p:HUD=%s\n' \
                "$([[ $USE_COLORS   -eq 1 ]] && echo on || echo off)" \
                "$([[ $EDGES_ONLY   -eq 1 ]] && echo on || echo off)" \
                "${EDGE_THRESHOLD:-none}" \
                "$([[ $INVERT       -eq 1 ]] && echo on || echo off)" \
                "${COLOR_DEPTH:-none}" \
                "$FPS" \
                "$([[ $SHOW_HUD     -eq 1 ]] && echo on || echo off)"

            printf '5:border=%s  6:flipx=%s  7:flipy=%s  8:fit=%s  9:center=%s  0:zoom=%s  x:gray=%s  y:bg=%s  f:fill=%s\n' \
                "$([[ $BORDER      -eq 1 ]] && echo on || echo off)" \
                "$([[ $FLIPX       -eq 1 ]] && echo on || echo off)" \
                "$([[ $FLIPY       -eq 1 ]] && echo on || echo off)" \
                "$([[ $TERM_FIT    -eq 1 ]] && echo on || echo off)" \
                "$([[ $TERM_CENTER -eq 1 ]] && echo on || echo off)" \
                "$([[ $TERM_ZOOM   -eq 1 ]] && echo on || echo off)" \
                "$([[ $GRAYSCALE   -eq 1 ]] && echo on || echo off)" \
                "${BACKGROUND:-none}" \
                "$([[ $FILL        -eq 1 ]] && echo on || echo off)"

            printf 'r/R:red=%s  g/G:green=%s  b/B:blue=%s  w/W:width=%s  h/H:height=%s  s:size=%s\n' \
                "${RED_WEIGHT:-auto}" \
                "${GREEN_WEIGHT:-auto}" \
                "${BLUE_WEIGHT:-auto}" \
                "${WIDTH:-auto}" \
                "${HEIGHT:-auto}" \
                "$([[ $USE_SIZE -eq 1 ]] && echo on || echo off)"
        fi

        sleep "$INTERVAL"
    done
done
