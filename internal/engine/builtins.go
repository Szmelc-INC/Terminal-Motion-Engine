package engine

// The palettes and glyph ramps that ship with termo. The pixel-art and
// handheld palettes are the work of their authors (DawnBringer, ENDESGA,
// GrafxKid, Arne, Kerrie Lake, Adigun Polack, Kirokaze and others) as
// published on lospec.com; the machine palettes are those of the hardware.

// paletteFamily groups the built-in palettes for the managers.
type paletteFamily struct {
	tag, about string
	list       []NamedPalette
}

func pal(name, colors string) NamedPalette { return NamedPalette{name, hexes(colors)} }

// ramp is a palette blended through a few color stops; with n = 0 the
// stops themselves are the palette.
func ramp(name string, n int, stops string) NamedPalette {
	c := hexes(stops)
	if n > 0 {
		c = Gradient(c, n)
	}
	return NamedPalette{name, c}
}

// wheel is n hues at equal steps around the color wheel, between black
// and white.
func wheel(name string, n int, light, chroma float64) NamedPalette {
	c := []RGB{{0, 0, 0}}
	for i := 0; i < n; i++ {
		c = append(c, FromOklch(light, chroma, float64(i)*360/float64(n)+25))
	}
	return NamedPalette{name, append(c, RGB{255, 255, 255})}
}

// levels is every combination of the given channel values: the palette of
// a machine with a few bits per channel.
func levels(name string, v ...uint8) NamedPalette {
	var c []RGB
	for _, r := range v {
		for _, g := range v {
			for _, b := range v {
				c = append(c, RGB{r, g, b})
			}
		}
	}
	return NamedPalette{name, c}
}

