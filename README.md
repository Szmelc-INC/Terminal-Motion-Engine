<div align="center">

<picture>
    <source media="(prefers-color-scheme: dark)"  srcset="https://raw.githubusercontent.com/serainox420/serainox420/personal/vector/logo-white.png">
    <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/serainox420/serainox420/personal/vector/logo-black.png">
    <img src="https://i.imgur.com/qpgeyna.png" alt="termo" width="124" />

</picture><h1><samp><b>𝙩𝙚𝙧𝙢𝙤</b></samp></h1>

<table><tr><td>𝚃𝚎𝚛𝚖𝚒𝚗𝚊𝚕 𝙼𝚘𝚝𝚒𝚘𝚗 𝙴𝚗𝚐𝚒𝚗𝚎 · 𝟸.𝟶</td></tr></table>
    <i><samp><h4>𝙿𝚕𝚊𝚢 𝙶𝙸𝙵𝚜 𝚊𝚗𝚍 𝚟𝚒𝚍𝚎𝚘𝚜 𝚒𝚗 𝚝𝚑𝚎 𝚝𝚎𝚛𝚖𝚒𝚗𝚊𝚕 𝚊𝚜 𝙰𝚂𝙲𝙸𝙸 / 𝙰𝙽𝚂𝙸 𝚊𝚛𝚝, with sound, at the source frame rate, without flicker.</h4></samp></i>

 <nav align="center">
  <!-- Row 1: stars · forks · contributors · pull requests · last commit -->
  <a href="https://github.com/Szmelc-INC/Terminal-Motion-Engine/stargazers">
    <img src="https://img.shields.io/github/stars/Szmelc-INC/Terminal-Motion-Engine?style=flat&color=gold" alt="stars">
  </a>
  <a href="https://github.com/Szmelc-INC/Terminal-Motion-Engine/network/members">
    <img src="https://img.shields.io/github/forks/Szmelc-INC/Terminal-Motion-Engine?style=flat&color=81C459" alt="forks">
  </a>
  <a href="https://github.com/Szmelc-INC/Terminal-Motion-Engine/graphs/contributors">
    <img src="https://img.shields.io/github/contributors-anon/Szmelc-INC/Terminal-Motion-Engine?style=flat&color=1E7B85" alt="contributors">
  </a>
  <a href="https://github.com/Szmelc-INC/Terminal-Motion-Engine/pulls">
    <img src="https://img.shields.io/github/issues-pr/Szmelc-INC/Terminal-Motion-Engine?style=flat&color=FF570A" alt="pull requests">
  </a>
  <a href="https://github.com/Szmelc-INC/Terminal-Motion-Engine/commits">
    <img src="https://img.shields.io/github/last-commit/Szmelc-INC/Terminal-Motion-Engine?style=flat" alt="last commit">
  </a>
  <br>
  <!-- Row 2: language · license · discord · youtube · website -->
  <a href="https://go.dev">
    <img src="https://img.shields.io/github/go-mod/go-version/Szmelc-INC/Terminal-Motion-Engine?style=flat&logo=go&logoColor=white" alt="go version">
  </a>
  <a href="https://github.com/Szmelc-INC/Terminal-Motion-Engine/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/Szmelc-INC/Terminal-Motion-Engine?style=flat" alt="license">
  </a>
  <a href="https://discord.gg/xSYSqufd">
    <img src="https://img.shields.io/badge/discord-7289da.svg?style=flat&logo=discord&logoColor=white" alt="discord">
  </a>
  <a href="https://youtube.com/@LinuxMajster">
    <img src="https://img.shields.io/badge/youtube-darkred?style=flat&logo=youtube&logoColor=white" alt="youtube">
  </a>
  <a href="https://szmelc.com">
    <img src="https://img.shields.io/badge/website-szmelc.com-purple?style=flat&labelColor=darkcyan&color=104C35" alt="website">
  </a>
 </nav>
</div>

---

