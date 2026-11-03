//go:build !windows

package engine

import (
	"errors"

	"github.com/mehran9675/touchup/internal/tr"
)

// RTX Video Super Resolution is reached through Direct3D 11, so only
// on Windows.
const rtxVideoSupported = false

func newRTXVideo() (Upscaler, error) {
	return nil, errors.New(tr.T("RTX Video Super Resolution is only available on Windows"))
}