var paletteFamilies = []paletteFamily{
	{"1-bit", "two colors: everything is in the dither", []NamedPalette{
		pal("1bit", "000000 ffffff"),
		pal("paper", "f5f0e1 1c1c1c"),
		pal("monitor-glow", "222323 f0f6f0"),
		pal("paperback", "b8c2b9 382b26"),
		pal("obra-dinn", "2e3037 ebe5ce"),
		pal("ibm51", "323c39 d3c9a1"),
		pal("c64-boot", "352879 6c5eb5"),
	}},
	{"handheld", "four shades, as on a pocket console", []NamedPalette{
		pal("gameboy", "0f380f 306230 8bac0f 9bbc0f"),
		pal("gb-pocket", "1f1f1f 4d533c 8b956d c4cfa1"),
		pal("gb-bgb", "081820 346856 88c070 e0f8d0"),
		pal("virtualboy", "ef0000 a40000 550000 000000"),
		pal("kirokaze-gb", "332c50 46878f 94e344 e2f3e4"),
		pal("ice-cream-gb", "7c3f58 eb6b6f f9a875 fff6d3"),
		pal("demichrome", "211e20 555568 a0a08b e9efec"),
		pal("mist-gb", "2d1b00 1e606e 5ab9a8 c4f0c2"),
		pal("nostalgia", "d0d058 a0a840 708028 405010"),
		pal("wish-gb", "622e4c 7550e8 608fcf 8be5ff"),
		pal("hollow", "0f0f1b 565a75 c6b7be fafbf6"),
		pal("ayy4", "00303b ff7777 ffce96 f1f2da"),
		pal("rustic-gb", "2c2137 764462 edb4a1 a96868"),
		pal("aqua-gb", "002b59 005f8c 00b9be 9ff4e5"),
		pal("crimson", "eff9d6 ba5044 7a1c4b 1b0326"),
		pal("lava-gb", "051f39 4a2480 c53a9d ff8e80"),
		pal("pen-n-paper", "e4dbba a4929a 4f3a54 260d1c"),
		pal("moonlight-gb", "0f052d 203671 36868f 5fc75d"),
		pal("spacehaze", "f8e3c4 cc3495 6b1fb1 0b0630"),
	}},
	{"computer", "the fixed colors of old machines", []NamedPalette{
		pal("cga", "000000 55ffff ff55ff ffffff"),
		pal("cga-hot", "000000 55ff55 ff5555 ffff55"),
		pal("ega16", "000000 555555 aaaaaa ffffff 0000aa 5555ff 00aa00 55ff55 00aaaa 55ffff aa0000 ff5555 aa00aa ff55ff aa5500 ffff55"),
		pal("windows16", "000000 7e7e7e bebebe ffffff 7e0000 fe0000 047e00 06ff04 7e7e00 ffff04 00007e 0000ff 7e007e fe00ff 047e7e 06ffff"),
		pal("macintosh16", "ffffff ffff00 ff6500 dc0000 ff0097 360097 0000ca 0097ff 00a800 006500 653600 976536 b9b9b9 868686 454545 000000"),
		pal("teletext", "000000 ff0000 00ff00 ffff00 0000ff ff00ff 00ffff ffffff"),
		pal("secam", "000000 2121ff f03c79 ff50ff 7fff00 7fffff ffff3f ffffff"),
		pal("c64", "000000 ffffff 880000 aaffee cc44cc 00cc55 0000aa eeee77 dd8855 664400 ff7777 333333 777777 aaff66 0088ff bbbbbb"),
		pal("vic20", "000000 ffffff a8734a e9b287 772d26 b66862 85d4dc c5ffff a85fb4 e99df5 559e4a 92df87 42348b 7e70ca bdcc71 ffffb0"),
		pal("zx", "000000 0000d7 d70000 d700d7 00d700 00d7d7 d7d700 d7d7d7 0000ff ff0000 ff00ff 00ff00 00ffff ffff00 ffffff"),
		pal("apple2", "000000 6c2940 403578 d93cf0 135740 808080 2697f0 bfb4f8 404b07 d9680f eca8bf 26c30f bfca87 93d6bf ffffff"),
		pal("msx", "000000 cacaca ffffff b75e51 d96459 fe877c cac15e ddce85 3ca042 40b64a 73ce7c 5955df 7e75f0 64daee b565b3"),
		pal("intellivision", "0c0005 a7a8a8 fffcff ff3e00 ffa600 faea27 00780f 00a720 6ccd30 002dff 5acbff bd95ff c81a7d ff3276 3c5800 c9d464"),
		pal("amstrad", "040404 808080 ffffff 800000 ff0000 ff8080 ff7f00 ffff80 ffff00 808000 008000 01ff00 80ff00 80ff80 01ff80 008080 "+
			"01ffff 80ffff 0080ff 0000ff 00007f 7f00ff 8080ff ff80ff ff00ff ff0080 800080"),
		pal("nes", "000000 fcfcfc f8f8f8 bcbcbc 7c7c7c a4e4fc 3cbcfc 0078f8 0000fc b8b8f8 6888fc 0058f8 0000bc d8b8f8 9878f8 6844fc "+
			"4428bc f8b8f8 f878f8 d800cc 940084 f8a4c0 f85898 e40058 a80020 f0d0b0 f87858 f83800 a81000 fce0a8 fca044 e45c10 "+
			"881400 f8d878 f8b800 ac7c00 503000 d8f878 b8f818 00b800 007800 b8f8b8 58d854 00a800 006800 b8f8d8 58f898 00a844 "+
			"005800 00fcfc 00e8d8 008888 004058 f8d8f8 787878"),
		levels("mastersystem", 0x00, 0x55, 0xaa, 0xff),
	}},
	{"pixelart", "palettes drawn by pixel artists", []NamedPalette{
		pal("pico8", "000000 1d2b53 7e2553 008751 ab5236 5f574f c2c3c7 fff1e8 ff004d ffa300 ffec27 00e436 29adff 83769c ff77a8 ffccaa"),
		pal("sweetie16", "1a1c2c 5d275d b13e53 ef7d57 ffcd75 a7f070 38b764 257179 29366f 3b5dc9 41a6f6 73eff7 f4f4f4 94b0c2 566c86 333c57"),
		pal("db16", "140c1c 442434 30346d 4e4a4e 854c30 346524 d04648 757161 597dce d27d2c 8595a1 6daa2c d2aa99 6dc2ca dad45e deeed6"),
		pal("db32", "000000 222034 45283c 663931 8f563b df7126 d9a066 eec39a fbf236 99e550 6abe30 37946e 4b692f 524b24 323c39 3f3f74 "+
			"306082 5b6ee1 639bff 5fcde4 cbdbfc ffffff 9badb7 847e87 696a6a 595652 76428a ac3232 d95763 d77bba 8f974a 8a6f30"),
		pal("endesga32", "be4a2f d77643 ead4aa e4a672 b86f50 733e39 3e2731 a22633 e43b44 f77622 feae34 fee761 63c74d 3e8948 265c42 193c3e "+
			"124e89 0099db 2ce8f5 ffffff c0cbdc 8b9bb4 5a6988 3a4466 262b44 181425 ff0044 68386c b55088 f6757a e8b796 c28569"),
		pal("arne16", "000000 493c2b be2633 e06f8b 9d9d9d a46422 eb8931 f7e26b ffffff 1b2632 2f484e 44891a a3ce27 005784 31a2f2 b2dcef"),
		pal("na16", "8c8fae 584563 3e2137 9a6348 d79b7d f5edba c0c741 647d34 e4943a 9d303b d26471 70377f 7ec4c1 34859d 17434b 1f0e1c"),
		pal("steam-lords", "213b25 3a604a 4f7754 a19f7c 77744f 775c4f 603b3a 3b2137 170e19 2f213b 433a60 4f5277 65738c 7c94a1 a0b9ba c0d1cc"),
		pal("bubblegum16", "16171a 7f0622 d62411 ff8426 ffd100 fafdff ff80a4 ff2674 94216a 430067 234975 68aed4 bfff3c 10d275 007899 002859"),
		pal("lost-century", "d1b187 c77b58 ae5d40 79444a 4b3d44 ba9158 927441 4d4539 77743b b3a555 d2c9a5 8caba1 4b726e 574852 847875 ab9b8e"),
		pal("eroge-copper", "0d080d 4f2b24 825b31 c59154 f0bd77 fbdf9b fff9e4 bebbb2 7bb24e 74adbb 4180a0 32535f 2a2349 7d3840 c16c5b e89973"),
		pal("fantasy24", "1f240a 39571c a58c27 efac28 efd8a1 ab5c1c 183f39 ef692f efb775 a56243 773421 724113 2a1d0d 392a1c 684c3c 927e6a "+
			"276468 ef3a0c 45230d 3c9f9c 9b1a0a 36170c 550f0a 300f0a"),
		pal("vinik24", "000000 6f6776 9a9a97 c5ccb8 8b5580 c38890 a593a5 666092 9a4f50 c28d75 7ca1c0 416aa3 8d6268 be955c 68aca9 387080 "+
			"6e6962 93a167 6eaa78 557064 9d9f7f 7e9e99 5d6872 433455"),
		pal("cc29", "f2f0e5 b8b5b9 868188 646365 45444f 3a3858 212123 352b42 43436a 4b80ca 68c2d3 a2dcc7 ede19e d3a068 b45252 6a536e "+
			"4b4158 80493a a77b5b e5ceb4 c2d368 8ab060 567b79 4e584a 7b7243 b2b47e edc8c4 cf8acb 5f556a"),
		pal("zughy32", "472d3c 5e3643 7a444a a05b53 bf7958 eea160 f4cca1 b6d53c 71aa34 397b44 3c5956 302c2e 5a5353 7d7071 a0938e cfc6b8 "+
			"dff6f5 8aebf1 28ccdf 3978a8 394778 39314b 564064 8e478c cd6093 ffaeb6 f4b41b f47e1b e6482e a93b3b 827094 4f546b"),
		pal("apollo", "172038 253a5e 3c5e8b 4f8fba 73bed3 a4dddb 19332d 25562e 468232 75a743 a8ca58 d0da91 4d2b32 7a4841 ad7757 c09473 "+
			"d7b594 e7d5b3 341c27 602c2c 884b2b be772b de9e41 e8c170 241527 411d31 752438 a53030 cf573c da863e 1e1d39 402751 "+
			"7a367b a23e8c c65197 df84a5 090a14 10141f 151d28 202e37 394a50 577277 819796 a8b5b2 c7cfcc ebede9"),
		pal("resurrect64", "2e222f 3e3546 625565 966c6c ab947a 694f62 7f708a 9babb2 c7dcd0 ffffff 6e2727 b33831 ea4f36 f57d4a ae2334 e83b3b "+
			"fb6b1d f79617 f9c22b 7a3045 9e4539 cd683d e6904e fbb954 4c3e24 676633 a2a947 d5e04b fbff86 165a4c 239063 1ebc73 "+
			"91db69 cddf6c 313638 374e4a 547e64 92a984 b2ba90 0b5e65 0b8a8f 0eaf9b 30e1b9 8ff8e2 323353 484a77 4d65b4 4d9be6 "+
			"8fd3ff 45293f 6b3e75 905ea9 a884f3 eaaded 753c54 a24b6f cf657f ed8099 831c5d c32454 f04f78 f68181 fca790 fdcbb0"),
		pal("aap64", "060608 141013 3b1725 73172d b4202a df3e23 fa6a0a f9a31b ffd541 fffc40 d6f264 9cdb43 59c135 14a02e 1a7a3e 24523b "+
			"122020 143464 285cc4 249fde 20d6c7 a6fcdb ffffff fef3c0 fad6b8 f5a097 e86a73 bc4a9b 793a80 403353 242234 221c1a "+
			"322b28 71413b bb7547 dba463 f4d29c dae0ea b3b9d1 8b93af 6d758d 4a5462 333941 422433 5b3138 8e5252 ba756a e9b5a3 "+
			"e3e6ff b9bffb 849be4 588dbe 477d85 23674e 328464 5daf8d 92dcba cdf7e2 e4d2aa c7b08b a08662 796755 5a4e44 423934"),
		pal("journey", "050914 110524 3b063a 691749 9c3247 d46453 f5a15d ffcf8e ff7a7d ff417d d61a88 94007a 42004e 220029 100726 25082c "+
			"3d1132 73263d bd4035 ed7b39 ffb84a fff540 c6d831 77b02a 429058 2c645e 153c4a 052137 0e0421 0c0b42 032769 144491 "+
			"488bd4 78d7ff b0fff1 faffff c7d4e1 928fb8 5b537d 392946 24142c 0e0f2c 132243 1a466b 10908e 28c074 3dff6e f8ffb8 "+
			"f0c297 cf968c 8f5765 52294b 0f022e 35003b 64004c 9b0e3e d41e3c ed4c40 ff9757 d4662f 9c341a 691b22 450c28 2d002e"),
		pal("nyx8", "08141e 0f2a3f 20394f f6d6bd c3a38a 997577 816271 4e495f"),
		pal("slso8", "0d2b45 203c56 544e68 8d697a d08159 ffaa5e ffd4a3 ffecd6"),
		pal("ammo8", "040c06 112318 1e3a29 305d42 4d8061 89a257 bedc7f eeffcc"),
		pal("dreamscape8", "c9cca1 caa05a ae6a47 8b4049 543344 515262 63787d 8ea091"),
		pal("pollen8", "73464c ab5675 ee6a7c ffa7a5 ffe07e ffe7d6 72dcbb 34acba"),
		pal("rust-gold8", "f6cd26 ac6b26 563226 331c17 bb7f57 725956 393939 202020"),
		pal("funkyfuture8", "2b0f54 ab1f65 ff4f69 fff7f8 ff8142 ffda45 3368dc 49e7ec"),
		pal("oil6", "fbf5ef f2d3ab c69fa5 8b6d9c 494d7e 272744"),
		pal("curiosities", "46425e 15788c 00b9be ffeecc ffb0a3 ff6973"),
		pal("twilight5", "fbbbad ee8695 4a7a96 333f58 292831"),
		pal("ink", "1f1f29 413a42 596070 96a2b3 eaf0d8"),
	}},
	{"editor", "the colors of well-known editor themes", []NamedPalette{
		pal("nord", "2e3440 3b4252 434c5e 4c566a d8dee9 e5e9f0 eceff4 8fbcbb 88c0d0 81a1c1 5e81ac bf616a d08770 ebcb8b a3be8c b48ead"),
		pal("gruvbox", "282828 cc241d 98971a d79921 458588 b16286 689d6a a89984 928374 fb4934 b8bb26 fabd2f 83a598 d3869b 8ec07c ebdbb2"),
		pal("dracula", "282a36 44475a f8f8f2 6272a4 8be9fd 50fa7b ffb86c ff79c6 bd93f9 ff5555 f1fa8c"),
		pal("solarized", "002b36 073642 586e75 657b83 839496 93a1a1 eee8d5 fdf6e3 b58900 cb4b16 dc322f d33682 6c71c4 268bd2 2aa198 859900"),
		pal("catppuccin", "1e1e2e 313244 45475a 585b70 cdd6f4 f5e0dc f5c2e7 cba6f7 f38ba8 fab387 f9e2af a6e3a1 94e2d5 89dceb 89b4fa b4befe"),
		pal("tokyo-night", "1a1b26 24283b 414868 565f89 a9b1d6 c0caf5 7aa2f7 7dcfff 2ac3de 9ece6a e0af68 ff9e64 f7768e bb9af7 73daca b4f9f8"),
		pal("monokai", "272822 3e3d32 75715e f8f8f2 f92672 fd971f e6db74 a6e22e 66d9ef ae81ff"),
		pal("one-dark", "282c34 3e4451 5c6370 abb2bf e06c75 be5046 d19a66 e5c07b 98c379 56b6c2 61afef c678dd"),
		pal("everforest", "2d353b 343f44 475258 859289 d3c6aa e67e80 e69875 dbbc7f a7c080 83c092 7fbbb3 d699b6"),
		pal("rose-pine", "191724 1f1d2e 26233a 6e6a86 908caa e0def4 eb6f92 f6c177 ebbcba 31748f 9ccfd8 c4a7e7"),
		pal("kanagawa", "1f1f28 2a2a37 54546d 727169 dcd7ba c8c093 e82424 ff5d62 ffa066 e6c384 98bb6c 7aa89f 7fb4ca 7e9cd8 957fb8 d27e99"),
		pal("ayu", "0a0e14 1f2430 4d5566 b3b1ad ff3333 f07178 ff8f40 ffb454 e6b450 c2d94c 95e6cb 39bae6 59c2ff d2a6ff"),
		pal("synthwave84", "241b2f 262335 34294f 848bbd ffffff fe4450 f97e72 fede5d 72f1b8 36f9f6 2ee2fa ff7edb"),
		pal("material", "263238 37474f 546e7a b0bec5 eeffff f07178 f78c6c ffcb6b c3e88d 89ddff 82aaff c792ea"),
		pal("github-dark", "0d1117 161b22 30363d 8b949e c9d1d9 f0f6fc ff7b72 ffa657 d29922 3fb950 56d364 79c0ff 58a6ff bc8cff"),
	}},
	{"ramp", "one hue family from dark to light", []NamedPalette{
		ramp("amber", 0, "000000 331a00 663300 995200 cc7a00 ffb000 ffd27f"),
		ramp("matrix", 0, "000000 003b00 008f11 00ff41 afffaf"),
		ramp("sepia", 0, "1a1208 3d2b17 6b4c2a a07a4a d1ab78 f1dcb3"),
		ramp("ice", 0, "03045e 023e8a 0077b6 0096c7 00b4d8 48cae4 90e0ef caf0f8"),
		ramp("sunset", 0, "1b1031 4b1d52 8c2f5b c94f4f f08a4b f9c74f fff3b0"),
		ramp("thermal", 12, "000004 2d0a5a 8a1c7c d43d51 f98e09 f9d423 ffffe0"),
		ramp("viridis", 10, "440154 3b528b 21918c 5ec962 fde725"),
		ramp("magma", 10, "000004 3b0f70 8c2981 de4968 fe9f6d fcfdbf"),
		ramp("plasma", 10, "0d0887 7e03a8 cc4778 f89540 f0f921"),
		ramp("night-vision", 8, "000300 062a08 12701a 3ed44a b8ffbe"),
		ramp("blueprint", 6, "041733 0b3a78 2a6fc0 9cc9f5 ffffff"),
		ramp("cyanotype", 8, "04111f 0c2f52 16639a 5aa7d0 c4e6f3 f4fbfd"),
		ramp("fire", 10, "000000 4a0000 b01000 ff5a00 ffc400 ffffc0"),
		ramp("blood", 6, "000000 2a0004 6b000c b80f1a ff5a4f ffd6cf"),
		ramp("gold", 8, "120a02 5a3a08 a8760f e3b73b fff3b8"),
		ramp("copper", 8, "000000 3a1f10 7d4524 c07a45 f0b98a ffe9d2"),
		ramp("coffee", 8, "120b07 3a2417 6b452b a67b52 d9b88c f5e9d3"),
		ramp("autumn", 8, "1c0f0a 5a1e0e a23b12 dc7a1a f2b84b f8e7b0"),
		ramp("forest", 8, "0a140a 163a1c 2f6b2b 6fa33a c6d66a f4f1c0"),
		ramp("toxic", 6, "050a00 1f3a00 4f8a00 a3e000 e6ff5a"),
		ramp("ocean", 10, "01030f 04255c 0a5c9c 1fa3c4 7fe0d6 e8fff4"),
		ramp("arctic", 7, "0b1d2e 1f4b6e 4d8fb3 9ccfe0 e3f6fa ffffff"),
		ramp("twilight", 8, "0b0b2a 2d1b5e 6a2c8f b54a9a f08a8a ffd9a0"),
		ramp("ultraviolet", 7, "05000f 2a0a5e 6a1bd1 b14cff e6a6ff ffffff"),
		ramp("candy", 8, "2b1055 7a2a9a d94fb4 ff8fb3 ffc6a8 fff1c9"),
		ramp("sakura", 6, "2a1a2e 6b3a5b c46a8c f5a8b8 ffd9df fff5f2"),
		ramp("e-ink", 5, "1a1a18 55534d 8d8a80 c2beb1 efe9d9"),
	}},
	{"mood", "color sets with an attitude", []NamedPalette{
		pal("vaporwave", "1a1033 2d1b69 7b2cbf ff71ce 01cdfe 05ffa1 b967ff fffb96"),
		pal("cyberpunk", "0d0221 261447 541388 d100d1 f706cf 00f0ff fcee0c ffffff"),
		pal("miami", "01012b 0d0221 005678 05d9e8 d1f7ff ff2a6d ff6c11 f9c80e"),
		pal("neon", "000000 2b00ff ff00d4 ff3d00 f6ff00 00ff66 00fff0 ffffff"),
		pal("aurora", "020d1a 0b3d4f 12806a 4fd68a b5f5a0 7a5cff d9a6ff f2f7ff"),
		pal("earth", "2b2118 5c4630 8a6f47 b59a6a d8c79e 3f5a3a 6b7d4f 8fa7a8"),
		pal("halloween", "0b0b0b 3d1f5c 6a994e f77f00 ffb703 f1faee"),
		pal("xmas", "0b3d0b 165b33 bb2528 ea4630 f8b229 f5f5f5"),
		pal("cmyk", "000000 2e3192 00a651 00aeef ed1c24 ec008c fff200 ffffff"),
		wheel("rainbow12", 12, 0.72, 0.19),
		wheel("pastel12", 12, 0.86, 0.08),
	}},
}

