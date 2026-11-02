"""TouchUp's launcher for AudioSR's command line.

AudioSR 0.0.7 pins packages that have no builds for the Python TouchUp
runs (numpy 1.23, transformers 4.30), so TouchUp installs it without
them and with current versions instead. This makes those work with it,
then runs AudioSR's own command line with the arguments it's given:

  - torch.load loads AudioSR's checkpoints in full, as PyTorch before
    2.6 did by default (they hold more than weights);
  - torchaudio.load reads WAV with soundfile, since newer torchaudio
    needs TorchCodec and FFmpeg's libraries for it;
  - librosa.util.pad_center takes its size by position again, as
    AudioSR calls it.

    python audiosr_launch.py -i in.wav -s out/ --model_name basic
"""

import functools
import runpy
import sys

import torch

_load = torch.load


@functools.wraps(_load)
def load(*args, **kwargs):
    kwargs.setdefault("weights_only", False)
    return _load(*args, **kwargs)


torch.load = load

import soundfile  # noqa: E402
import torchaudio  # noqa: E402


def read_audio(path, *args, **kwargs):
    data, rate = soundfile.read(path, dtype="float32", always_2d=True)
    return torch.from_numpy(data.T.copy()), rate


torchaudio.load = read_audio

import librosa.util  # noqa: E402

_pad_center = librosa.util.pad_center


@functools.wraps(_pad_center)
def pad_center(data, *args, **kwargs):
    if args:
        kwargs.setdefault("size", args[0])
    return _pad_center(data, **kwargs)


librosa.util.pad_center = pad_center

if __name__ == "__main__":
    sys.argv[0] = "audiosr"
    runpy.run_module("audiosr", run_name="__main__", alter_sys=True)
