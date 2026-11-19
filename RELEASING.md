# Releasing TouchUp

Publishing a GitHub release with a tag like `v0.1.0` runs
`.github/workflows/release.yml`. The tag sets the version. The workflow runs
`go vet` and `go test`, builds every package and attaches them to the
release. The workflow is written by the Giode CLI from `giode.toml`: after
changing `[ci]` or a store section, run `giode ci -f` instead of editing it,
then redo the two hand edits noted in `[ci]` (pin the CLI to
`@v0.1.35-alpha.1`, `go-version-file: go.mod`).

| Where | What the release gets | Needs |
| --- | --- | --- |
| GitHub, Linux | `.deb` and `.rpm`, x64 | nothing |
| GitHub, Windows | `TouchUp-<version>-amd64-setup.exe` | nothing; a code signing certificate stops SmartScreen warnings |
| GitHub, macOS | a universal `.dmg`, signed and notarized | a Developer ID certificate, or macOS refuses to open it |
| Microsoft Store | `TouchUp-<version>-amd64.msix` to upload in Partner Center | a Partner Center account and the reserved name |

There is no Mac App Store, Google Play or iOS App Store build; see
[Stores TouchUp isn't in](#stores-touchup-isnt-in).

## Repository secrets

Set these in GitHub under Settings → Secrets and variables → Actions. Each
`*_BASE64` secret is the file encoded with `base64 -i file | pbcopy` on macOS,
or `base64 -w0 file` on Linux. Without a secret, that step is skipped and the
rest of the release still builds (unsigned).

| Secret | What it is |
| --- | --- |
| `MACOS_CERT_BASE64`, `MACOS_CERT_PASS` | The "Developer ID Application" certificate as a `.p12`, and its password: signs the `.dmg` |
| `GIODE_NOTARY_ID`, `GIODE_NOTARY_TEAM_ID`, `GIODE_NOTARY_PASS` | Your Apple ID, team ID and an app-specific password: notarizes the `.dmg` |
| `WINDOWS_CERT_BASE64`, `GIODE_WINDOWS_CERT_PASS` | An Authenticode `.pfx` and its password: signs the Windows installer (optional; the Store signs the MSIX itself) |

## macOS (.dmg on GitHub)

1. Join the Apple Developer Program.
2. In Certificates, Identifiers & Profiles, create a "Developer ID
   Application" certificate, export it from Keychain Access as a `.p12`, and
   set `MACOS_CERT_BASE64` and `MACOS_CERT_PASS`.
3. At appleid.apple.com create an app-specific password, and set
   `GIODE_NOTARY_ID` (your Apple ID), `GIODE_NOTARY_TEAM_ID` (the team ID) and
   `GIODE_NOTARY_PASS`.
4. Publish a release. The `.dmg` is signed with the hardened runtime and
   notarized. Engines TouchUp downloads aren't quarantined (it downloads them
   itself, not through a browser), so Gatekeeper lets them run.

## Microsoft Store

1. Use the Partner Center developer account that already publishes EasySend
   and AnyFile (publisher `CN=B40E8F57-A7E0-40BB-8E74-5D051D0B578B`, Mehran
   Zahiroddini).
2. Apps and games → New product → MSIX or PWA app, and reserve the name
   "TouchUp". If it's taken, reserve a longer one (e.g. "TouchUp AI
   Upscaler") and make the release workflow's MSIX build use it, as AnyFile's
   does.
3. In the product's Product identity page, check these against
   `[windows.msix]` in `giode.toml` and correct them there if they differ:
   - **Package/Identity/Name** → `identity_name` (expected
     `MehranZahir.TouchUp`).
   - **Package/Identity/Publisher** → `publisher`.
   - **Package/Properties/PublisherDisplayName** → `publisher_display_name`.
4. Commit and publish a release. It gets `TouchUp-<version>-amd64.msix`;
   upload it in a Partner Center submission. The Store signs it, so it needs
   no certificate here.
5. **Restricted capability `runFullTrust`:** explain in the submission notes
   that TouchUp is a desktop app that runs FFmpeg and the open-source
   upscaling engines the user chooses to install, as separate processes.
6. **Notes for certification** (paste): "TouchUp needs FFmpeg, which the user
   installs (winget install ffmpeg) or points to in Settings. Without FFmpeg
   the app opens and explains what's missing. To test without a GPU: install
   FFmpeg, open any image, choose the built-in Classic engine and press
   Upscale. AI engines are optional downloads from their authors' GitHub and
   Hugging Face pages, listed in the Engines tab."

### Store listing (Microsoft Store)

