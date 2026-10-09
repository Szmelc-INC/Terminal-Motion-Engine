<div align="center">

<picture>
    <source media="(prefers-color-scheme: dark)"  srcset="https://raw.githubusercontent.com/serainox420/serainox420/personal/vector/logo-white.png">
    <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/serainox420/serainox420/personal/vector/logo-black.png">
    <img src="https://i.imgur.com/qpgeyna.png" alt="termo" width="124" />

</picture><h1><samp><b>𝙩𝙚𝙧𝙢𝙤</b></samp></h1>


<table><tr><td>𝚃𝚎𝚛𝚖𝚒𝚗𝚊𝚕 𝙼𝚘𝚝𝚒𝚘𝚗 𝙴𝚗𝚐𝚒𝚗𝚎 · 𝟹.𝟶</td></tr></table>
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
> ## ***You are looking at Terminal Command Line...***
> No, this is not a GUI of a video player...
>
> <img width="384" height="216" alt="v3" src="https://github.com/user-attachments/assets/561df610-a31a-4e1b-bb27-6b5491739350" />
>


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
> `termo` is a single Go binary. Since 2.0 it replaces the `jp2a` + bash scripts of 1.x, which are kept in [`legacy/`](legacy/).
> You do not split a video into a folder of frames first, and the screen is not cleared between frames.
>
> <details open>
> <summary><b>✨ What's new in 3.0</b></summary>
>
> | Feature | Details |
> |---|---|
> | **Bind modes** | The keys are split into five sets — `PLAY`, `VIDEO`, `COLOR`, `FX`, `AUDIO` — and one is active at a time. `Tab` switches, the bars change color with it, and a line above the seek bar lists what the letters do right now. |
> | **F1–F12 windows** | Twelve windows manage everything: keys, picture, adjustments, effects, sound, sound effects, presets, palettes, media search, playlist and downloads, playback, preferences. |
> | **Two independent sizes** | `Ctrl` `+` / `-` resizes only the picture (zoom, crop, pan). `Alt` `+` / `-` resizes only the interface. |
> | **Video effects** | VHS tape, tracking errors, CRT curvature and masks, scanlines, RGB split and anaglyph 3D, glitch, jitter, wave, motion trails, posterize, pixelate, blur, glow, grain, vignette. 38 effect presets, 26 color filters. |
> | **Audio effects** | 96 sound settings and 54 sound presets: equaliser, compressor, reverb, echo, pitch, bit crusher and sample-rate reduction with audio dither, tremolo, ring modulation, stutter, and a synth that follows the track. |
> | **Find media** | Search YouTube, Giphy, Tenor, Pinterest, archive.org and Wikimedia from inside the player. Preview, read the details, play from the web, or download as mp4, webm, mkv, gif, mp3, m4a, opus, flac or wav. |
> | **Managers and generators** | Palettes, glyph ramps, interface themes, looks, sounds and effects can be listed, filtered, created, generated, edited, copied, renamed and deleted in the player. |
> | **More built in** | 125 palettes in 7 families, 38 glyph ramps, 33 dithers, 57 looks, 18 interface themes. |
>
> </details>
> <details>
> <summary><b>✨ What came with 2.0</b></summary>
>
> | Feature | Details |
> |---|---|
> | **Plays files directly** | Any GIF, video or picture that ffmpeg can read. Several files make a playlist. |
> | **Audio** | The sound track plays in sync, through PulseAudio, PipeWire, ALSA or ffplay. |
> | **Smooth output** | Only the cells that changed are written, in one write per frame, inside a synchronized update. No `clear`, no tearing, no blinking. |
> | **Fast** | Turning a frame into a 240×67 grid takes 0.1–4.5 ms depending on mode and dither, and about 6.5 ms with a stack of effects on top, against a 16.7 ms budget. `termo bench` measures it on your machine. |
> | **5 render modes** | Half blocks, quadrants, sextants, braille and ASCII. |
> | **Randomize** | One key rolls a whole new look, and undo brings the last one back. |
> | **A real TUI** | Everything works with the keyboard and with the mouse. |
>
> </details>
> <details>
> <summary><b>🔀 Migrating</b></summary>
>
> **From 2.0** — three keys changed, because the letters now belong to the bind modes:
> - `Tab` switches the bind mode. It used to open the settings menu: that is now `Enter`, a right-click, or an F key.
> - `s` / `S` no longer open the menu, and `t` `T` `w` `W` no longer step through the effects. Effects are `p` / `P` / `r` in the `FX` mode.
> - `-` / `=` no longer change the number of colors. They repeat the last setting key; colors are `k` / `K` in the `VIDEO` mode.
>
> **From 1.x**
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
> termo <link>                      # play a YouTube / web link without saving it
> termo find  [words…]              # search the web for media: browse, preview, play, download
> termo search [--site S] words…    # print search results
> termo get [--as FORMAT] <link | words…>   # download a link or the best match
> termo snap  [options] <file>      # print a single frame (--at SEC, --size COLSxROWS)
> termo bench [options] <file>      # measure render speed (--frames N, --size, --all)
> termo info  <file>                # show stream information
> termo presets [list|show|save|edit|rename|rm|path] [name]
> termo palettes [list FAMILY|add|import|rm|export]
> termo charsets [add|gen|rm]       # glyph ramps
> termo themes · termo sounds · termo effects   # list what is built in and what is yours
> termo options                     # every setting with its values
> termo keys [MODE]                 # every key of the player
> ```
> ### Examples
> ```sh
> termo clip.mp4                       # play
> termo                                # no file: open the file browser
> termo a.gif b.mp4 c.webm             # playlist (F10 manages it)
> termo -m braille -p matrix clip.mp4  # pick a look up front
> termo -P vhs-tape clip.mp4           # start from a preset
> termo --fx crt-tv --sound lofi-tape clip.mp4   # effects and sound presets on top
> termo --keys fx clip.mp4             # start with the effect keys active
> termo -r clip.mp4                    # start with a random look
> termo -p harmony --scheme triadic --hue 20 -c 6 -d halftone clip.mp4
>
> termo find cat memes                 # open the media finder with a search
> termo get --as gif 'dancing cat'     # download the best match as a GIF
> termo get --as mp3 https://youtu.be/…
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
> | `-P`, `--preset <name>` | Start from a look preset |
> | `--fx <name>` · `--filter <name>` | Effect preset on top of the look · color filter |
> | `--sound <name>` | Start with a sound preset |
> | `--keys <mode>` | Bind mode to start in: `play`, `video`, `color`, `fx`, `audio` |
> | `-r`, `--random` | Start with a randomized look (`--seed <n>` repeats it) |
> | `--zoom <x>` · `--ui <size>` | Picture size, 0.1–8 · interface size: `compact`, `normal`, `large`, `huge` |
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
> Every setting of every panel is also a flag: 170 of them. Run `termo options` for the full list.
>
> ### Keys that work everywhere
> | Action | Key(s) |
> |---|---|
> | Play / pause | `Space` |
> | Seek 5 s · 30 s | `←` `→` · `Shift`+`←` `→` |
> | Previous / next frame | `,` `.` |
> | Jump to 0–90 % · start | `0`–`9` · `Home` |
> | Volume · mute | `↑` `↓` · `m` |
> | Speed | `[` `]` |
> | **Next / previous bind mode** | **`Tab`** / `Shift`+`Tab` (also `` ` `` / `~`) |
> | Pick a bind mode | `Alt`+`1` … `Alt`+`5` |
> | Open the panel of the active mode | `Enter` |
> | Reset what the active mode controls | `Backspace` |
> | Repeat the last setting key, up / down | `=` / `-` |
> | **Picture bigger / smaller / 100 %** | `Ctrl`+`+` / `Ctrl`+`-` / `Ctrl`+`0` (also `z` / `Z` / `\`) |
> | Pan a zoomed picture | `Alt`+arrows |
> | **Interface bigger / smaller / normal** | `Alt`+`+` / `Alt`+`-` / `Alt`+`0` |
> | Undo the last change (looks, effects, sound) | `u` |
> | Save the current look as a preset | `Ctrl+S` |
> | HUD auto/on/off | `h` |
> | Open a file · find media on the web | `o` · `/` |
> | The twelve windows · next / previous one | `F1` … `F12` · `>` / `<` |
> | Redraw the screen | `Ctrl+L` |
> | Quit | `q` / `Ctrl+C` |
>
> ### Bind modes
> One mode is active at a time. Its name is lit on the title bar, the logo, the seek bar and the key hints take its color, and the line above the seek bar lists its keys. The colors are the accent of your theme with the hue turned, so they follow the theme.
>
> Every mode obeys the same grammar: **`r`** random · **`p` / `P`** next / previous preset · **a small letter raises a setting or steps forward, its capital goes back** · `=` / `-` repeat the last one · `Enter` opens the panel · `Backspace` resets.
>
> | Mode | Panel | Keys |
> |---|---|---|
> | **`PLAY`** `Alt+1` | Playback `F11` | `n` file · `s` speed · `l` loop · `k` pause · `x` shuffle · `a` audio sync · `f` fps cap · `p` look preset · `r` random look · `R` random palette · `i` what is playing · `e` playlist · `g` statistics |
> | **`VIDEO`** `Alt+2` | Picture `F2` | `v` mode · `c` palette · `d` dither · `a` glyph ramp · `k` colors · `w` dither amount · `s` scheme · `j` hue · `t` color on/off · `b` background · `f` fit · `x` `y` flip · `g` mirror · `e` edges · `i` invert · `p` look preset · `r` random look |
> | **`COLOR`** `Alt+3` | Adjust `F3` | `b` brightness · `c` contrast · `g` gamma · `s` saturation · `e` exposure · `t` temperature · `i` tint · `j` hue shift · `v` vibrance · `d` shadows · `l` highlights · `k` black point · `w` white point · `a` fade · `x` sharpness · `y` vignette · `n` glow · `p` filter · `f` filter amount · `r` random filter |
> | **`FX`** `Alt+4` | Effects `F4` | `p` effect preset · `r` random effects · `v` VHS · `s` scanlines · `c` CRT curvature · `k` CRT mask · `g` glitch · `a` RGB split · `d` split mode (3D) · `e` posterize · `f` blur · `l` motion trails · `x` pixelate · `t` tracking · `b` bleed · `j` jitter · `w` wave · `n` grain · `y` hue cycle · `i` interlace |
> | **`AUDIO`** `Alt+5` | Sound `F5` | `p` sound preset · `r` random sound · `b` bass · `d` mid · `t` treble · `g` gain · `f` pitch · `v` reverb · `e` echo · `l` low-pass · `k` high-pass · `a` drive · `x` bit crush · `s` sample rate · `j` tremolo · `w` width · `y` synth · `c` compressor · `i` limiter · `n` normalize |
>
> `termo keys` prints the same lists, and `F1` shows them in the player with a page for each mode. Preferences (`F12`) choose the mode termo starts in, or `last` to remember it, and can hide the key hints.
>
> ### The twelve windows
> | Key | Window | What you do there |
> |---|---|---|
> | `F1` | Keys | A page for every bind mode, one for the keys that work everywhere, one for the mouse. `Enter` on a mode's page makes it the active one. |
> | `F2` | Picture | Render mode, glyph ramp, palette, color scheme, dither, geometry. |
> | `F3` | Adjust & filters | Exposure, levels, shadows, highlights, white balance, the color filters, blur, sharpness, glow, grain, vignette. |
> | `F4` | Effects | The effect presets (use, save your own, update, copy, rename, delete, roll at random), then every tape / tube / glitch setting. |
> | `F5` | Sound | Tone, 10-band equaliser, dynamics, reverb and echo. |
> | `F6` | Sound effects | Motion (tremolo, vibrato, flanger, 8D), lo-fi (bits, sample rate, noise), synth. |
> | `F7` | Presets | Looks, sounds and effects: load, create, update, rename, copy, delete, set as default, export. |
> | `F8` | Palettes & symbols | The palette manager and generator, the palette editor, the glyph ramp manager and generator. |
> | `F9` | Find media | Search the web, preview, play, download. |
> | `F10` | Media | The playlist (play, reorder, remove, open, find) and the downloads (play, cancel, open the folder). |
> | `F11` | Playback | Speed, frame rate, picture size, audio sync, and a "Now playing" page with the facts about the stream. |
> | `F12` | Preferences & themes | Interface size, theme, HUD, key hints, start-up mode, download folder, API keys; the theme manager and generator. |
>
> The same key closes its window. While one is open, the hint line shows all twelve and each can be clicked — handy where the terminal or the desktop keeps some F keys to itself. `<` / `>` and the `‹` `›` buttons on a window's title bar walk through them too.
>
> Inside a window: `Tab` or `1`–`9` switch its pages, `↑` `↓` select, `←` `→` change (`Shift` for bigger steps), `Enter` types a value or runs the first action, `/` filters a list, `Esc` closes. A window that belongs to a mode (`F2`–`F6`, `F11`) switches the keys to that mode.
>
> ### Mouse
> | Action | Gesture |
> |---|---|
> | Play / pause | Click the picture |
> | Panel of the active mode | Right-click the picture |
> | Randomize the look | Middle-click |
> | Volume · picture size · interface size | Wheel · `Ctrl`+wheel · `Alt`+wheel |
> | Switch the bind mode | Click a mode name on the title bar, or scroll over them |
> | Run a key from the hint line | Click it; right-click runs it backwards |
> | Open a window | Click it on the F key strip |
> | Seek, change a slider | Drag the seek bar or the slider |
> | Cycle what a button shows | Click or scroll it; right-click goes back; right-click the dice to undo |
> | Move a window | Drag its title bar |
>
> ### Picture size and interface size in kitty
> termo asks the terminal for the kitty keyboard protocol, so `Ctrl`+`=` and `Ctrl`+`-` reach it in kitty, ghostty, foot, wezterm and alacritty. If your terminal keeps `Ctrl`+`+` / `Ctrl`+`-` for its own font size, use `z` / `Z` / `\` instead, or hand the keys over while termo has focus. termo sets the user variable `termo` for that; in `kitty.conf`:
> ```conf
> map --when-focus-on var:termo ctrl+shift+equal
> map --when-focus-on var:termo ctrl+shift+minus
> map --when-focus-on var:termo ctrl+shift+backspace
> ```
> A mapping without an action is removed, so the key goes to the program. Use the key names your own font-size mappings have.

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
> For a system-wide install, build as your user and copy the binary as root:
> ```sh
> make && sudo install -m 0755 termo /usr/local/bin/termo
> ```
> On macOS, `make install` does not work with the BSD `install` command. Use the line above or `go install`.
> Or manually:
> ```sh
> go build -trimpath -ldflags "-s -w" -o termo ./cmd/termo
> ```
> ## Layout
> | Path | Contents |
> |---|---|
> | `cmd/termo` | Command-line entry point |
> | `internal/engine` | Color, palettes, dithering, filters and effects, glyph composition, the option table |
> | `internal/dsp` | Live audio effects: reverb, bit crusher, synth, modulation |
> | `internal/tty` | Diffing cell screen, key and mouse decoder |
> | `internal/media` | ffmpeg video and audio pipelines |
> | `internal/fetch` | Media search and download: YouTube, Giphy, Tenor, Pinterest, archive.org, Wikimedia |
> | `internal/app` | Player, bind modes, HUD, windows, managers, presets |
> | `legacy/` | The termo 1.x bash scripts |
> | `frames/` | Sample frame folder in the 1.x format |

---

> [!NOTE]
> ## Looks: modes, palettes, dithering, effects, presets
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
> ### Glyph ramps (`--charset`)
> 38 are built in: `standard` `detailed` `dense` `minimal` `solid` `blocks` `blocks-fine` `bars-v` `bars-h` `quadrants` `dots` `braille-fill` `lines` `slashes` `crosses` `waves` `box` `circles` `squares` `diamonds` `triangles` `stars` `binary` `numbers` `hex` `alphabet` `caps` `code` `math` `arrows` `cards` `chess` `dice` `music` `runes` `greek` `cyrillic` `katakana`, plus `custom` with `--chars 'TEXT'` (darkest character first).
> `termo charsets add NAME 'TEXT'` saves your own (`--sort` orders the characters from empty to full for you), and `termo charsets gen NAME POOL [COUNT]` or the generator in `F8` picks an even ramp out of a pool of characters.
>
> ### Palettes (`--palette`)
> | Kind | Values |
> |---|---|
> | Full color | `truecolor` |
> | Generated | `harmony` (color theory), `adaptive` (median cut of the picture), `gray`, `cube`, `ansi16`, `xterm256` |
> | Named | 125 palettes in seven families: `1-bit`, `handheld`, `computer`, `pixelart`, `editor`, `ramp`, `mood` |
> | Custom | `custom` with `--custom '#1a1c2c,#f4f4f4,#ef7d57'`, or your own saved palettes |
>
> `termo palettes` shows every palette with its colors, and `termo palettes list pixelart` shows one family. `termo palettes add NAME '#hex,#hex,…'`, `import FILE|URL|lospec:NAME`, `export` and `rm` manage your own; `F8` does the same in the player and adds a generator and an editor.
>
> Harmony schemes (`--scheme`): `monochromatic`, `analogous`, `complementary`, `split-complementary`, `triadic`, `tetradic`, `square`, `hexadic`, `hue-shift`, `golden`, `rainbow`.
> Tune them with `--hue`, `--chroma`, `--lmin`, `--lmax` and `--colors`.
>
> ### Dithering (`--dither`)
> | Family | Algorithms | On video |
> |---|---|---|
> | Ordered | `bayer2` `bayer4` `bayer8` `bayer16` `bluenoise` `whitenoise` `grain` `halftone` `cluster4` `cluster8` `diamond` `spiral` `hlines` `vlines` `diagonal` `crosshatch` `grid` `checker` `waves` `zigzag` `bricks` `weave` | Fixed to the screen, so the pattern stays still |
> | Error diffusion | `floyd-steinberg` `false-fs` `atkinson` `jjn` `stucki` `burkes` `sierra` `sierra2` `sierra-lite` `fan` `shiau-fan` | Good on stills, can shimmer in motion |
>
> `--dither-amount` sets the strength (0–2). `--serpentine` alternates the scan direction for error diffusion.
>
> ### Adjustments and filters (`F3`, the `COLOR` keys)
> `--brightness` `--contrast` `--gamma` `--saturation` `--vibrance` `--exposure` `--black` `--white` `--shadows` `--highlights` `--fade` `--temperature` `--tint` `--hue-shift` `--invert`, then detail: `--blur` `--sharpen` `--glow` `--grain` `--vignette` `--pixelate` `--posterize` `--mirror` `--edges`.
>
> Color filters (`--filter`, strength with `--filter-amount`): `mono` `noir` `sepia` `vintage` `faded` `polaroid` `kodachrome` `technicolor` `xpro` `lomo` `bleach` `teal-orange` `cinema` `cool` `warm` `golden-hour` `infrared` `negative` `cyanotype` `thermal` `night-vision` `x-ray` `gold` `redscale` `duotone` `acid`.
>
> ### Effects (`F4`, the `FX` keys, `--fx`)
> | Effect | Options |
> |---|---|
> | Video tape | `--vhs` `--tracking` `--bleed` `--jitter` |
> | Picture tube | `--scanlines` `--curvature` `--mask` `--interlace` |
> | Glitch and 3D | `--glitch` `--split` with `--split-mode` (RGB split, anaglyph 3D, phase), `--wave` |
> | Time | `--motion-blur` `--hue-cycle` |
>
> 38 effect presets combine them and sit on top of any look: `vhs-tape` `vhs-worn` `vhs-pause` `crt-tv` `crt-arcade` `crt-terminal` `retro-scanlines` `old-tv-static` `anaglyph-3d` `rgb-split` `phase-glitch` `glitch-light` `glitch-heavy` `security-cam` `night-vision` `thermal-cam` `x-ray` `old-film` `silent-movie` `noir` `polaroid` `lomo` `cross-process` `blockbuster` `dream` `ghost-trails` `poster` `pop-comic` `solarized-print` `mosaic` `lofi-webcam` `underwater` `heat-haze` `trip` `kaleidoscope` `mirror-world` `interlaced`. `termo effects` lists them with what they set.
>
> ### Presets
> A look preset stores a whole look, effects included. On Linux, presets live in `~/.config/termo/presets.json` (`$XDG_CONFIG_HOME` is honoured); your palettes, glyph ramps, themes, sounds and effect presets are in `library.json` next to it, and preferences in `config.json`. `termo presets path` prints the location. A preset that you name `default` is loaded at start-up.
> ```sh
> termo presets list
> termo presets show NAME
> termo presets save NAME [look options]
> termo presets edit NAME [look options]
> termo presets rename OLD NEW
> termo presets rm NAME
> ```
> 57 looks are built in, among them `sextant-hd` `ascii-classic` `braille-mono` `gameboy` `newsprint` `matrix` `amber-crt` `cga` `vaporwave` `vhs-tape` `crt-tv` `arcade` `green-terminal` `anaglyph-3d` `thermal-cam` `old-film` `noir` `comic` `woodcut` `engraving` `e-ink` `macintosh` `obra-dinn` `teletext` `c64` `nes` `pico8` `synthwave` `hex-dump` `runes`.
>
> To edit a preset: load it, change what you like, then save over it (`F7`, `s`).

---

> [!NOTE]
> ## Sound: filters, effects, presets
>
> The sound track goes through an effect chain that you shape the way you shape the picture: with the `AUDIO` keys, in `F5` / `F6`, or with flags. Most settings change the sound as it plays.
>
> | Group | Settings |
> |---|---|
> | Tone | Preamp, sub bass, bass, mid, presence, treble, low-pass, high-pass, band-pass |
> | EQ | Ten bands from 31 Hz to 16 kHz |
> | Dynamics | Compressor, limiter, gate, normalize, drive, clipping, exciter |
> | Space | Reverb (size, damping, width, pre-delay), echo, stereo width, balance, auto-pan, karaoke, mono |
> | Motion | Pitch, formant, frequency shift, ring modulation, tremolo, vibrato, chorus, flanger, phaser |
> | Lo-fi | Bit depth 1–16 with audio dither (`rectangular`, `triangular`, `shaped`), quantisation curve, sample-rate reduction, tape wow and flutter, hiss, crackle, stutter, reverse |
> | Synth | `chip`, `acid`, `drone` and `wind` voices that follow the pitch and loudness of the track, with scale, key, glide and arpeggio |
>
> 54 sound presets (`--sound NAME`, `p` / `P` in the `AUDIO` mode, `termo sounds`): `bass-boost` `loudness` `warm-tube` `telephone` `am-radio` `megaphone` `underwater` `next-room` `cathedral` `cave` `stadium` `8d-audio` `karaoke` `lofi-tape` `cassette` `vinyl` `gramophone` `8-bit` `4-bit-crunch` `1-bit-speaker` `chiptune` `acid-bass` `drone` `robot` `dalek` `chipmunk` `demon` `nightcore` `vaporwave` `psychedelic` `jet-flanger` `glitch` `skipping-cd` and more. Save your own in `F7`.

---

> [!NOTE]
> ## Finding and downloading media
>
> `F9` (or `/`, or `termo find words…`) opens the media finder.
>
> | Site | What it finds | Needs |
> |---|---|---|
> | YouTube | Videos; filters for duration and order | `yt-dlp`; a YouTube Data API key makes the search faster and richer, without one `yt-dlp` searches |
> | Giphy | GIFs, stickers, short loops | Nothing; a Giphy API key is used when you have one |
> | Tenor | Reaction GIFs and memes | Nothing |
> | Pinterest | Pins: pictures, GIFs, short videos | `gallery-dl` |
> | Archive | archive.org films, clips, old TV | Nothing |
> | Wikimedia | Free videos, GIFs and pictures | Nothing |
> | URL | Any link that `yt-dlp` or `ffmpeg` can open | — |
>
> In the list: `Enter` plays the result from the web without saving it, `a` adds it to the playlist, `i` shows the details and the available streams, `d` downloads it in the format you pick, `f` `t` `o` set the filters (kind, duration, order), `m` loads more, `l` opens the download folder.
>
> Download formats (`--as` on the command line): `mp4`, `mp4-720`, `mp4-480`, `webm`, `mkv`, `gif`, `mp3`, `m4a`, `opus`, `flac`, `wav`, `original`. Downloads go to `~/Videos/termo` unless Preferences name another folder; `F10` lists them.
>
> API keys are optional. Set them in Preferences (`F12`), where they are stored in `config.json` with mode 0600, or in the environment: `TERMO_YOUTUBE_KEY` or `YOUTUBE_API_KEY`, and `GIPHY_API_KEY`.

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
> - Optional, for the media finder: **yt-dlp** (YouTube and most video sites, playing links) and **gallery-dl** (Pinterest). Giphy, Tenor, archive.org and Wikimedia need neither.
> - A truecolor terminal is recommended (kitty, ghostty, alacritty, foot, wezterm, iTerm2…). 256-color and 16-color terminals work too: the picture is quantized and dithered to what the terminal can show. `--depth 8` or `--depth 4` forces it.
>
> ---
>
> ## 📝 Notes
>
> - **"Full HD"** means full-HD *sources*. They are decoded and scaled down to your terminal grid. The resolution on screen is the grid multiplied by the sub-cells of the render mode. Shrink the font for more detail.
> - **Sextants** need a font or terminal with the Unicode "Symbols for Legacy Computing" block. kitty, ghostty, wezterm and foot draw them natively.
> - The fancier **glyph ramps** (chess, cards, dice, runes, music…) depend on your font having those characters at one cell wide.
> - If sound and picture drift apart on your setup, adjust **Audio delay** in `F11` (or `a` / `A` in the `PLAY` mode), or pass `--audio-delay`.
> - Terminal resizing is handled on the fly.
> - Licensed under the [GNU GPL v3](LICENSE).
>
> ## 🎨 Credits
>
> The `pixelart` and `handheld` palettes are the work of the pixel artists who published them on [Lospec](https://lospec.com/palette-list), among them DawnBringer (DawnBringer 16 and 32), ENDESGA (ENDESGA 32), GrafxKid (Sweetie 16), Arne Niklas Jansson (Arne 16), Kerrie Lake (Resurrect 64), Adigun A. Polack (AAP-64) and Kirokaze; the PICO-8 palette is Lexaloffle's. Each palette page on Lospec names its author. `termo palettes import lospec:NAME` fetches any other palette from there.
> The `editor` palettes follow the color schemes of Nord, Gruvbox, Dracula, Solarized, Catppuccin, Tokyo Night, Monokai, One Dark, Everforest, Rosé Pine, Kanagawa, Ayu, SynthWave '84, Material and GitHub Dark; the `computer` palettes are those of the machines they are named after.
