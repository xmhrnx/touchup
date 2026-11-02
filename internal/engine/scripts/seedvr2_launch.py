"""TouchUp's launcher for SeedVR2's command line.

Runs src/inference_cli.py with the arguments it's given, after taking
out its own, which SeedVR2 has no flags for:

    --touchup-chunk-frames N  frames to hold in memory at once, rounded to
                              a whole number of batches (--chunk_size)
    --touchup-vae-memory GB   memory each VAE convolution may use before
                              it's split into pieces (SeedVR2's is 0.5)
    --touchup-cudnn-tune      let cuDNN time its kernels and keep the
                              fastest for these sizes
    --touchup-plain-conv3d    skip SeedVR2's workaround for a Conv3d
                              memory bug of PyTorch 2.9 and 2.10, which
                              calls cuDNN directly with tuning off

A folder as the input also keeps the models loaded from file to file
(--cache_dit, --cache_vae). With TOUCHUP_SERVE=1 it stays running and
takes more jobs from standard input, with the models loaded once (see
serve below).

    python seedvr2_launch.py src/inference_cli.py video.mp4 --batch_size 21 ...
"""

import inspect
import os
import platform
import runpy
import shutil
import sys
import time


def take(argv, flag, has_value=True):
    """Removes flag (and its value) from argv, returning the value, True
    for a present switch, or None."""
    if flag not in argv:
        return None
    i = argv.index(flag)
    if not has_value:
        del argv[i]
        return True
    value = argv[i + 1] if i + 1 < len(argv) else None
    del argv[i : i + 2]
    return value


def arg(argv, flag, default):
    if flag in argv:
        i = argv.index(flag)
        if i + 1 < len(argv):
            return argv[i + 1]
    return default


def aligned_chunk(frames, batch, overlap):
    """A chunk size that splits into whole batches. Each chunk after the
    first starts with `overlap` frames of the last one, and batches of
    `batch` frames overlap by as many, so every batch adds batch - overlap
    new frames: chunks that are a multiple of that waste nothing."""
    step = batch - overlap if batch > overlap else batch
    return max(step, round(frames / step) * step)


def patch_vae_memory(gigabytes):
    from src.models.video_vae_v3.modules import attn_video_vae

    patched = 0
    for cls in vars(attn_video_vae).values():
        # Only the VAE's own method, which takes both limits: the module
        # also imports its convolutions, whose set_memory_limit takes one.
        if not isinstance(cls, type) or "set_memory_limit" not in vars(cls):
            continue
        if "conv_max_mem" not in inspect.signature(vars(cls)["set_memory_limit"]).parameters:
            continue
        original = cls.set_memory_limit

        def set_memory_limit(self, conv_max_mem=None, norm_max_mem=None, _original=original):
            _original(self, conv_max_mem=gigabytes, norm_max_mem=gigabytes)

        cls.set_memory_limit = set_memory_limit
        patched += 1
    if patched:
        print(f"TouchUp: VAE steps may use {gigabytes:g} GB before splitting", flush=True)


def plain_conv3d():
    from src.models.video_vae_v3.modules import causal_inflation_lib

    causal_inflation_lib.NVIDIA_CONV3D_MEMORY_BUG_WORKAROUND = False
    print("TouchUp: Conv3d workaround off", flush=True)


def tune_cudnn():
    import torch

    torch.backends.cudnn.benchmark = True
    # SeedVR2 calls cuDNN directly for 3D convolutions on recent PyTorch,
    # with tuning off; turn it on there too.
    original = torch.cudnn_convolution

    def cudnn_convolution(*args, benchmark=False, **kwargs):
        return original(*args, benchmark=True, **kwargs)

    torch.cudnn_convolution = cudnn_convolution
    print("TouchUp: cuDNN tuning on", flush=True)


def prepare(argv):
    """Turns TouchUp's flags in argv into SeedVR2's, returning the patch
    settings: (vae memory, tune, plain conv3d)."""
    chunk = take(argv, "--touchup-chunk-frames")
    vae_memory = take(argv, "--touchup-vae-memory")
    tune = take(argv, "--touchup-cudnn-tune", has_value=False)
    plain = take(argv, "--touchup-plain-conv3d", has_value=False)

    if chunk is not None:
        batch = int(arg(argv, "--batch_size", "5"))
        overlap = int(arg(argv, "--temporal_overlap", "0"))
        size = aligned_chunk(int(chunk), batch, overlap)
        argv += ["--chunk_size", str(size)]
        print(f"TouchUp: {size} frames per chunk, in batches of {batch}", flush=True)

    # A folder is processed file by file, and a server runs job after job;
    # keep the models loaded in between instead of loading them again.
    # (They wait in system memory, which takes a moment, not a reload.)
    if SERVING or (argv and os.path.isdir(argv[0])):
        for flag in ("--cache_dit", "--cache_vae"):
            if flag not in argv:
                argv.append(flag)
    return vae_memory, tune, plain


