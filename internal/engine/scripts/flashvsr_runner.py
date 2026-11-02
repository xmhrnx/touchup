"""TouchUp's runner for FlashVSR, a one-step diffusion video upscaler.

Runs FlashVSR v1.1 through lihaoyun6's ComfyUI-FlashVSR_Ultra_Fast
pipeline, which swaps the original's compiled Block-Sparse-Attention for
Triton kernels ("sparse SageAttention") that work on RTX 20 to 50 cards.
Upscales a folder of video frames, or one image, and prints progress as
"N%" lines.

    python flashvsr_runner.py --code flashvsr --models models --input frames/ --output upscaled/ --scale 4

The video goes through in chunks of frames that share a few frames of
context with the chunk before, so memory stays flat on long videos and
motion carries over from chunk to chunk. Each chunk is upscaled in
overlapping tiles (--tile), which is what lets it fit in 8–12 GB.

With TOUCHUP_SERVE=1 it stays running after the job and takes more from
standard input, a JSON array of the command's arguments a line, ending
each with "TOUCHUP-SERVE-DONE <exit code>", so the models load once.
"""

import argparse
import json
import math
import os
import sys
import traceback
from concurrent.futures import ThreadPoolExecutor

import numpy as np
import torch
import torch.nn.functional as F
from PIL import Image

IMAGE_EXTS = {".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff"}

# Frames from the chunk before that a chunk starts with, for motion to
# carry over; their results are dropped.
CONTEXT = 8