> [!TIP]
> # Demo 🎬
> <details>
> <summary><b>Gif / Asciimation</b></summary>
>
> Recorded with termo 1.x. Version 2.0 adds sound, color palettes, dithering and a full TUI on top of this look.
>
> <p align="left">
>   <img src="https://github.com/user-attachments/assets/faef350c-63e8-4ebc-bdfb-ab74071356b0" alt="termo demo" width="680">
> </p>
>
> </details>
> <details>
> <summary><b>Music video made with termo</b></summary>
>
> <p align="left">
>   <a href="https://www.youtube.com/watch?v=KoaDMKpmaZo">
>     <img src="https://github.com/user-attachments/assets/705c763f-06e7-492b-85a0-445b80d48f7a" alt="music video made with termo" width="680">
>   </a>
> </p>
>
> Watch it on YouTube: https://www.youtube.com/watch?v=KoaDMKpmaZo
>
> </details>

> [!IMPORTANT]
> # Quick Install
> Clone
> ```sh
> git clone https://github.com/Szmelc-INC/Terminal-Motion-Engine.git && cd Terminal-Motion-Engine
> ```
> Build, install into `~/.local/bin/termo` and clean build artifacts
> ```sh
> make install && make clean
> ```
> Or let Go fetch and build it in one step (installs into `~/go/bin`)
> ```sh
> go install github.com/Szmelc-INC/Terminal-Motion-Engine/cmd/termo@latest
> ```
> Then play something
> ```sh
> termo video.mp4
> ```

---

