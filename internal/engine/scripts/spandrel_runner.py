"""TouchUp's runner for PyTorch super-resolution models.

Loads any model spandrel recognizes (ESRGAN, Real-ESRGAN, SwinIR, HAT,
DAT, DRCT, ATD, RealPLKSR, RCAN, ...) and upscales one image, or every
image in a folder, tile by tile so large pictures fit in GPU memory.
Progress is printed as "N%" lines.

    python spandrel_runner.py --model 4x.pth --input in.png --output out.png
    python spandrel_runner.py --model 4x.pth --input frames/ --output upscaled/
"""

import argparse
import json
import math
import os
import sys
import traceback
from collections import deque
from concurrent.futures import ThreadPoolExecutor

import numpy as np
import torch
from PIL import Image

import spandrel

try:  # architectures with non-commercial licenses live in a separate package
    import spandrel_extra_arches

    spandrel_extra_arches.install()
except ImportError:
    pass

IMAGE_EXTS = {".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff"}


def pick_device(gpu):
    if torch.cuda.is_available():
        return torch.device(f"cuda:{gpu}" if gpu >= 0 else "cuda")
    if getattr(torch.backends, "mps", None) and torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def supports(device):
    """Whether this PyTorch build has kernels for a CUDA device: native
    ones for its architecture, or PTX it can compile for it."""
    if device.type != "cuda":
        return True
    i = device.index if device.index is not None else torch.cuda.current_device()
    major, minor = torch.cuda.get_device_capability(i)
    arches = torch.cuda.get_arch_list()
    return f"sm_{major}{minor}" in arches or any(
        a.startswith("compute_") and a[8:].isdigit() and int(a[8:]) <= major * 10 + minor for a in arches)


def describe_device(device):
    if device.type != "cuda":
        return str(device)
    i = device.index if device.index is not None else torch.cuda.current_device()
    major, minor = torch.cuda.get_device_capability(i)
    return f"{torch.cuda.get_device_name(i)} (CUDA {torch.version.cuda}, compute {major}.{minor})"


# What a GPU raises when it can't run a model: CUDA errors are
# RuntimeErrors, and newer PyTorch has its own AcceleratorError.
# FloatingPointError is a model overflowing in reduced precision.
GPU_ERRORS = (RuntimeError, FloatingPointError) + ((torch.AcceleratorError,) if hasattr(torch, "AcceleratorError") else ())


class Upscaler:
    """A loaded model that steps down when the GPU can't run it: from
    reduced to full precision, then from the GPU to the CPU.

    The weights stay in float32 and reduced precision comes from autocast,
    which casts each operation's inputs. Casting the weights instead breaks
    models that build tensors as they run, such as the attention masks of
    DRCT, ATD, HAT and SwinIR, which are float32 whatever the weights are."""

    def __init__(self, path, device, precision="auto"):
        self.model = spandrel.ModelLoader(device=torch.device("cpu")).load_from_file(path)
        if not isinstance(self.model, spandrel.ImageModelDescriptor):
            raise ValueError(f"{os.path.basename(path)} is not an image upscaling model")
        self.model.eval()
        self.place(device, self.best_dtype(device, precision))

    def best_dtype(self, device, precision="auto"):
        if device.type != "cuda" or precision == "full":
            return torch.float32
        if precision == "half":
            return torch.float16
        if precision == "bfloat16":
            return torch.bfloat16
        if device.type == "cuda":
            if self.model.supports_half:
                return torch.float16
            if self.model.supports_bfloat16:
                return torch.bfloat16
        return torch.float32

    def place(self, device, dtype):
        self.device, self.dtype = device, dtype
        self.model.to(device)

    def precision(self):
        return {torch.float16: "half precision", torch.bfloat16: "bfloat16"}.get(self.dtype, "full precision")

    def step_down(self, err):
        """Moves to a safer setting after err, or returns False when there
        is none left."""
        reason = str(err).strip().splitlines()[0] if str(err).strip() else type(err).__name__
        if self.device.type == "cpu":
            return False
        if self.dtype != torch.float32:
            print(f"The GPU failed in {self.precision()} ({reason}); retrying in full precision", flush=True)
            self.place(self.device, torch.float32)
        else:
            print(f"The GPU failed ({reason}); continuing on the CPU, which is much slower", flush=True)
            if self.device.type == "cuda":
                torch.cuda.empty_cache()
            self.place(torch.device("cpu"), torch.float32)
        return True


