"""TouchUp's launcher for Apollo's inference script.

Runs src/inference.py with the arguments it's given, but when the GPU
runs out of memory it halves the chunk of audio processed at a time and
tries again, down to a second, instead of failing: how long a chunk fits
depends on the GPU and on what else is using it.

    python apollo_launch.py src/inference.py --in_wav in.wav --out_wav out.wav --chunk-seconds 5 ...
"""

import importlib.util
import os
import sys

import torch


def main():
    argv = sys.argv[1:]
    if not argv:
        sys.exit(__doc__)
    script = os.path.abspath(argv.pop(0))
    sys.path.insert(0, os.path.dirname(script))
    spec = importlib.util.spec_from_file_location("apollo_inference", script)
    inference = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(inference)

    run_model = inference.run_model
    rate = inference.SAMPLE_RATE

    def run_model_fitting(model, audio, device, chunk_samples=None, overlap_samples=0, chunk_batch_size=1, chunk_pad_samples=0):
        while True:
            try:
                return run_model(model, audio, device, chunk_samples=chunk_samples, overlap_samples=overlap_samples,
                                 chunk_batch_size=chunk_batch_size, chunk_pad_samples=chunk_pad_samples)
            except torch.OutOfMemoryError:
                current = chunk_samples or audio.shape[-1]
                smaller = current // 2
                # A chunk has to hold its crossfades, and below a second
                # the padding around it costs more than the chunk.
                if smaller < max(rate, 2 * overlap_samples):
                    raise
                if device.type == "cuda":
                    torch.cuda.empty_cache()
                print(f"TouchUp: out of GPU memory; retrying with {smaller / rate:.1f}-second chunks", flush=True)
                chunk_samples = smaller

    inference.run_model = run_model_fitting
    sys.argv = [script] + argv
    inference.main()


if __name__ == "__main__":
    main()