func builtinPalettes() []NamedPalette {
	var out []NamedPalette
	for _, f := range paletteFamilies {
		out = append(out, f.list...)
	}
	return out
}

// PaletteTag names the family a built-in palette belongs to ("handheld",
// "computer", "pixelart", "ramp"…), or "" for any other palette.
func PaletteTag(name string) string {
	for _, f := range paletteFamilies {
		for _, p := range f.list {
			if p.Name == name {
				return f.tag
			}
		}
	}
	return ""
}

// PaletteFamilies lists the families and what they are.
func PaletteFamilies() [][2]string {
	out := make([][2]string, len(paletteFamilies))
	for i, f := range paletteFamilies {
		out[i] = [2]string{f.tag, f.about}
	}
	return out
}

func set(name, chars string) Charset { return Charset{name, []rune(chars)} }

// sorted is a ramp made of the given glyphs, put in order of density.
func sorted(name, chars string) Charset { return Charset{name, []rune(SortByDensity(" " + chars))} }

// builtinCharsets lists the glyph ramps, each from the emptiest glyph to
// the fullest. "custom" is last: it stands for the user's own glyphs.
func builtinCharsets() []Charset {
	return []Charset{
		set("standard", " .:-=+*#%@"),
		set("detailed", " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$"),
		set("dense", " .,:;i1tfLCG08@"),
		set("minimal", " .oO@"),
		set("solid", " █"),
		set("blocks", " ░▒▓█"),
		set("blocks-fine", " ▁▂░▃▄▒▅▆▓▇█"),
		set("bars-v", " ▁▂▃▄▅▆▇█"),
		set("bars-h", " ▏▎▍▌▋▊▉█"),
		set("quadrants", " ▘▝▖▗▚▞▙▛▜▟█"),
		set("dots", " ⠁⠃⠇⠏⠟⠿⡿⣿"),
		set("braille-fill", " ⠄⠆⠖⠶⡶⣶⣾⣿"),
		set("lines", " ˙-~=≡#"),
		set("slashes", " ./\\|X#"),
		set("crosses", " ·+×╳#"),
		set("waves", " ·˜~≈≋"),
		set("box", " ╶─┼╋╬"),
		set("circles", " ·•○◎◉●"),
		set("squares", " ·▫▪□▣■"),
		set("diamonds", " ·⋄◇◈◆"),
		set("triangles", " ·▵▴△▲"),
		set("stars", " .·⋆+*★"),
		set("binary", " 01"),
		sorted("numbers", "0123456789"),
		sorted("hex", "0123456789ABCDEF"),
		sorted("alphabet", "abcdefghijklmnopqrstuvwxyz"),
		sorted("caps", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
		sorted("code", ".,;:!|/(){}[]<>=+*#&%@$"),
		set("math", " ·∙∶∴≈≡≣⊕⊗⊞⊠"),
		set("arrows", " ·↑→↗↔⇒⇔"),
		set("cards", " ·♤♡♢♧♠♥♦♣"),
		set("chess", " ♙♘♗♖♕♔♟♞♝♜♛♚"),
		set("dice", " ⚀⚁⚂⚃⚄⚅"),
		set("music", " ·♩♪♫♬"),
		set("runes", " ᛁᚲᚢᚦᚱᚠᚨᛒᛗᛞᛟ"),
		set("greek", " ·ιτγνλσπαεδβθψωΞΣΦΩΨ"),
		set("cyrillic", " ·гтсукеаодлбяфюжщш"),
		set("katakana", " ･ｰｧｨｩｱｲｳｴｵｶｷｸｹｻｼｽｾﾀﾁﾂﾃﾄﾅﾆﾇﾈﾊﾋﾌﾍﾎﾏﾐﾑﾒﾓﾔﾕﾖﾗﾘﾙﾚﾛﾜﾝ"),
		{"custom", nil},
	}
}