def pad_to(t, req):
    """Pads a 1xCxHxW tensor to the model's size requirements."""
    _, _, h, w = t.shape
    m = max(1, req.multiple_of)
    ph = max(req.minimum, math.ceil(h / m) * m)
    pw = max(req.minimum, math.ceil(w / m) * m)
    if req.square:
        ph = pw = max(ph, pw)
    if ph == h and pw == w:
        return t
    mode = "reflect" if ph - h < h and pw - w < w else "replicate"
    return torch.nn.functional.pad(t, (0, pw - w, 0, ph - h), mode=mode)


class Progress:
    def __init__(self, total):
        self.total, self.done, self.last = max(1, total), 0, -1

    def step(self):
        self.done += 1
        pct = int(self.done * 100 / self.total)
        if pct != self.last:
            self.last = pct
            print(f"{pct}%", flush=True)


@torch.inference_mode()
def upscale(up, rgb, tile, progress):
    """Upscales an HxWx3 float32 array in [0, 1] by the model's scale."""
    model, device = up.model, up.device
    autocast = torch.autocast(device.type, dtype=up.dtype, enabled=up.dtype != torch.float32)
    scale = model.scale
    h, w, _ = rgb.shape
    src = torch.from_numpy(np.ascontiguousarray(rgb.transpose(2, 0, 1))).unsqueeze(0)
    out = np.zeros((h * scale, w * scale, 3), dtype=np.float32)
    tile = tile if tile > 0 else max(h, w)
    overlap = 16 if tile < max(h, w) else 0
    for y in range(0, h, tile):
        for x in range(0, w, tile):
            y0, x0 = max(0, y - overlap), max(0, x - overlap)
            y1, x1 = min(h, y + tile + overlap), min(w, x + tile + overlap)
            part = src[:, :, y0:y1, x0:x1].to(device=device)
            with autocast:
                res = model(pad_to(part, model.size_requirements))
            res = res[:, :, : (y1 - y0) * scale, : (x1 - x0) * scale]
            if not torch.isfinite(res).all():
                raise FloatingPointError(f"the model's output overflowed in {up.precision()}")
            res = res.float().clamp_(0, 1).squeeze(0).permute(1, 2, 0).cpu().numpy()
            # Keep the tile's own area; the overlap only gave it context.
            ty, tx = (y - y0) * scale, (x - x0) * scale
            th, tw = (min(h, y + tile) - y) * scale, (min(w, x + tile) - x) * scale
            out[y * scale : y * scale + th, x * scale : x * scale + tw] = res[ty : ty + th, tx : tx + tw]
            progress.step()
    return out


def color_fix(out, rgb, scale):
    """Keeps the model's detail but takes color from the original.

    Some models drift on saturated colors. The result's low frequencies
    are swapped for those of the original upscaled smoothly, which leaves
    texture alone and puts every hue back where it was.
    """
    h, w = out.shape[:2]
    k = max(2, scale * 4)
    up = torch.from_numpy(np.ascontiguousarray(out.transpose(2, 0, 1))).unsqueeze(0)
    ref = torch.from_numpy(np.ascontiguousarray(rgb.transpose(2, 0, 1))).unsqueeze(0)
    ref = torch.nn.functional.interpolate(ref, size=(h, w), mode="bicubic", align_corners=False)

    def low(t):
        pooled = torch.nn.functional.avg_pool2d(t, k, stride=k, ceil_mode=True, count_include_pad=False)
        return torch.nn.functional.interpolate(pooled, size=(h, w), mode="bilinear", align_corners=False)

    fixed = (up - low(up) + low(ref)).clamp_(0, 1)
    return fixed.squeeze(0).permute(1, 2, 0).numpy()


def tiles_for(size, tile):
    w, h = size
    if tile <= 0:
        return 1
    return math.ceil(h / tile) * math.ceil(w / tile)


def load(path):
    """Reads an image as an HxWx3 float32 array in [0, 1], and its alpha
    channel when it has one."""
    with Image.open(path) as im:
        alpha = None
        if im.mode in ("RGBA", "LA") or (im.mode == "P" and "transparency" in im.info):
            im = im.convert("RGBA")
            alpha = im.getchannel("A")
        return np.asarray(im.convert("RGB"), dtype=np.float32) / 255.0, alpha


