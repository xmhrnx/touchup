<p align="center"><img src="assets/icon.png" width="96" alt="TouchUp icon"></p>

<h1 align="center">TouchUp</h1>

<p align="center">Upscale images, video and audio with open-source AI engines that run on your own computer.</p>

TouchUp is a desktop app built with [giode](https://github.com/mehran9675/giode). It doesn't
upscale anything itself: it installs and drives command-line engines such as Real-ESRGAN or
AudioSR, and takes care of everything around them. That means extracting and re-encoding video
frames, keeping the original soundtrack, converting audio, running jobs in a queue and showing
before/after comparisons. Nothing is uploaded.

Videos go through image engines a segment at a time: one FFmpeg process decodes frames into a
pipe, TouchUp hands the engine a few dozen at once, and the results go straight into the encoder,
so the work folder never holds more than a few hundred megabytes, however long the video.

## Features

- **Any engine.** A built-in catalog of open-source engines installs with one click, and you can
  add any other command-line upscaler from a form or a JSON manifest.
- **Models installed one by one.** Engines with large weights list their models on the engine
  card; download only the ones you want, with checksums verified. The studio offers the
  installed ones.
- **Video preview on a single frame.** Scrub the timeline to any moment and upscale just that
  frame. A split view with zoom and pan compares the original with the result before you
  commit to the whole video. Engines that only work on whole videos get a one-second clip
  around the frame; those that also take images, like SeedVR2, get the frame alone.
- **Audio preview on a section.** Choose 5–30 seconds on the waveform, enhance only that, then
  play the original and enhanced versions and compare their spectrograms.
- **Images go straight to the queue.** No preview step.
- **A scale or a resolution.** Upscale by 2x, 3x or 4x, or to 720p, 1080p, 1440p or 4K
  (by the short side, so portrait video works too). Engines told a resolution, like SeedVR2,
  render it directly; the others run at the smallest scale that reaches it, and the result is
  resized to the exact size with Lanczos.
- **Queue.** Jobs run one at a time, with live progress, the engine's latest output, cancel and
  retry. Previews and jobs show how long they've run and warn when the engine has gone quiet,
  and their log so far can be copied while they run.
- **Errors you can report.** Every failure, from a preview, a job, an engine install or a model
  download, has a Copy button. It copies the whole report: what failed, the TouchUp version,
  system, GPU setting and Python, and everything the engine or installer printed.
- **Engine settings.** Engines list what can be tuned: tile sizes for small GPUs, frames per
  batch, precision, threads, model offloading, diffusion steps. Change them from the engine card
  or the studio's **Adjust** button; each says what it does and what its default is.
- **GPU memory that stays on the GPU.** By default, Python engines stop with "out of memory" when
  the GPU is full instead of spilling into system memory, which Windows drivers allow and which
  is often ten times slower or worse. **Settings → Use system memory when the GPU is full** turns
  spilling back on.
- **Output control.** PNG/JPEG/WebP, H.264/H.265/ProRes with a quality setting, WAV/FLAC/MP3,
  GPU selection, and a custom output folder.
- In 26 languages: English, Bulgarian, Croatian, Czech, Danish, Dutch, Estonian, Finnish,
  French, German, Greek, Hungarian, Irish, Italian, Latvian, Lithuanian, Maltese, Norwegian
  Bokmål, Polish, Portuguese, Romanian, Slovak, Slovenian, Spanish, Swedish and Ukrainian, with
  each language's own number formats. It follows the system's language, or the one chosen in
  **Settings → Language**.
- Dark and light themes, file drop, and "Open with TouchUp" on every desktop.
- A custom title bar on Windows. Linux and macOS keep the system frame, because Gio can't
  resize a frameless window from its edges there; set `TOUCHUP_CUSTOM_FRAME=1` to use the
  custom bar anyway (`0` turns it off on Windows).

## Built-in engines

The strongest engines are listed first.

| Engine | Media | What it's best at | Install |
| --- | --- | --- | --- |
| [SeedVR2](https://github.com/numz/ComfyUI-SeedVR2_VideoUpscaler) (3B, 7B, 7B Sharp; FP8 and GGUF) | Video, image | One-step diffusion restoration with real, steady texture: the strongest open video upscaler | Python, NVIDIA GPU (8 GB+) |
| [FlashVSR](https://github.com/OpenImagingLab/FlashVSR) v1.1, through [FlashVSR Ultra Fast](https://github.com/lihaoyun6/ComfyUI-FlashVSR_Ultra_Fast) | Video | One-step streaming diffusion: detail like SeedVR2's at several times the speed, for long videos | Python, NVIDIA GPU (8 GB+) |
| PyTorch SR, through [spandrel](https://github.com/chaiNNer-org/spandrel) | Image, video | 13 community models that restore rather than invent detail, in seconds: DAT2 and RealPLKSR trained on compressed photos, DRCT-L and ATD for clean sources, SwinIR-L, Real-ESRGAN, anime and AI-art models | Python, NVIDIA GPU recommended |
| [Apollo](https://github.com/JusperLee/Apollo) | Audio | Restoring music damaged by MP3 and streaming compression | Python |
| [AudioSR](https://github.com/haoheliu/versatile_audio_super_resolution) | Audio | Diffusion bandwidth extension to 48 kHz for music, speech and effects | Python |
| [Real-ESRGAN](https://github.com/xinntao/Real-ESRGAN) (x4plus, anime, anime video v3) | Image, video | Fast all-rounder on any GPU | Download, Vulkan GPU |
| [Real-CUGAN](https://github.com/nihui/realcugan-ncnn-vulkan) | Image, video | Anime with adjustable denoising | Download, Vulkan GPU |
| [waifu2x](https://github.com/nihui/waifu2x-ncnn-vulkan) | Image, video | Drawings and anime art | Download, Vulkan GPU |
| [RealSR](https://github.com/nihui/realsr-ncnn-vulkan) | Image, video | Natural 4x photos | Download, Vulkan GPU |
| [VoiceFixer](https://github.com/haoheliu/voicefixer) | Audio | Badly degraded speech | Python |
| [Resemble Enhance](https://github.com/resemble-ai/resemble-enhance) | Audio | Speech denoising and enhancement | Python |
| [DeepFilterNet](https://github.com/Rikorose/DeepFilterNet) | Audio | Real-time speech denoising | Python |

PyTorch SR runs every model through `spandrel_runner.py`, a script TouchUp ships into the
engine's folder. It tiles large images to fit in GPU memory, puts video frames that fit in one tile
through the model several at a time, loads and saves frames on worker threads while the GPU works,
keeps transparency, scales 4x models
down to 2x or 3x when asked, and by default restores the original's colors where a model drifts
on saturated hues (`--no-color-fix` turns that off in a custom manifest). On an NVIDIA GPU it runs
in half precision, or in bfloat16 for models that overflow in half (DRCT, DAT, ATD, HAT, SwinIR),
through autocast with the weights kept in float32. When the GPU can't run a model (a card too old
for the installed PyTorch, or output that overflows), it retries in full precision and then on the
CPU, and a failure ends with one `Error:` line that TouchUp shows. Installed PyTorch engines have a
**Check GPU** button that reports the PyTorch and CUDA versions, the GPU and its compute
capability, and whether a small test runs on it in full and half precision.

SeedVR2 runs through `seedvr2_launch.py`, which rounds its chunks of video to whole batches, lets
its VAE use more memory per step than SeedVR2's 0.5 GB when there's room, and can turn on cuDNN's
kernel tuning. SeedVR2 can't split a picture, so TouchUp upscales pictures over 2048 pixels a side
(an engine setting) in overlapping parts and joins them with feathered seams; the parts go to SeedVR2
as one folder, so it starts and loads its model once. Its attention runs on
[SageAttention](https://github.com/thu-ml/SageAttention) when it's installed (an optional step of the
install, through Triton, which on Windows is `triton-windows`), which is much faster than PyTorch's
own on Windows.

SeedVR2 and FlashVSR stay running between jobs (`serve` in the manifest), so previews and later
jobs skip starting Python and loading the model: TouchUp sends a server each job with the same
settings, and stops it after ten idle minutes, when settings change, when another engine runs, after a
failed or cancelled job, and when it quits.

FlashVSR runs through `flashvsr_runner.py` on FlashVSR Ultra Fast's pipeline, whose Triton attention
kernels need no compiling and run on RTX 20 to 50 cards. It upscales video frames in chunks that
start with a few frames of the chunk before, in overlapping tiles, and pads frames to the sizes the
model needs instead of cropping them.

Diffusion models like SeedVR2 take seconds per frame on any home GPU, so a TV episode takes hours;
TouchUp warns before such a job. Which of its settings are fastest depends on the GPU, so **Find the
fastest** in the studio times them on two seconds of the open video: more VAE memory per step, bigger
tiles, cuDNN tuning, the Conv3d workaround off, bigger batches. It keeps each that's faster, stops a
trial that's already slower than the best or runs out of memory, saves the winners and shows what each
try took. Engines declare what to try in their manifest's `tune`.

The GPU memory limit comes from `sitecustomize.py`, which TouchUp puts on `PYTHONPATH` for Python
engines. When the engine starts CUDA, it caps PyTorch at the memory free at that moment.

Engines are installed per user into `~/.local/share/TouchUp/engines` (Linux),
`~/Library/Application Support/TouchUp/engines` (macOS) or `%LOCALAPPDATA%\TouchUp\engines`
(Windows). Python engines each get a private virtual environment there.

## Requirements

- [FFmpeg](https://ffmpeg.org) with `ffprobe` (and `ffplay` to listen to audio previews), on
  `PATH`, next to the TouchUp executable, or set in Settings.
- A GPU with Vulkan for the ncnn engines.
- Nothing for the Python engines: TouchUp downloads its own Python 3.12 (a
  [standalone build](https://github.com/astral-sh/python-build-standalone), checksum-verified)
  the first time one is installed, so a missing, too new or Store-shortcut system Python doesn't
  matter. A Python set in Settings is used instead when it's 3.9–3.13 with venv. An NVIDIA GPU
  is strongly recommended for diffusion models.

## Build

TouchUp needs Go 1.26 or later. Build and run it with the
[giode CLI](https://github.com/mehran9675/giode-cli), which builds against giode's patched Gio
(file drop on Linux, among other things):

```sh
go install github.com/mehran9675/giode-cli/cmd/giode@latest
giode doctor                         # what this computer still needs
giode run                            # run it
giode build linux deb appimage       # or windows-installer, dmg, ...
```

A plain `go run .` works too, without file drop on Linux.

```sh
go test ./...                        # unit tests, and pipeline tests with stand-in engines (needs FFmpeg)
```

## Adding your own engine

**Engines → Add custom engine** covers most tools: name the program, choose what it upscales
and how it installs, and write its arguments with placeholders. For anything more, write a
manifest and use **Import manifest**. This one is the built-in Real-CUGAN, slightly shortened:

```json
{
  "id": "realcugan-ncnn",
  "name": "Real-CUGAN",
  "vendor": "Bilibili AI Lab",
  "description": "Cascaded U-Net trained on anime.",
  "media": ["image", "video"],
  "input": "image",
  "batch": true,
  "install": {
    "type": "archive",
    "urls": {
      "linux": "https://…/realcugan-ncnn-vulkan-20220728-ubuntu.zip",
      "windows": "https://…/realcugan-ncnn-vulkan-20220728-windows.zip",
      "darwin": "https://…/realcugan-ncnn-vulkan-20220728-macos.zip"
    }
  },
  "executable": {"windows": "realcugan-ncnn-vulkan.exe", "*": "realcugan-ncnn-vulkan"},
  "args": ["-i", "{input}", "-o", "{output}", "-s", "{scale}", "{modelArgs}", "-f", "png"],
  "gpuArgs": ["-g", "{gpu}"],
  "outputExt": ".png",
  "scales": [2, 3, 4],
  "models": [
    {"id": "se", "name": "SE · Conservative", "args": ["-m", "models-se", "-n", "-1"]},
    {"id": "pro", "name": "Pro · Conservative", "args": ["-m", "models-pro", "-n", "-1"], "scales": [2, 3]}
  ]
}
```

| Field | Meaning |
| --- | --- |
| `media` | What the engine upscales: `image`, `video`, `audio`. |
| `input` | What the program reads. An `image` engine listed for `video` runs on every frame; a `video` engine gets the whole file, and TouchUp copies the original audio back in. |
| `batch` | The program accepts folders for `{input}` and `{output}`, so video frames, and the parts of a picture split by `touchup.tileImage`, go through in one run. |
| `serve` | The program can stay running between jobs. Started with `TOUCHUP_SERVE=1` in its environment, it runs the job in its arguments, prints `TOUCHUP-SERVE-DONE <exit code>`, then reads further jobs from standard input, each a JSON array of the command's arguments on one line, until it closes. |
| `install.type` | `archive` (a `.zip` or `.tar.gz` per platform; a single top folder is unwrapped), `pip` (a private virtual environment) or `none` (a program already on the computer). |
| `install.packages`, `install.pip` | For `pip`: requirement specifiers, and further install steps in order, each with `args`, an optional `indexURL` by OS (for PyTorch's CUDA builds on Windows), `os` to limit it to some systems, and `optional` for extras the engine works without. `{engineDir}` works in `args`, so a step can be `["-r", "{engineDir}/src/requirements.txt"]`. |
| `install.source` | For `pip`: a `url` of an archive of the engine's code and the `dir` to unpack it into, before the packages install. |
| `executable` | The program by OS (`linux`, `windows`, `darwin`, or `*`): a path, or a name looked up in the install folder, the Python environment, or `PATH`. |
| `args` | Arguments with placeholders: `{input}`, `{output}`, `{inputDir}`, `{outputDir}`, `{scale}`, `{gpu}`, `{engineDir}`, `{model}`, `{modelDir}`, `{modelPath}` (the model's first file), `{inWidth}`, `{inHeight}`, `{outWidth}`, `{outHeight}` and `{outShort}` (for engines that take a target resolution). An argument that is exactly `{modelArgs}` becomes the selected model's `args`. A Python engine's `executable` can be `python`, its environment's interpreter. |
| `models[].files` | Weights to download into `{modelDir}`: `url`, `name` (shared between models when equal) and an optional `sha256`. Each model then has its own download on the engine card; `size` and `recommended` describe it. |
| `inputRate`, `imageOutputExt` | The sample rate an audio engine needs, and what a video engine writes when given an image. |
| `outputExt` | The extension the program writes. When it picks its own file name, TouchUp takes the newest such file in `{outputDir}`. |
| `progress` | Optional regular expression whose first group is a percentage in the program's output (default: `N%`). |
| `tune` | What **Find the fastest** tries: `seconds` of the user's video to time, and `steps`, each an `option` and the `values` to try in order of cost. |
| `options` | Settings shown on the engine card: `id`, `name`, `description`, `type` (`choice` with `choices` of `value` and `label`, `number` with optional `min` and `max`, or `toggle`), `default`, and `args` with `{value}` (for a toggle, `args` when on and `argsOff` when off). The option `touchup.tileImage` (output pixels a side, `0` for never) makes TouchUp split large pictures for the engine. |
| `progressSteps` | For programs that count parts instead: regular expressions, outermost first, whose two groups are a part's number and the total, such as `"Chunk (\\d+)/(\\d+)"` and `"batch (\\d+)/(\\d+)"`. |

Custom manifests are stored next to the installed engines, in `custom-engines/`.

## Project layout

```
main.go                 window setup and the root view
assets/                 app icon, Inter font, Lucide icons (with their licenses)
tools/icon/             draws assets/icon.png: go run ./tools/icon assets/icon.png
tools/i18n/             lists the messages and checks the translations (see Translations)
internal/
  engine/               manifests, the built-in catalog, install, run, engine servers
    scripts/            runner and launcher scripts shipped to Python engines
  media/                FFmpeg: probe, frames, clips, waveforms, spectrograms, playback
  pipeline/             whole-file upscaling and previews, built on engine + media
  settings/, paths/     preferences and where files live
  proc/                 child processes without console windows
  ui/
    store/              application state and actions
    shell/              sidebar and page switching
    pages/              Studio (with its stages and inspector), Engines, Queue, Settings
    components/         reusable components, one per file
    theme/, draw/       colors, type, buttons, icons; custom painting helpers
    format/             durations, sizes and rates as text, in the user's language
  tr/                   translation: tr.T and tr.N, number formats, the language list
```

## Translations

Messages are their English text: `tr.T("Saved to {folder}", "folder", dir)`, or for plurals
`tr.N("{count} engines installed", n)` with the English forms in `assets/locales/en.json`. A
translation is `assets/locales/<code>.json`, keyed by the English text and embedded in the app;
a message it lacks shows in English. The built-in engines' descriptions, tags, model names and
settings are translated the same way, while custom engines show as their manifests say.

```sh
go run ./tools/i18n -write   # rewrite tools/i18n/template.json from the code and catalog
go run ./tools/i18n          # report missing, unused and mismatched messages per language
```

`tools/i18n/BRIEF.md` is the guide for translators: what to keep, placeholders, and each
language's plural forms. To add a language, translate the template into a new file and add it
to `internal/tr/languages.go`.

## License

TouchUp is licensed under the Apache License 2.0; see [LICENSE](LICENSE). The bundled Inter
font is under the SIL Open Font License (`assets/fonts/OFL.txt`), and the Lucide icons are under
the ISC License (`assets/icons/LICENSE`). Each engine has its own license, shown on its card.