def setup(cli, argv, vae_memory, tune, plain):
    # SeedVR2 picks the GPU by setting CUDA_VISIBLE_DEVICES before torch
    # starts CUDA, and the patches below import modules that start it, so
    # it's set here first. SeedVR2 leaves a set one alone.
    device = arg(argv, "--cuda_device", None)
    if device is not None and "," not in device and "CUDA_VISIBLE_DEVICES" not in os.environ:
        os.environ["CUDA_VISIBLE_DEVICES"] = device.strip()

    # Imports below need SeedVR2's folder first on the path, as running
    # its script directly would give.
    sys.path.insert(0, os.path.dirname(cli))
    if tune:
        tune_cudnn()
    if vae_memory is not None:
        patch_vae_memory(float(vae_memory))
    if plain:
        plain_conv3d()


# SERVING is set when TouchUp runs this as a server (TOUCHUP_SERVE=1):
# after the job in the arguments it reads more from standard input, a
# JSON array of arguments a line, and ends each with "TOUCHUP-SERVE-DONE
# <exit code>". The models stay loaded from job to job.
SERVING = os.environ.get("TOUCHUP_SERVE") == "1"


def run_job(cli, argv, cache):
    """Runs one job like SeedVR2's main(), but with models kept in cache
    from one job to the next."""
    sys.argv = [cli.__file__] + argv
    args = cli.parse_arguments()
    cli.debug.enabled = args.debug
    args.cache_dit = args.cache_vae = True
    if args.video_backend == "ffmpeg" and shutil.which("ffmpeg") is None:
        raise RuntimeError("--video_backend ffmpeg requires ffmpeg in PATH")
    if platform.system() == "Darwin" or not args.cuda_device:
        devices = ["0"]
    else:
        devices = [d.strip() for d in str(args.cuda_device).split(",") if d.strip()]
    if not cli.download_weight(dit_model=args.dit_model, vae_model=cli.DEFAULT_VAE, model_dir=args.model_dir, debug=cli.debug):
        raise RuntimeError("the model files are missing")

    started = time.time()
    kind = cli.get_input_type(args.input)
    if kind == "directory":
        for path in cli.get_media_files(args.input):
            file_kind = cli.get_input_type(path)
            args.output_format = "mp4" if file_kind == "video" else "png"
            out = cli.generate_output_path(path, args.output_format, args.output, input_type=file_kind, from_directory=True)
            cli.process_single_file(path, args, devices, out, format_auto_detected=True, runner_cache=cache)
    elif kind in ("video", "image"):
        args.output_format = "mp4" if kind == "video" else "png"
        cli.process_single_file(args.input, args, devices, args.output, format_auto_detected=True, runner_cache=cache)
    else:
        raise RuntimeError(f"Unsupported input type: {args.input}")
    print(f"TouchUp: job done in {time.time() - started:.1f}s", flush=True)


def serve(cli_path, argv):
    """Runs the job in argv, then the ones TouchUp sends, with the models
    loaded once."""
    import importlib.util
    import json
    import traceback

    # SeedVR2 reads --cuda_device when it's imported.
    sys.argv = [cli_path] + argv
    spec = importlib.util.spec_from_file_location("seedvr2_cli", cli_path)
    cli = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(cli)

    cache = {}
    while True:
        code = 0
        try:
            run_job(cli, argv, cache)
        except SystemExit as e:
            code = e.code if isinstance(e.code, int) else (0 if e.code is None else 1)
        except Exception as e:
            traceback.print_exc()
            print(f"Error: {e}", flush=True)
            code = 1
        # Results of the last job wait for the next one in system memory,
        # not on the GPU.
        ctx = cache.get("ctx")
        if ctx is not None:
            for key in [k for k in ctx if k not in KEEP]:
                del ctx[key]
        try:
            import torch

            if torch.cuda.is_available():
                torch.cuda.empty_cache()
        except Exception:
            pass
        print(f"TOUCHUP-SERVE-DONE {code}", flush=True)
        if code != 0:
            return  # TouchUp starts a new server after a failure
        line = sys.stdin.readline()
        if not line:
            return
        # The command's arguments: this script, SeedVR2's, then the job.
        argv = json.loads(line)[2:]
        prepare(argv)  # the patches were set up by the first job


# What SeedVR2 keeps of a generation's context between jobs.
KEEP = {"dit_device", "vae_device", "dit_offload_device", "vae_offload_device", "tensor_offload_device", "compute_dtype"}


def main():
    argv = sys.argv[1:]
    if not argv:
        sys.exit(__doc__)
    cli = os.path.abspath(argv.pop(0))
    patches = prepare(argv)
    setup(cli, argv, *patches)
    if SERVING:
        serve(cli, argv)
        return
    sys.argv = [cli] + argv
    runpy.run_path(cli, run_name="__main__")


if __name__ == "__main__":
    main()