def finish(out, rgb, alpha, up, scale, fix, dst):
    """Color-fixes, resizes and saves one result. Runs on a worker thread
    while the GPU moves on to the next frame."""
    if fix:
        out = color_fix(out, rgb, up.model.scale)
    result = Image.fromarray((out * 255.0).round().astype(np.uint8), "RGB")
    target = (rgb.shape[1] * scale, rgb.shape[0] * scale)
    if scale != up.model.scale:
        result = result.resize(target, Image.LANCZOS)
    if alpha is not None:
        result.putalpha(alpha.resize(result.size, Image.LANCZOS))
    # Light compression: frames are read once more and thrown away, and
    # the default level takes longer than many models.
    result.save(dst, compress_level=1)


def upscale_safely(up, rgb, tile, progress):
    """upscale, retrying with smaller tiles when the GPU runs out of
    memory and with safer settings when it fails."""
    while True:
        done = progress.done
        try:
            return upscale(up, rgb, tile, progress)
        except torch.cuda.OutOfMemoryError:
            progress.done = done
            torch.cuda.empty_cache()
            if tile and tile <= 128:
                raise
            tile = 512 if tile <= 0 else tile // 2
            print(f"Out of GPU memory, retrying with {tile}px tiles", flush=True)
        except GPU_ERRORS as err:
            progress.done = done
            if not up.step_down(err):
                raise


@torch.inference_mode()
def upscale_batch(up, frames):
    """Upscales same-sized frames that each fit in one tile in a single
    pass, which keeps the GPU busy with small, fast models."""
    model = up.model
    scale = model.scale
    h, w, _ = frames[0].shape
    src = torch.from_numpy(np.ascontiguousarray(np.stack(frames).transpose(0, 3, 1, 2)))
    autocast = torch.autocast(up.device.type, dtype=up.dtype, enabled=up.dtype != torch.float32)
    with autocast:
        res = model(pad_to(src.to(device=up.device), model.size_requirements))
    res = res[:, :, : h * scale, : w * scale]
    if not torch.isfinite(res).all():
        raise FloatingPointError(f"the model's output overflowed in {up.precision()}")
    return list(res.float().clamp_(0, 1).permute(0, 2, 3, 1).cpu().numpy())


class Prefetch:
    """Loads images ahead on a worker thread, keeping a few ready."""

    def __init__(self, paths, ahead):
        self.pool = ThreadPoolExecutor(max_workers=2)
        self.paths, self.ahead = paths, ahead
        self.pending = deque(self.pool.submit(load, p) for p in paths[:ahead])
        self.next = ahead

    def get(self):
        item = self.pending.popleft().result()
        if self.next < len(self.paths):
            self.pending.append(self.pool.submit(load, self.paths[self.next]))
            self.next += 1
        return item


