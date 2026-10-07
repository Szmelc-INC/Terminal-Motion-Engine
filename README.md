# Terminal-Motion-Engine
Script to play media (gif/video), in terminal as ASCII


# Demo:
![output](https://github.com/user-attachments/assets/faef350c-63e8-4ebc-bdfb-ab74071356b0)

> Also this music Video on YouTube has a videoclip made with termo
> <img width="1917" height="1070" alt="image" src="https://github.com/user-attachments/assets/705c763f-06e7-492b-85a0-445b80d48f7a" />
> https://www.youtube.com/watch?v=KoaDMKpmaZo

# Usage:
```bash
./splice.sh clip.gif            # → frames/clip/frame_000001.jpg ...
./splice.sh -r 15 -w 480 video.mp4 # resample to 15 fps, 480 px wide
./termo.sh -I -f 15 frames/video
```
> `./splice.sh -h` for all options.