> [!NOTE]
> # ***Info***
>
> `termo` 2.0 is a single Go binary. It replaces the `jp2a` + bash scripts of 1.x, which are kept in [`legacy/`](legacy/).
> You no longer split a video into a folder of frames first, and the screen is no longer cleared between frames.
>
> <details>
> <summary><b>✨ What's new in 2.0</b></summary>
>
> | Feature | Details |
> |---|---|
> | **Plays files directly** | Any GIF, video or picture that ffmpeg can read. Several files make a playlist. |
> | **Audio** | The sound track plays in sync, through PulseAudio, PipeWire, ALSA or ffplay. |
> | **Smooth output** | Only the cells that changed are written, in one write per frame, inside a synchronized update. No `clear`, no tearing, no blinking. |
> | **Fast** | Turning a 1080p60 frame into a 240×67 grid takes 0.2–5 ms depending on mode and dither, against a 16.7 ms budget. `termo bench` measures it on your machine. |
> | **5 render modes** | Half blocks, quadrants, sextants, braille and ASCII (9 character ramps, or your own). |
> | **Palettes** | Truecolor, 1–256 colors, 21 named palettes, 11 color-theory schemes, adaptive palettes computed from the picture, and custom ones. |
> | **18 dither algorithms** | Bayer 2/4/8, blue noise, white noise, film grain, halftone, line screens, Floyd–Steinberg, Atkinson, JJN, Stucki, Burkes and three Sierra kernels. |
> | **Randomize** | One key rolls a whole new look, another rolls only the palette, and undo brings the last one back. |
> | **Presets** | Save, load, edit, rename and delete looks, from the menu or from the command line. 13 presets are built in. |
> | **A real TUI** | HUD, settings popup, palette editor and file browser. Everything works with the keyboard and with the mouse. |
>
> </details>
> <details>
> <summary><b>🔀 Migrating from 1.x</b></summary>
>
> - `termo.sh` and `splice.sh` moved from the repository root to [`legacy/`](legacy/). Scripts that call them by their old path need the new one.
> - `jp2a` is no longer needed. `ffmpeg` and `ffprobe` are needed at runtime instead.
> - Splitting with `splice.sh` is no longer needed: `termo clip.mp4` plays the file directly.
> - Frame folders made for 1.x still play. `termo frames/gun` plays a folder of numbered pictures at 30 fps.
>
> </details>
> <details>
> <summary><b>📦 Packaging</b></summary>
>
> | Status | Source | How |
> |---|---|---|
> | ✅ | Source build | `make install` |
> | ✅ | Go toolchain | `go install github.com/Szmelc-INC/Terminal-Motion-Engine/cmd/termo@latest` |
> | ❌ | Arch / Endeavour / Manjaro | **AUR** (PKGBUILD) |
> | ❌ | Debian / Ubuntu / Mint | **.deb** package |
> | ❌ | Fedora / RHEL family | **COPR** |
> | ❌ | NixOS / any Linux with Nix | **nixpkgs** |
> | ❌ | macOS + Linux | **Homebrew** |
> | ❌ | Prebuilt binaries | **GitHub Releases** |
>
> </details>
> <details>
> <summary><b>🤝 Contribute</b></summary>
>
> ## Looking for a project to contribute to?
> Help with packaging termo for the platforms above is welcome, and so are bug reports and pull requests. Reach out on Discord or by email. \
> [![Email](https://img.shields.io/badge/Mail-EA4335?style=flat&logo=gmail&logoColor=white&labelColor=darkred&color=silver)](mailto:serainox@gmail.com) [![Discord](https://img.shields.io/badge/Sernik-002333?style=flat&logo=discord&logoColor=00FF84)](https://discord.com/users/818166724641030193)
>
> </details>

---

> [!TIP]
> Details on usage and available options
> # Usage
> ```sh
> termo [options] [file...]         # play files or folders of frames (no file: file browser)
> termo snap  [options] <file>      # print a single frame (--at SEC, --size COLSxROWS)
> termo bench [options] <file>      # measure render speed (--frames N, --size, --all)
> termo info  <file>                # show stream information
> termo presets [list|show|save|edit|rename|rm|path] [name]
> termo palettes                    # list palettes and color schemes
> termo options                     # list every look option with its values
> ```
> ### Examples
> ```sh
> termo clip.mp4                       # play
> termo                                # no file: open the file browser
> termo a.gif b.mp4 c.webm             # playlist (n / N to switch)
> termo frames/gun                     # a folder of numbered pictures, 30 fps
> termo -m braille -p matrix clip.mp4  # pick a look up front
> termo -P gameboy clip.mp4            # start from a preset
> termo -r clip.mp4                    # start with a random look
> termo -p harmony --scheme triadic --hue 20 -c 6 -d halftone clip.mp4
> termo --custom '#1a1c2c,#5d275d,#ef7d57,#f4f4f4' -d atkinson clip.mp4
>
> termo snap --at 12.5 --size 120x40 clip.mp4 > frame.ans   # one frame as ANSI text
> termo bench --all clip.mp4           # how fast is every mode on this machine
> ```
> ### Flags
> | Flag | Description |
> |------|-------------|
> | `-m`, `--mode <mode>` | Glyph set: `half`, `quad`, `sextant`, `braille` or `ascii` |
> | `-p`, `--palette <name>` | `truecolor`, a generated palette or a named one |
> | `-c`, `--colors <n>` | Palette size, 1–256 |
> | `--scheme <name>` | Color-theory scheme for `--palette harmony` |
> | `-d`, `--dither <name>` | Dither algorithm |
> | `-P`, `--preset <name>` | Start from a preset |
> | `-r`, `--random` | Start with a randomized look (`--seed <n>` repeats it) |
> | `-f`, `--fps <n>` | Cap the frame rate (default: the source rate) |
> | `-s`, `--start <sec>` | Start position |
> | `--loop` / `--no-loop` | Loop playback (default: on for clips without sound) |
> | `--volume <v>` · `--mute` · `--no-audio` | Sound level, 0–1.5 · start muted · do not play sound |
> | `--speed <x>` | Playback speed, 0.25–4 |
> | `--audio-delay <sec>` | Delay the sound by −1 to 1 s to fix lip-sync |
> | `--audio-sink <command>` | Shell command that plays raw s16le 48 kHz stereo PCM from stdin, instead of the detected player |
> | `--hud auto\|on\|off` · `--stats` | HUD visibility · performance statistics |
> | `--depth 24\|8\|4` | Force the terminal color depth |
> | `--hwaccel` | Hardware-accelerated decoding |
>
> Every setting in the menu is also a flag. Run `termo options` for the full list.
>
> ### Hotkeys (at runtime)
> | Action | Key(s) |
> |---|---|
> | Play / pause | `Space` / `k` |
> | Seek 5 s · 30 s | `←` `→` · `Shift`+`←` `→` |
> | Previous / next frame | `,` `.` |
> | Jump to 0–90 % · start | `0`–`9` · `Home` |
> | Volume · mute | `↑` `↓` · `m` |
> | Speed · loop | `[` `]` · `l` |
> | **Randomize the whole look** | **`r`** |
> | Randomize the palette only | `R` |
> | Undo the last change | `u` |
> | **Settings menu** | **`Tab`** / `s` / `F2` |
> | Next / previous preset | `p` / `P` |
> | Save the current look as a preset | `Ctrl+S` |
> | Cycle mode / dither / palette / charset (`Shift` = back) | `v` `d` `c` `a` |
> | Fewer / more palette colors | `-` `=` |
> | Fit · flip X · flip Y · edges · invert | `f` `x` `y` `e` `i` |
> | HUD auto/on/off · performance stats | `h` · `g` |
> | Open file · next / previous file | `o` · `n` `N` |
> | Redraw the screen | `Ctrl+L` |
> | Help | `?` / `F1` |
> | Quit | `q` / `Ctrl+C` |
>
> ### Mouse
> | Action | Gesture |
> |---|---|
> | Play / pause | Click the picture |
> | Settings menu | Right-click |
> | Randomize the look | Middle-click |
> | Volume | Wheel |
> | Seek, change a slider | Drag the seek bar or the slider |
> | Cycle mode / dither / palette | Click or scroll the HUD buttons |
> | Move the settings window | Drag its title bar |

---

> [!IMPORTANT]
> # Compiling
> You need **Go 1.26 or newer** to build. There are no C dependencies.
> ## Build
> With Makefile:
> ```sh
> #   make                 # build ./termo
> #   make install         # build and install to ~/.local/bin/termo  (override PREFIX)
> #   make test            # go vet + unit tests
> #   make bench           # render benchmarks
> #   make clean           # remove ./termo
> ```
> For a system-wide install, set `PREFIX`:
> ```sh
> sudo make install PREFIX=/usr/local
> ```
> Or manually:
> ```sh
> go build -trimpath -ldflags "-s -w" -o termo ./cmd/termo
> ```
> ## Layout
> | Path | Contents |
> |---|---|
> | `cmd/termo` | Command-line entry point |
> | `internal/engine` | Color, palettes, dithering, glyph composition |
> | `internal/tty` | Diffing cell screen, key and mouse decoder |
> | `internal/media` | ffmpeg video and audio pipelines |
> | `internal/app` | Player, HUD, menus, presets |
> | `legacy/` | The termo 1.x bash scripts |
> | `frames/` | Sample frame folder in the 1.x format |

---

> [!NOTE]
> ## Looks: modes, palettes, dithering, presets
>
> ### Render modes (`--mode`)
> | Mode | Glyphs | Sub-cells per character |
> |---|---|---|
> | `half` | Half blocks | 1×2 |
> | `quad` | Quadrants | 2×2 |
> | `sextant` | Sextants | 2×3 |
> | `braille` | Braille dots | 2×4 |
> | `ascii` | A brightness ramp (`--charset`) | 1×1 |
>
> ASCII ramps: `standard`, `detailed`, `minimal`, `blocks`, `dots`, `lines`, `slashes`, `binary`, `katakana`, or `custom` with `--chars 'TEXT'` (darkest character first).
>
> ### Palettes (`--palette`)
> | Kind | Values |
> |---|---|
> | Full color | `truecolor` |
> | Generated | `harmony` (color theory), `adaptive` (median cut of the picture), `gray`, `cube`, `ansi16`, `xterm256` |
> | Named | `1bit` `paper` `gameboy` `cga` `cga-hot` `amber` `matrix` `sepia` `ice` `sunset` `vaporwave` `cyberpunk` `pico8` `c64` `zx` `apple2` `nord` `gruvbox` `dracula` `solarized` `catppuccin` |
> | Custom | `custom` with `--custom '#1a1c2c,#f4f4f4,#ef7d57'` |
>
> Harmony schemes (`--scheme`): `monochromatic`, `analogous`, `complementary`, `split-complementary`, `triadic`, `tetradic`, `square`, `hexadic`, `hue-shift`, `golden`, `rainbow`.
> Tune them with `--hue`, `--chroma`, `--lmin`, `--lmax` and `--colors`. Run `termo palettes` to see every palette with its colors.
>
> ### Dithering (`--dither`)
> | Family | Algorithms | On video |
> |---|---|---|
> | Ordered | `bayer2` `bayer4` `bayer8` `bluenoise` `whitenoise` `grain` `halftone` `hlines` `vlines` `diagonal` | Fixed to the screen, so the pattern stays still |
> | Error diffusion | `floyd-steinberg` `atkinson` `jjn` `stucki` `burkes` `sierra` `sierra2` `sierra-lite` | Good on stills, can shimmer in motion |
>
> `--dither-amount` sets the strength (0–2). `--serpentine` alternates the scan direction for error diffusion.
>
> ### Picture adjustments
> `--brightness`, `--contrast`, `--gamma`, `--saturation`, `--hue-shift`, `--invert`, `--edges off|mono|color` (Sobel edge detection), `--fit fit|fill|stretch`, `--flip-x`, `--flip-y`.
>
> ### Presets
> A preset stores a whole look. On Linux, presets live in `~/.config/termo/presets.json` (`$XDG_CONFIG_HOME` is honoured). `termo presets path` prints the location on your system. A preset that you name `default` is loaded at start-up.
> ```sh
> termo presets list
> termo presets show NAME
> termo presets save NAME [look options]
> termo presets edit NAME [look options]
> termo presets rename OLD NEW
> termo presets rm NAME
> termo presets path
> ```
> Built-in presets: `default`, `sextant-hd`, `ascii-classic`, `ascii-color`, `braille-mono`, `gameboy`, `newsprint`, `matrix`, `amber-crt`, `cga`, `vaporwave`, `blueprint`, `pop-art`.
>
> ### Settings menu
> `Tab` opens a popup with seven tabs. Switch tabs with `Tab`, with `1`–`7`, or by clicking.
> | Tab | What you do there |
> |---|---|
> | Render / Color / Dither / Adjust / Playback | Every option. `↑` `↓` select, `←` `→` change (`Shift` for bigger steps), `Enter` types a value. |
> | Presets | `Enter` load · `n` new from the current look · `s` save over · `e` rename · `d` delete |
> | Palette | The colors in use. `t` turns any palette into an editable custom one, then `a` adds, `Enter` edits and `d` deletes a color. |
>
> To edit a preset: load it, change what you like, then save over it.

---

> [!WARNING]
> ## Logos / Banners
> <details><summary><b>Images / Logos</b></summary>
>
> Banners <br>
> <img src="https://raw.githubusercontent.com/serainox420/serainox420/refs/heads/personal/vector/banner-black2.svg" alt="banner, black" width="280">
> <br><br>
> <img src="https://raw.githubusercontent.com/serainox420/serainox420/refs/heads/personal/vector/banner-white2.svg" alt="banner, white" width="280">
> <br><br>
> Logos <br>
> <img src="https://raw.githubusercontent.com/serainox420/serainox420/refs/heads/personal/vector/logo-white.svg" alt="logo, white" width="80">
> <br><br>
> <img src="https://raw.githubusercontent.com/serainox420/serainox420/refs/heads/personal/vector/logo-black.svg" alt="logo, black" width="80">
>
> </details>

---

> [!CAUTION]
> DETAILS
> ## Requirements
>
> - Linux or macOS.
> - **Go 1.26+** to build.
> - **ffmpeg** with `ffprobe` at runtime.
> - For sound, any one of `pacat` (PulseAudio), `pw-cat` (PipeWire), `aplay` (ALSA) or `ffplay`. Without one of them, termo plays the picture without sound.
> - A truecolor terminal is recommended (kitty, ghostty, alacritty, foot, wezterm, iTerm2…). 256-color and 16-color terminals work too: the picture is quantized and dithered to what the terminal can show. `--depth 8` or `--depth 4` forces it.
>
> ---
>
> ## 📝 Notes
>
> - **"Full HD"** means full-HD *sources*. They are decoded and scaled down to your terminal grid. The resolution on screen is the grid multiplied by the sub-cells of the render mode. Shrink the font for more detail.
> - **Sextants** need a font or terminal with the Unicode "Symbols for Legacy Computing" block. kitty, ghostty, wezterm and foot draw them natively.
> - If sound and picture drift apart on your setup, adjust **Playback → Audio delay** in the menu, or pass `--audio-delay`.
> - Terminal resizing is handled on the fly.
> - Licensed under the [GNU GPL v3](LICENSE).
