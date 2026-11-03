"""TouchUp's runner for RealBasicVSR, a 4x video upscaler that recovers
detail from neighboring frames.

RealBasicVSR (Chan et al., CVPR 2022, Apache-2.0) first cleans each frame
of noise, blur and compression, then estimates motion between frames
(SPyNet optical flow), lines up every frame's features with those before
and after it, and merges them, so detail that moving pictures show over
several frames comes back. The network below is OpenMMLab's code made
self-contained, so its checkpoint loads without mmcv or mmagic.

    python realbasicvsr_runner.py --model realbasicvsr.pth --input frames/ --output upscaled/

Frames go through in chunks of --chunk that overlap by a few frames on
each side, so memory stays flat and chunks join without seams in time;
big frames go in overlapping tiles of --tile pixels.

With TOUCHUP_SERVE=1 it stays running after the job and takes more from
standard input, a JSON array of the command's arguments a line, ending
each with "TOUCHUP-SERVE-DONE <exit code>", so the model loads once.
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
import torch.nn as nn
import torch.nn.functional as F
from PIL import Image

IMAGE_EXTS = {".png", ".jpg", ".jpeg", ".webp", ".bmp", ".tif", ".tiff"}

# Frames on each side of a chunk that only give it context.
CHUNK_PAD = 2


# --- The network, as in OpenMMLab's mmagic, with the same module names. ---


def flow_warp(x, flow, padding_mode="zeros"):
    """Warps x (n, c, h, w) by flow (n, h, w, 2), in pixels."""
    _, _, h, w = x.size()
    grid_y, grid_x = torch.meshgrid(
        torch.arange(0, h, device=flow.device, dtype=x.dtype),
        torch.arange(0, w, device=flow.device, dtype=x.dtype),
        indexing="ij")
    grid = torch.stack((grid_x, grid_y), 2)
    grid_flow = grid + flow
    grid_flow_x = 2.0 * grid_flow[:, :, :, 0] / max(w - 1, 1) - 1.0
    grid_flow_y = 2.0 * grid_flow[:, :, :, 1] / max(h - 1, 1) - 1.0
    grid_flow = torch.stack((grid_flow_x, grid_flow_y), dim=3).type(x.type())
    return F.grid_sample(x, grid_flow, mode="bilinear", padding_mode=padding_mode, align_corners=True)


class ResidualBlockNoBN(nn.Module):
    def __init__(self, mid_channels=64):
        super().__init__()
        self.conv1 = nn.Conv2d(mid_channels, mid_channels, 3, 1, 1, bias=True)
        self.conv2 = nn.Conv2d(mid_channels, mid_channels, 3, 1, 1, bias=True)
        self.relu = nn.ReLU(inplace=True)

    def forward(self, x):
        return x + self.conv2(self.relu(self.conv1(x)))


class ResidualBlocksWithInputConv(nn.Module):
    def __init__(self, in_channels, out_channels=64, num_blocks=30):
        super().__init__()
        self.main = nn.Sequential(
            nn.Conv2d(in_channels, out_channels, 3, 1, 1, bias=True),
            nn.LeakyReLU(negative_slope=0.1, inplace=True),
            nn.Sequential(*[ResidualBlockNoBN(out_channels) for _ in range(num_blocks)]))

    def forward(self, feat):
        return self.main(feat)


class PixelShufflePack(nn.Module):
    def __init__(self, in_channels, out_channels, scale_factor, upsample_kernel):
        super().__init__()
        self.scale_factor = scale_factor
        self.upsample_conv = nn.Conv2d(in_channels, out_channels * scale_factor * scale_factor,
                                       upsample_kernel, padding=(upsample_kernel - 1) // 2)

    def forward(self, x):
        return F.pixel_shuffle(self.upsample_conv(x), self.scale_factor)


class ConvModule(nn.Module):
    """mmcv's ConvModule without normalization: a convolution and an
    optional ReLU, with the convolution named "conv" as in checkpoints."""

    def __init__(self, in_channels, out_channels, relu=True):
        super().__init__()
        self.conv = nn.Conv2d(in_channels, out_channels, 7, 1, 3, bias=True)
        self.relu = relu

    def forward(self, x):
        x = self.conv(x)
        return F.relu(x, inplace=True) if self.relu else x


class SPyNetBasicModule(nn.Module):
    def __init__(self):
        super().__init__()
        self.basic_module = nn.Sequential(
            ConvModule(8, 32), ConvModule(32, 64), ConvModule(64, 32), ConvModule(32, 16),
            ConvModule(16, 2, relu=False))

    def forward(self, x):
        return self.basic_module(x)


class SPyNet(nn.Module):
    def __init__(self):
        super().__init__()
        self.basic_module = nn.ModuleList([SPyNetBasicModule() for _ in range(6)])
        self.register_buffer("mean", torch.Tensor([0.485, 0.456, 0.406]).view(1, 3, 1, 1))
        self.register_buffer("std", torch.Tensor([0.229, 0.224, 0.225]).view(1, 3, 1, 1))

    def compute_flow(self, ref, supp):
        n, _, h, w = ref.size()
        ref = [(ref - self.mean) / self.std]
        supp = [(supp - self.mean) / self.std]
        for _ in range(5):
            ref.append(F.avg_pool2d(ref[-1], kernel_size=2, stride=2, count_include_pad=False))
            supp.append(F.avg_pool2d(supp[-1], kernel_size=2, stride=2, count_include_pad=False))
        ref, supp = ref[::-1], supp[::-1]
        flow = ref[0].new_zeros(n, 2, h // 32, w // 32)
        for level in range(len(ref)):
            if level == 0:
                flow_up = flow
            else:
                flow_up = F.interpolate(flow, scale_factor=2, mode="bilinear", align_corners=True) * 2.0
            flow = flow_up + self.basic_module[level](torch.cat([
                ref[level], flow_warp(supp[level], flow_up.permute(0, 2, 3, 1), padding_mode="border"), flow_up], 1))
        return flow

    def forward(self, ref, supp):
        h, w = ref.shape[2:4]
        w_up = w if w % 32 == 0 else 32 * (w // 32 + 1)
        h_up = h if h % 32 == 0 else 32 * (h // 32 + 1)
        ref = F.interpolate(ref, size=(h_up, w_up), mode="bilinear", align_corners=False)
        supp = F.interpolate(supp, size=(h_up, w_up), mode="bilinear", align_corners=False)
        flow = F.interpolate(self.compute_flow(ref, supp), size=(h, w), mode="bilinear", align_corners=False)
        flow[:, 0, :, :] *= float(w) / float(w_up)
        flow[:, 1, :, :] *= float(h) / float(h_up)
        return flow


class BasicVSRNet(nn.Module):
    def __init__(self, mid_channels=64, num_blocks=20):
        super().__init__()
        self.mid_channels = mid_channels
        self.spynet = SPyNet()
        self.backward_resblocks = ResidualBlocksWithInputConv(mid_channels + 3, mid_channels, num_blocks)
        self.forward_resblocks = ResidualBlocksWithInputConv(mid_channels + 3, mid_channels, num_blocks)
        self.fusion = nn.Conv2d(mid_channels * 2, mid_channels, 1, 1, 0, bias=True)
        self.upsample1 = PixelShufflePack(mid_channels, mid_channels, 2, upsample_kernel=3)
        self.upsample2 = PixelShufflePack(mid_channels, 64, 2, upsample_kernel=3)
        self.conv_hr = nn.Conv2d(64, 64, 3, 1, 1)
        self.conv_last = nn.Conv2d(64, 3, 3, 1, 1)
        self.img_upsample = nn.Upsample(scale_factor=4, mode="bilinear", align_corners=False)
        self.lrelu = nn.LeakyReLU(negative_slope=0.1, inplace=True)

    def forward(self, lrs):
        n, t, c, h, w = lrs.size()
        lrs_1 = lrs[:, :-1].reshape(-1, c, h, w)
        lrs_2 = lrs[:, 1:].reshape(-1, c, h, w)
        flows_backward = self.spynet(lrs_1, lrs_2).view(n, t - 1, 2, h, w)
        flows_forward = self.spynet(lrs_2, lrs_1).view(n, t - 1, 2, h, w)

        outputs = []
        feat_prop = lrs.new_zeros(n, self.mid_channels, h, w)
        for i in range(t - 1, -1, -1):
            if i < t - 1:
                feat_prop = flow_warp(feat_prop, flows_backward[:, i].permute(0, 2, 3, 1))
            feat_prop = self.backward_resblocks(torch.cat([lrs[:, i], feat_prop], dim=1))
            outputs.append(feat_prop)
        outputs = outputs[::-1]

        feat_prop = torch.zeros_like(feat_prop)
        for i in range(t):
            lr_curr = lrs[:, i]
            if i > 0:
                feat_prop = flow_warp(feat_prop, flows_forward[:, i - 1].permute(0, 2, 3, 1))
            feat_prop = self.forward_resblocks(torch.cat([lr_curr, feat_prop], dim=1))
            out = torch.cat([outputs[i], feat_prop], dim=1)
            out = self.lrelu(self.fusion(out))
            out = self.lrelu(self.upsample1(out))
            out = self.lrelu(self.upsample2(out))
            out = self.lrelu(self.conv_hr(out))
            out = self.conv_last(out)
            outputs[i] = out + self.img_upsample(lr_curr)
        return torch.stack(outputs, dim=1)


class RealBasicVSRNet(nn.Module):
    def __init__(self, mid_channels=64, num_propagation_blocks=20, num_cleaning_blocks=20):
        super().__init__()
        self.image_cleaning = nn.Sequential(
            ResidualBlocksWithInputConv(3, mid_channels, num_cleaning_blocks),
            nn.Conv2d(mid_channels, 3, 3, 1, 1, bias=True))
        self.basicvsr = BasicVSRNet(mid_channels, num_propagation_blocks)

    def forward(self, lqs, cleaning_passes=1):
        n, t, c, h, w = lqs.size()
        # The original cleans up to three times but stops once a pass
        # changes the frames by less than its threshold, 255/255 on
        # average, which in practice means once. More passes are for
        # badly damaged video.
        for _ in range(cleaning_passes):
            flat = lqs.reshape(-1, c, h, w)
            lqs = (flat + self.image_cleaning(flat)).view(n, t, c, h, w)
        return self.basicvsr(lqs)


def load_model(path, device):
    ckpt = torch.load(path, map_location="cpu", weights_only=False)
    state = ckpt.get("state_dict", ckpt)
    # Training checkpoints hold the generator, its moving average and the
    # discriminator; the average is what OpenMMLab tests with.
    for prefix in ("generator_ema.", "generator."):
        weights = {k[len(prefix):]: v for k, v in state.items() if k.startswith(prefix)}
        if weights:
            break
    else:
        weights = state
    net = RealBasicVSRNet()
    net.load_state_dict(weights, strict=True)
    return net.eval().to(device)


# --- Running it on frames. ---


def pick_device(gpu):
    if torch.cuda.is_available():
        return torch.device(f"cuda:{gpu}" if gpu >= 0 else "cuda")
    if getattr(torch.backends, "mps", None) and torch.backends.mps.is_available():
        return torch.device("mps")
    return torch.device("cpu")


def describe(device):
    if device.type != "cuda":
        return str(device)
    return torch.cuda.get_device_name(device)


@torch.inference_mode()
def run_net(net, frames, device, half, cleaning):
    """Upscales a t×h×w×3 float tensor in [0, 1], at least 2 frames."""
    x = frames.permute(0, 3, 1, 2).unsqueeze(0).to(device)
    with torch.autocast(device.type, dtype=torch.float16, enabled=half):
        out = net(x, cleaning)
    return out.float().clamp_(0, 1).squeeze(0).permute(0, 2, 3, 1).cpu()


def tile_starts(n, tile, overlap):
    if tile <= 0 or n <= tile:
        return [0]
    out, x = [], 0
    while x + tile < n:
        out.append(x)
        x += tile - overlap
    return out + [n - tile]


def feather(size, ramp):
    w = torch.ones(size)
    if ramp > 0:
        r = torch.linspace(0, 1, ramp + 2)[1:-1]
        w[:ramp] = torch.minimum(w[:ramp], r)
        w[-ramp:] = torch.minimum(w[-ramp:], r.flip(0))
    return w


def upscale(net, frames, args, device, half, report):
    """Upscales t×h×w×3 frames 4x, in tiles when they're bigger than
    args.tile. The model needs at least 64 pixels a side, so smaller
    pictures are padded."""
    t, h, w, _ = frames.shape
    ph, pw = max(h, 64), max(w, 64)
    if ph > h or pw > w:
        frames = F.pad(frames.permute(0, 3, 1, 2), (0, pw - w, 0, ph - h), mode="replicate").permute(0, 2, 3, 1)
    overlap = 16
    ys, xs = tile_starts(ph, args.tile, overlap), tile_starts(pw, args.tile, overlap)
    if len(ys) == 1 and len(xs) == 1:
        out = run_net(net, frames, device, half, args.cleaning)
        report(1)
        return out[:, : h * 4, : w * 4]
    tile = args.tile
    out = torch.zeros((t, ph * 4, pw * 4, 3))
    weight = torch.zeros((ph * 4, pw * 4, 1))
    total = len(ys) * len(xs)
    for i, (y, x) in enumerate((y, x) for y in ys for x in xs):
        th, tw = min(tile, ph), min(tile, pw)
        part = run_net(net, frames[:, y : y + th, x : x + tw], device, half, args.cleaning)
        wy = feather(th * 4, overlap * 4 if len(ys) > 1 else 0)
        wx = feather(tw * 4, overlap * 4 if len(xs) > 1 else 0)
        wt = (wy[:, None] * wx[None, :])[:, :, None]
        out[:, y * 4 : (y + th) * 4, x * 4 : (x + tw) * 4] += part * wt
        weight[y * 4 : (y + th) * 4, x * 4 : (x + tw) * 4] += wt
        report((i + 1) / total)
    return (out / weight.clamp_(min=1e-6))[:, : h * 4, : w * 4]


def load(path):
    with Image.open(path) as im:
        return np.asarray(im.convert("RGB"), dtype=np.float32) / 255.0


def save(arr, path):
    Image.fromarray((arr * 255.0).round().astype(np.uint8), "RGB").save(path, compress_level=1)


def run_job(net, args, device):
    if os.path.isdir(args.input):
        os.makedirs(args.output, exist_ok=True)
        names = sorted(n for n in os.listdir(args.input) if os.path.splitext(n)[1].lower() in IMAGE_EXTS)
        jobs = [(os.path.join(args.input, n), os.path.join(args.output, os.path.splitext(n)[0] + ".png")) for n in names]
    else:
        jobs = [(args.input, args.output)]
    if not jobs:
        return
    half = device.type == "cuda" and args.precision == "half"
    last = [-1]

    def report(f):
        pct = int(f * 100)
        if pct != last[0]:
            last[0] = pct
            print(f"{pct}%", flush=True)

    chunk = max(2, args.chunk)
    chunks = math.ceil(len(jobs) / chunk)
    pool = ThreadPoolExecutor(max_workers=max(2, min(8, os.cpu_count() or 2)))
    try:
        for c in range(chunks):
            start, end = c * chunk, min(len(jobs), (c + 1) * chunk)
            first, stop = max(0, start - CHUNK_PAD), min(len(jobs), end + CHUNK_PAD)
            frames = torch.from_numpy(np.stack(list(pool.map(load, [src for src, _ in jobs[first:stop]]))))
            if frames.shape[0] == 1:
                # A single picture: the model needs a sequence, so it gets
                # the picture twice.
                frames = frames.repeat(2, 1, 1, 1)
            out = upscale(net, frames, args, device, half, lambda f: report((c + f) / chunks))
            saving = [pool.submit(save, out[i - first].numpy(), dst) for i, (_, dst) in enumerate(jobs[start:end], start)]
            for s in saving:
                s.result()
            del frames, out
    finally:
        pool.shutdown()
        if device.type == "cuda":
            torch.cuda.empty_cache()


def parse(argv):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--model", required=True)
    ap.add_argument("--input", required=True, help="a folder of frames, or an image")
    ap.add_argument("--output", required=True, help="a folder, or an image")
    ap.add_argument("--chunk", type=int, default=15, help="frames upscaled together")
    ap.add_argument("--tile", type=int, default=480, help="tile size in input pixels; 0 for none")
    ap.add_argument("--cleaning", type=int, default=1, choices=[1, 2, 3], help="passes of the cleaning module")
    ap.add_argument("--precision", choices=["half", "full"], default="half")
    ap.add_argument("--gpu", type=int, default=-1)
    return ap.parse_args(argv)


def explain(err):
    msg = str(err).strip().splitlines()[0] if str(err).strip() else type(err).__name__
    if isinstance(err, torch.cuda.OutOfMemoryError) or "out of memory" in msg.lower():
        return "The GPU ran out of memory. Use smaller tiles or fewer frames at a time."
    if isinstance(err, FileNotFoundError):
        return f"File not found: {err.filename or msg}"
    if "state_dict" in msg or "size mismatch" in msg:
        return "This file isn't a RealBasicVSR x4 checkpoint: " + msg
    return f"{type(err).__name__}: {msg}"


def main():
    serving = os.environ.get("TOUCHUP_SERVE") == "1"
    argv, net, key, device = sys.argv[1:], None, None, None
    while True:
        code = 0
        try:
            args = parse(argv)
            if net is None or key != (args.model, args.gpu):
                device = pick_device(args.gpu)
                if device.type == "cuda":
                    torch.backends.cudnn.benchmark = True
                net, key = load_model(args.model, device), (args.model, args.gpu)
                print(f"Loaded RealBasicVSR 4x on {describe(device)}, PyTorch {torch.__version__}", flush=True)
            run_job(net, args, device)
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