def parse(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--code", required=True, help="the ComfyUI-FlashVSR_Ultra_Fast folder")
    ap.add_argument("--models", required=True, help="the folder with FlashVSR v1.1's files")
    ap.add_argument("--input", required=True, help="a folder of frames, or an image")
    ap.add_argument("--output", required=True, help="a folder, or an image")
    ap.add_argument("--scale", type=int, default=4, choices=[2, 4])
    ap.add_argument("--mode", default="tiny", choices=["tiny", "full"],
                    help="tiny decodes with FlashVSR's small decoder; full with the Wan VAE")
    ap.add_argument("--tile", type=int, default=256, help="tile size in input pixels; 0 for none")
    ap.add_argument("--tile-overlap", type=int, default=24)
    ap.add_argument("--chunk", type=int, default=64, help="frames upscaled at a time")
    ap.add_argument("--tiled-vae", action="store_true", help="decode in tiles, for less GPU memory")
    ap.add_argument("--unload-dit", action="store_true", help="move the DiT off the GPU while decoding")
    ap.add_argument("--sparse-ratio", type=float, default=2.0)
    ap.add_argument("--kv-ratio", type=float, default=3.0)
    ap.add_argument("--local-range", type=int, default=11, choices=[9, 11])
    ap.add_argument("--precision", default="bf16", choices=["bf16", "fp16"])
    ap.add_argument("--no-color-fix", action="store_true")
    ap.add_argument("--seed", type=int, default=0)
    ap.add_argument("--gpu", type=int, default=-1)
    return ap.parse_args(argv)


def pick_device(gpu):
    if torch.cuda.is_available():
        return f"cuda:{gpu}" if gpu >= 0 else "cuda:0"
    if getattr(torch.backends, "mps", None) and torch.backends.mps.is_available():
        return "mps"
    raise RuntimeError("FlashVSR needs a GPU: no CUDA or Apple GPU was found")


def load_pipeline(args):
    """Builds the pipeline as the ComfyUI node's init_pipeline does,
    from TouchUp's model folder."""
    sys.path.insert(0, os.path.abspath(args.code))
    from src import FlashVSRFullPipeline, FlashVSRTinyPipeline, ModelManager
    from src.models import wan_video_dit
    from src.models.TCDecoder import build_tcdecoder
    from src.models.utils import Causal_LQ4x_Proj

    wan_video_dit.USE_BLOCK_ATTN = False  # the Triton kernels, not the compiled library
    device = pick_device(args.gpu)
    if device.startswith("cuda"):
        torch.cuda.set_device(device)
    dtype = torch.bfloat16 if args.precision == "bf16" else torch.float16

    def need(name):
        path = os.path.join(args.models, name)
        if not os.path.exists(path):
            raise FileNotFoundError(path)
        return path

    ckpt = need("diffusion_pytorch_model_streaming_dmd.safetensors")
    mm = ModelManager(torch_dtype=dtype, device="cpu")
    if args.mode == "full":
        mm.load_models([ckpt, need("Wan2.1_VAE.pth")])
        pipe = FlashVSRFullPipeline.from_model_manager(mm, device=device)
        pipe.vae.model.encoder = None
        pipe.vae.model.conv1 = None
    else:
        mm.load_models([ckpt])
        pipe = FlashVSRTinyPipeline.from_model_manager(mm, device=device)
        pipe.TCDecoder = build_tcdecoder(new_channels=[512, 256, 128, 128], device=device, dtype=dtype, new_latent_channels=16 + 768)
        pipe.TCDecoder.load_state_dict(torch.load(need("TCDecoder.ckpt"), map_location=device), strict=False)
        pipe.TCDecoder.clean_mem()
    # v1.1 uses the causal projection (v1 used Buffer_LQ4x_Proj).
    proj = Causal_LQ4x_Proj(in_dim=3, out_dim=1536, layer_num=1).to(device, dtype=dtype)
    proj.load_state_dict(torch.load(need("LQ_proj_in.ckpt"), map_location="cpu"), strict=True)
    pipe.denoising_model().LQ_proj_in = proj.to(device)
    pipe.to(device, dtype=dtype)
    pipe.enable_vram_management(num_persistent_param_in_dit=None)
    pipe.init_cross_kv(prompt_path=os.path.join(args.code, "posi_prompt.pth"))
    pipe.load_models_to_device(["dit", "vae"])
    pipe.offload_model()
    name = torch.cuda.get_device_name(device) if device.startswith("cuda") else device
    print(f"Loaded FlashVSR v1.1 ({args.mode}) on {name} in {args.precision}, PyTorch {torch.__version__}", flush=True)
    return pipe


def largest_8n1_leq(n):
    return 0 if n < 1 else ((n - 1) // 8) * 8 + 1


def next_8n5(n):
    return 21 if n < 21 else ((n - 5 + 7) // 8) * 8 + 5


def run_pipe(pipe, args, frames, report):
    """Upscales frames, an N×H×W×3 float tensor in [0, 1] whose sides
    times the scale are multiples of 128, as the node's flashvsr() does
    for one tile. Returns N×H'×W'×3 in [0, 1] on the CPU."""
    n, h, w, _ = frames.shape
    frames = torch.cat([frames, frames[-1:].repeat(next_8n5(n) - n, 1, 1, 1)], 0)
    count = largest_8n1_leq(frames.shape[0] + 4)
    th, tw = h * args.scale, w * args.scale
    dtype, device = pipe.torch_dtype, pipe.device
    lq = []
    for i in range(count):
        f = frames[min(i, frames.shape[0] - 1)].to(device).permute(2, 0, 1).unsqueeze(0)
        f = F.interpolate(f, size=(th, tw), mode="bicubic", align_corners=False)
        lq.append(f.squeeze(0).to("cpu", dtype) * 2.0 - 1.0)
    lq = torch.stack(lq, 0).permute(1, 0, 2, 3).unsqueeze(0).to(device)

    def steps(it, **_):
        items = list(it)
        for i, x in enumerate(items):
            yield x
            report((i + 1) / len(items))

    video = pipe(
        prompt="", negative_prompt="", cfg_scale=1.0, num_inference_steps=1, seed=args.seed,
        tiled=args.tiled_vae, progress_bar_cmd=steps, LQ_video=lq, num_frames=count, height=th, width=tw,
        is_full_block=False, if_buffer=True, topk_ratio=args.sparse_ratio * 768 * 1280 / (th * tw),
        kv_ratio=args.kv_ratio, local_range=args.local_range, color_fix=not args.no_color_fix,
        unload_dit=args.unload_dit, force_offload=False,
    )
    out = ((video.squeeze(0).permute(1, 2, 3, 0).float() + 1.0) / 2.0).clamp_(0, 1).cpu()
    del video, lq
    return out[:n]


def tile_starts(n, tile, overlap):
    if tile <= 0 or n <= tile:
        return [0]
    out, step = [], tile - overlap
    x = 0
    while x + tile < n:
        out.append(x)
        x += step
    return out + [n - tile]


def feather(size, ramp):
    """A weight that rises over ramp pixels at each edge, for blending
    overlapping tiles."""
    w = torch.ones(size)
    if ramp > 0:
        r = torch.linspace(0, 1, ramp + 2)[1:-1]
        w[:ramp] = torch.minimum(w[:ramp], r)
        w[-ramp:] = torch.minimum(w[-ramp:], r.flip(0))
    return w


def upscale(pipe, args, frames, report):
    """Upscales N×H×W×3 frames in [0, 1] by args.scale, in tiles."""
    n, h, w, _ = frames.shape
    s = args.scale
    # FlashVSR wants sides that, upscaled, are multiples of 128. The node
    # crops to get them, losing the picture's edges; here the frames are
    # padded instead and the padding is cut off the result.
    m = 128 // s
    ph, pw = math.ceil(h / m) * m, math.ceil(w / m) * m
    tile = args.tile if args.tile > 0 else max(ph, pw)
    tile = max(m, tile // m * m)
    if ph > h or pw > w:
        x = frames.permute(0, 3, 1, 2)
        x = F.pad(x, (0, pw - w, 0, ph - h), mode="replicate")
        frames = x.permute(0, 2, 3, 1)
    ys, xs = tile_starts(ph, tile, args.tile_overlap), tile_starts(pw, tile, args.tile_overlap)
    if len(ys) == 1 and len(xs) == 1:
        return run_pipe(pipe, args, frames, report)[:, : h * s, : w * s]

    out = torch.zeros((n, ph * s, pw * s, 3), dtype=torch.float16)
    weight = torch.zeros((ph * s, pw * s, 1), dtype=torch.float32)
    ramp = args.tile_overlap * s
    total = len(ys) * len(xs)
    for i, (y, x) in enumerate((y, x) for y in ys for x in xs):
        th, tw = min(tile, ph), min(tile, pw)
        part = run_pipe(pipe, args, frames[:, y : y + th, x : x + tw], lambda f, i=i: report((i + f) / total))
        wy = feather(th * s, ramp if len(ys) > 1 else 0)
        wx = feather(tw * s, ramp if len(xs) > 1 else 0)
        wt = (wy[:, None] * wx[None, :])[:, :, None]
        out[:, y * s : (y + th) * s, x * s : (x + tw) * s] += (part * wt).half()
        weight[y * s : (y + th) * s, x * s : (x + tw) * s] += wt
        del part
    out = (out.float() / weight.clamp_(min=1e-6)).clamp_(0, 1)
    return out[:, : h * s, : w * s]


def load(path):
    with Image.open(path) as im:
        return np.asarray(im.convert("RGB"), dtype=np.float32) / 255.0


def save(arr, path):
    Image.fromarray((arr * 255.0).round().astype(np.uint8), "RGB").save(path, compress_level=1)


def run_job(pipe, args):
    if os.path.isdir(args.input):
        os.makedirs(args.output, exist_ok=True)
        names = sorted(n for n in os.listdir(args.input) if os.path.splitext(n)[1].lower() in IMAGE_EXTS)
        jobs = [(os.path.join(args.input, n), os.path.join(args.output, os.path.splitext(n)[0] + ".png")) for n in names]
    else:
        jobs = [(args.input, args.output)]
    if not jobs:
        return

    last = [-1]

    def report(f):
        pct = int(f * 100)
        if pct != last[0]:
            last[0] = pct
            print(f"{pct}%", flush=True)

    chunk = max(9, args.chunk)
    chunks = math.ceil(len(jobs) / chunk)
    pool = ThreadPoolExecutor(max_workers=max(2, min(8, os.cpu_count() or 2)))
    try:
        for c in range(chunks):
            start, end = c * chunk, min(len(jobs), (c + 1) * chunk)
            first = max(0, start - CONTEXT)
            frames = torch.from_numpy(np.stack(list(pool.map(load, [src for src, _ in jobs[first:end]]))))
            out = upscale(pipe, args, frames, lambda f: report((c + f) / chunks))
            if len(jobs) == 1:
                # One picture was repeated to make a clip; every result
                # frame is that picture, and their median is cleanest.
                out = out.median(dim=0).values.unsqueeze(0)
            saving = [pool.submit(save, out[i - first].numpy(), dst) for i, (_, dst) in enumerate(jobs[start:end], start)]
            for s in saving:
                s.result()
            del frames, out
    finally:
        pool.shutdown()
        pipe.offload_model()
        if torch.cuda.is_available():
            torch.cuda.empty_cache()


def explain(err):
    msg = str(err).strip().splitlines()[0] if str(err).strip() else type(err).__name__
    low = msg.lower()
    if isinstance(err, torch.cuda.OutOfMemoryError) or "out of memory" in low:
        return "The GPU ran out of memory. Use smaller tiles, fewer frames at a time, or turn on tiled decoding."
    if isinstance(err, FileNotFoundError):
        return f"File not found: {err.filename or msg}"
    if "triton" in low or "ptxas" in low:
        return f"{msg}. FlashVSR's attention needs Triton, which didn't work on this computer."
    return f"{type(err).__name__}: {msg}"


def main():
    serving = os.environ.get("TOUCHUP_SERVE") == "1"
    argv = sys.argv[1:]
    pipe, key = None, None
    while True:
        code = 0
        try:
            args = parse(argv)
            # The models depend on these; the server is restarted by
            # TouchUp when they change, but check anyway.
            k = (args.code, args.models, args.mode, args.precision, args.gpu)
            if pipe is None or k != key:
                pipe, key = load_pipeline(args), k
            run_job(pipe, args)
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