def run(up, jobs, scale, tile, fix, batch, progress):
    """Upscales every (source, destination) pair. Frames that fit in one
    tile go through the model batch at a time; loading and saving happen
    on worker threads alongside."""
    loader = Prefetch([src for src, _ in jobs], max(2, batch * 2))
    saver = ThreadPoolExecutor(max_workers=max(2, min(8, os.cpu_count() or 2)))
    saving = deque()

    def save(*args):
        saving.append(saver.submit(finish, *args))
        while len(saving) > batch * 4:  # bounded, so memory stays flat
            saving.popleft().result()

    i, carry = 0, None
    while i < len(jobs) or carry:
        if carry:
            group, carry = [carry], None
        else:
            group = [(*loader.get(), jobs[i][1])]
            i += 1
        shape = group[0][0].shape
        whole = tile <= 0 or max(shape[:2]) <= tile
        while whole and batch > 1 and len(group) < batch and i < len(jobs):
            item = (*loader.get(), jobs[i][1])
            i += 1
            if item[0].shape != shape:  # a different size starts the next group
                carry = item
                break
            group.append(item)
        if len(group) > 1:
            try:
                outs = upscale_batch(up, [r for r, _, _ in group])
                for _ in group:
                    progress.step()
            except torch.cuda.OutOfMemoryError:
                torch.cuda.empty_cache()
                batch = max(1, len(group) // 2)
                print(f"Out of GPU memory, retrying with batches of {batch}", flush=True)
                outs = [upscale_safely(up, r, tile, progress) for r, _, _ in group]
            except GPU_ERRORS as err:
                if not up.step_down(err):
                    raise
                outs = [upscale_safely(up, r, tile, progress) for r, _, _ in group]
        else:
            outs = [upscale_safely(up, group[0][0], tile, progress)]
        for out, (r, a, d) in zip(outs, group):
            save(out, r, a, up, scale, fix, d)
    for f in saving:
        f.result()
    saver.shutdown()
    loader.pool.shutdown()


def explain(err):
    """A one-line reason for a failure, with what to do where it's known."""
    msg = str(err).strip().splitlines()[0] if str(err).strip() else type(err).__name__
    low = msg.lower()
    if "no kernel image" in low or "not compatible with the current pytorch" in low:
        return msg + ". This GPU isn't supported by the installed PyTorch build."
    if isinstance(err, torch.cuda.OutOfMemoryError) or "out of memory" in low:
        return "The GPU ran out of memory even with small tiles. Try a smaller model or close other GPU apps."
    if isinstance(err, FileNotFoundError):
        return f"File not found: {err.filename}"
    return f"{type(err).__name__}: {msg}"


def parse(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", required=True)
    ap.add_argument("--input", required=True, help="an image or a folder of images")
    ap.add_argument("--output", required=True, help="an image or a folder")
    ap.add_argument("--scale", type=int, default=0, help="output scale; defaults to the model's")
    ap.add_argument("--tile", type=int, default=512, help="tile size in pixels; 0 for none")
    ap.add_argument("--gpu", type=int, default=-1)
    ap.add_argument("--no-color-fix", action="store_true", help="keep the model's own colors")
    ap.add_argument("--batch", type=int, default=1, help="frames per pass, for frames that fit in one tile")
    ap.add_argument("--precision", choices=["auto", "half", "bfloat16", "full"], default="auto")
    return ap.parse_args(argv)


def load_model(args):
    device = pick_device(args.gpu)
    if device.type == "cuda":
        torch.backends.cudnn.benchmark = True  # video frames share a size: tune once
    if not supports(device):
        print(f"This PyTorch (CUDA {torch.version.cuda}) has no kernels for {describe_device(device)}; "
              "running on the CPU. Reinstall the engine to get a PyTorch for this GPU.", flush=True)
        device = torch.device("cpu")
    up = Upscaler(args.model, device, args.precision)
    print(f"Loaded {up.model.architecture.name} {up.model.scale}x on {describe_device(up.device)} "
          f"in {up.precision()}, PyTorch {torch.__version__}", flush=True)
    return up


def run_job(up, args):
    scale = args.scale or up.model.scale
    if os.path.isdir(args.input):
        os.makedirs(args.output, exist_ok=True)
        names = sorted(n for n in os.listdir(args.input) if os.path.splitext(n)[1].lower() in IMAGE_EXTS)
        jobs = [(os.path.join(args.input, n), os.path.join(args.output, os.path.splitext(n)[0] + ".png")) for n in names]
    else:
        jobs = [(args.input, args.output)]
    if not jobs:
        return
    # Video frames all share a size; reading every header would be slow.
    with Image.open(jobs[0][0]) as im:
        first = im.size
    progress = Progress(len(jobs) * tiles_for(first, args.tile))
    run(up, jobs, scale, args.tile, not args.no_color_fix, max(1, args.batch), progress)


def main():
    """Runs the job in the arguments. With TOUCHUP_SERVE=1, it then takes
    more from standard input, a JSON array of the command's arguments a
    line, ending each with "TOUCHUP-SERVE-DONE <exit code>", so the model
    loads once for every segment of a video."""
    serving = os.environ.get("TOUCHUP_SERVE") == "1"
    argv, up, key = sys.argv[1:], None, None
    while True:
        code = 0
        try:
            args = parse(argv)
            k = (args.model, args.gpu, args.precision)
            if up is None or k != key:
                up, key = load_model(args), k
            run_job(up, args)
        except SystemExit as e:
            code = e.code if isinstance(e.code, int) else (0 if e.code is None else 1)
        except KeyboardInterrupt:
            sys.exit(130)
        except Exception as err:  # the last line is what TouchUp shows
            traceback.print_exc()
            print(f"Error: {explain(err)}", flush=True)
            code = 1
        if not serving:
            sys.exit(code)
        print(f"TOUCHUP-SERVE-DONE {code}", flush=True)
        if code != 0:
            sys.exit(code)
        line = sys.stdin.readline()
        if not line:
            return
        argv = json.loads(line)[1:]  # the command's arguments, after this script


if __name__ == "__main__":
    main()