- **Name:** TouchUp
- **Short description** (≤80): Upscale images, video and audio with AI engines that run on your PC.
- **Description:**

  > TouchUp upscales images, video and audio with open-source AI engines that run on your own computer. Nothing is uploaded.
  >
  > Install engines with one click from a built-in catalog: Real-ESRGAN, Real-CUGAN, waifu2x and RealSR on any Vulkan GPU; SeedVR2, FlashVSR, RealBasicVSR and 13 PyTorch models through spandrel on NVIDIA GPUs; and AudioSR, Apollo, VoiceFixer, Resemble Enhance and DeepFilterNet for sound. Two engines are built in and need no install: Classic, an edge-adaptive scaler with sharpening that runs instantly on the CPU, and RTX Video, NVIDIA's RTX Video Super Resolution from the graphics driver. Add any other command-line upscaler from a form or a manifest.
  >
  > Preview before you commit: scrub a video to any frame and compare the original with the result in a split view with zoom, or enhance a few seconds of audio and compare spectrograms. Upscale by 2x, 3x or 4x, or straight to 720p, 1080p, 1440p or 4K. Long videos go through a segment at a time, so the work folder stays small, and the original sound is kept.
  >
  > Clean up the source with deinterlace, deblock, denoise and deband filters, and finish with sharpening, color and film grain. Jobs run in a queue with live progress, and every error has a Copy button with a full report.
  >
  > In 26 languages, with dark and light themes.
  >
  > Requires FFmpeg (free, installed separately). AI engines need a GPU with Vulkan; the diffusion engines need an NVIDIA GPU with 8 GB or more.

- **Features** (up to 20): One-click install of open-source AI engines · Images, video and audio · Split-view preview on a single video frame · Audio preview with spectrograms · Scale by 2x–4x or to 720p–4K · Built-in Classic and RTX Video engines · Clean-up filters and color look · Job queue with progress · Add your own command-line engines · Nothing is uploaded
- **Search terms** (up to 7): upscale, upscaler, AI upscaling, video enhancer, super resolution, Real-ESRGAN, audio restoration
- **Category:** Photo & video (subcategory none).
- **System requirements:** 64-bit Windows 10 1809 or later; DirectX 11 GPU. Recommended: Vulkan GPU, NVIDIA RTX for RTX Video and the diffusion engines; 16 GB memory.
- **Privacy policy URL:** https://xmhrnx.github.io/touchup/#privacy
- **Website / support:** https://xmhrnx.github.io/touchup/ · https://github.com/xmhrnx/touchup/issues
- **Age rating (IARC questionnaire):** category "Utility, productivity, communication or other". Answer No to violence, sexual content, language, controlled substances, gambling; No to user interaction or sharing content between users; No to sharing location; No to digital purchases; No to unrestricted web access (it downloads engines from fixed pages, it isn't a browser). Expected rating: Everyone / 3+.
- **Data collection:** none. TouchUp sends its developer nothing (see the privacy policy).
- **Screenshots:** the website's `site/img/studio-compare.webp` (split-view compare), `studio-light.webp` (light theme) and `engines.webp` (engine catalog) show the right screens, but they're 1360×860 and the Store wants at least 1366×768 as PNG. Retake them with the window at 1920×1080 (or 1366×860+), same screens, as PNG.

## Stores TouchUp isn't in

- **Mac App Store.** Every App Store app runs in the App Sandbox, and TouchUp's
  whole job is running other programs: FFmpeg, which every job goes through
  (images too), the ncnn engines it downloads, and the Python and pip packages
  it installs for the PyTorch engines. A sandboxed app can't run the user's
  FFmpeg, and guideline 2.5.2 forbids downloading code that adds features.
  AnyFile's way out (built-in engines only in the sandbox) leaves TouchUp with
  nothing that works, since even Classic needs FFmpeg. Bundling FFmpeg and
  every engine would be a rewrite and a multi-gigabyte app. Mac users get the
  notarized `.dmg`, which has none of these limits.
- **Google Play and the iOS App Store.** TouchUp is desktop-only
  (`app.platforms`): Android and iOS can't run its engines, FFmpeg or Python.

## Known Microsoft Store limitations

- The MSIX build is a full-trust desktop app, and Windows redirects its
  writes to `%LOCALAPPDATA%\TouchUp` into the package's own folder
  (`%LOCALAPPDATA%\Packages\MehranZahir.TouchUp_…\LocalCache\Local\TouchUp`).
  Engines, Python and models live there and are removed on uninstall; they
  don't carry over from the GitHub installer. The longer path can push deep
  Python packages past 260 characters on systems without long paths turned
  on: if a PyTorch engine install fails with a path error, the fix is the
  Windows "Enable Win32 long paths" setting.
- FFmpeg isn't included: users install it themselves, as with the GitHub
  download. Say so in the listing (it's in the description above).
- File associations ("Open with TouchUp") come from the MSIX manifest.

## Linux

The `.deb` and `.rpm` are attached to the release. They list the system
libraries they need, so `apt` and `dnf` install those too.
