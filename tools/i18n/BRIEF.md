# Translating TouchUp

TouchUp is a desktop app that upscales images, video and audio with local AI
engines (Real-ESRGAN, SeedVR2, PyTorch SR, AudioSR...). Its messages are in
`tools/i18n/template.json`: an object whose keys are the English text. A
translation is `assets/locales/<code>.json` with the same keys and the
translated text as values.

## Rules

- Keys stay exactly as in the template, byte for byte. Translate the values.
- Every message must be translated, except text that reads the same in your
  language (then copy it).
- Keep every `{placeholder}` exactly, with the same name; you may move it.
  `{engine}`, `{model}`, `{file}`, `{program}` are names; `{time}`, `{size}`,
  `{n}`, `{count}`, `{frames}` and the like are already-formatted numbers
  and durations. Placeholders in braces that aren't filled (such as
  `{input}` and `{output}` in the engine form) must stay as they are.
- Plural messages have object values in the template (`{"one": ..., "other": ...}`).
  Give the forms your language uses (see below); `other` is always needed.
- Keep product, engine, model-architecture, file-format and technical names
  as they are: TouchUp, SeedVR2, PyTorch, PyTorch SR, Real-ESRGAN, Real-CUGAN,
  waifu2x, RealSR, AudioSR, Apollo, VoiceFixer, Resemble Enhance,
  DeepFilterNet, FFmpeg, FFprobe, FFplay, Python, pip, CUDA, cuDNN, NVIDIA,
  Vulkan, VAE, DiT, GAN, CNN, DRCT, DAT, ATD, HAT, SwinIR, RealPLKSR, RCAN,
  ESRGAN, GGUF, FP8, bfloat16, H.264, H.265, ProRes, PNG, JPEG, WebP, WAV,
  FLAC, MP3, MP4, CRF, LAB, HSV, AdaIN, TTA. Translate the ordinary words
  around them ("General · x4plus": translate "General").
  GPU and VRAM: use the abbreviation your language's software uses (usually
  GPU itself).
- Messages starting with a lowercase letter are reasons shown after a colon,
  like "Couldn't install SeedVR2: download failed: 404". Start them lowercase
  if your language allows.
- Keep trailing "…" and "." as in the English. Keep " · " separators.
- Units: `{s}s`, `{m}m {s}s`, `{h}h {m}m`, `{m} min` are compact durations
  ("42s", "3m 05s"); write them with your language's usual short units.
  `{n} KB` ... `{n} TB`, `{n} kHz`, `{n} fps`, `{n} B`: use your language's
  unit symbols and spacing (French writes Ko/Mo/Go). `{n}%`: your language's
  percent spacing (German and French put a space before %).
- Style: this is desktop software. Buttons and menu items are short commands
  in the form your language's desktop software uses (infinitive or
  imperative, as on Windows and macOS in your language). Where the text
  addresses the user, use the form of address current mainstream desktop
  software in your language uses, consistently throughout.
- "Upscale" means raising a picture's resolution with AI; use the term
  established in your language's photo and video software. "Engine" is an
  upscaling program; "model" is a set of AI weights; "preview" is a quick
  test on one frame or a few seconds of audio; "queue" is the list of jobs.
- Keep the length close to the English where you can: buttons and labels
  sit in tight spaces.
- Write valid JSON in UTF-8, two-space indent, keys in the template's order.

## Plural forms TouchUp asks for

| Language | Forms |
| --- | --- |
| bg, da, de, el, es, et, fi, hu, it, nb, nl, sv | one (1), other |
| fr, pt | one (0 and 1), other |
| cs, sk | one (1), few (2-4), other |
| lt | one (1, 21, 31...), few (2-9, 22-29...), other (0, 10-20...) |
| lv | zero (0, 10-20, 30...), one (1, 21, 31...), other |
| pl | one (1), few (2-4, 22-24...), many (0, 5-21, 25-31...), other (same as many) |
| hr, uk | one (1, 21, 31...), few (2-4, 22-24...), many (0, 5-20, 25-30...), other (same as many) |
| ro | one (1), few (0, 2-19, 102-119...), other |
| sl | one (1, 101...), two (2, 102...), few (3-4, 103-104...), other |
| ga | one (1), two (2), few (3-6), many (7-10), other |
| mt | one (1), two (2), few (0, 3-10, 103-110...), many (11-19, 111-119...), other |

## Checking

From the repository root, `go run ./tools/i18n` lists, per language, missing
messages, messages that aren't in the template, and translations whose
placeholders differ from the English. Fix what it reports for your files.
