package engine

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/mehran9675/touchup/internal/filters"
	"github.com/mehran9675/touchup/internal/tr"
)

// RTX Video Super Resolution is NVIDIA's AI upscaler for video, the one
// browsers and VLC use. The graphics driver runs it inside Direct3D 11's
// video processor when an app turns it on with an NVIDIA extension, so
// it needs no download: TouchUp creates a video processor on the NVIDIA
// GPU, enables the extension, and puts each frame through it, converted
// to NV12 (the YUV layout video decoders hand the processor) and read
// back as BGRA.
//
// The calls go straight through COM vtables with syscall, so TouchUp
// needs no C compiler. Vtable indexes follow d3d11.h and dxgi.h.

const rtxVideoSupported = true

var (
	modD3D11               = windows.NewLazySystemDLL("d3d11.dll")
	modDXGI                = windows.NewLazySystemDLL("dxgi.dll")
	procD3D11CreateDevice  = modD3D11.NewProc("D3D11CreateDevice")
	procCreateDXGIFactory1 = modDXGI.NewProc("CreateDXGIFactory1")
)

var (
	iidIDXGIFactory1     = windows.GUID{Data1: 0x770aae78, Data2: 0xf26f, Data3: 0x4dba, Data4: [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}
	iidID3D11VideoDevice = windows.GUID{Data1: 0x10ec4d5b, Data2: 0x975a, Data3: 0x4689, Data4: [8]byte{0xb9, 0xe4, 0xd0, 0xaa, 0xc3, 0x0f, 0xe3, 0x33}}
	iidID3D11VideoCtx    = windows.GUID{Data1: 0x61f21c45, Data2: 0x3c0e, Data3: 0x4a74, Data4: [8]byte{0x9c, 0xea, 0x67, 0x10, 0x0d, 0x9a, 0xd5, 0xe4}}
	// NVIDIA's post-processing extension, as Chromium and VLC use it.
	guidNvidiaPPE = windows.GUID{Data1: 0xd43ce1b3, Data2: 0x1f4b, Data3: 0x48ac, Data4: [8]byte{0xba, 0xee, 0xc3, 0xc2, 0x53, 0x75, 0xe6, 0xf7}}
)

// Vtable indexes.
const (
	vtRelease        = 2
	vtQueryInterface = 0

	vtFactoryEnumAdapters1 = 12
	vtAdapterGetDesc1      = 10

	vtDeviceCreateTexture2D = 5

	vtCtxMap          = 14
	vtCtxUnmap        = 15
	vtCtxCopyResource = 47

	vtVDevCreateVideoProcessor           = 4
	vtVDevCreateVideoProcessorInputView  = 8
	vtVDevCreateVideoProcessorOutputView = 9
	vtVDevCreateVideoProcessorEnumerator = 10

	vtVCtxSetOutputTargetRect  = 13
	vtVCtxSetOutputColorSpace  = 15
	vtVCtxSetStreamFrameFormat = 27
	vtVCtxSetStreamColorSpace  = 28
	vtVCtxSetStreamSourceRect  = 30
	vtVCtxSetStreamDestRect    = 31
	vtVCtxSetStreamExtension   = 39
	vtVCtxBlt                  = 53
)

const (
	formatBGRA8 = 87
	formatNV12  = 103

	usageDefault = 0
	usageStaging = 3

	bindShaderResource = 0x8
	bindRenderTarget   = 0x20
	bindDecoder        = 0x200

	cpuWrite = 0x10000
	cpuRead  = 0x20000

	mapRead  = 1
	mapWrite = 2

	createVideoSupport = 0x800
	createBGRASupport  = 0x20

	dxgiErrorNotFound = 0x887A0002
	nvidiaVendor      = 0x10DE
)

// comObject is a COM interface pointer: the object starts with a pointer
// to its vtable.
type comObject struct{ vtbl *[128]uintptr }

func (c *comObject) call(i int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(c.vtbl[i], append([]uintptr{uintptr(unsafe.Pointer(c))}, args...)...)
	return r
}

func (c *comObject) release() {
	if c != nil {
		c.call(vtRelease)
	}
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

func hresult(what string, hr uintptr) error {
	return fmt.Errorf("%s failed (HRESULT 0x%08X)", what, uint32(hr))
}

type texture2DDesc struct {
	Width, Height, MipLevels, ArraySize, Format uint32
	SampleCount, SampleQuality                  uint32
	Usage, BindFlags, CPUAccessFlags, MiscFlags uint32
}

type mappedSubresource struct {
	Data       unsafe.Pointer
	RowPitch   uint32
	DepthPitch uint32
}

type adapterDesc1 struct {
	Description           [128]uint16
	VendorID              uint32
	DeviceID              uint32
	SubSysID              uint32
	Revision              uint32
	DedicatedVideoMemory  uintptr
	DedicatedSystemMemory uintptr
	SharedSystemMemory    uintptr
	AdapterLuid           [8]byte
	Flags                 uint32
}

type videoProcessorContentDesc struct {
	InputFrameFormat          uint32
	InputRateNum, InputRateD  uint32
	InputWidth, InputHeight   uint32
	OutRateNum, OutRateD      uint32
	OutputWidth, OutputHeight uint32
	Usage                     uint32
}

type videoProcessorStream struct {
	Enable              int32
	OutputIndex         uint32
	InputFrameOrField   uint32
	PastFrames          uint32
	FutureFrames        uint32
	PastSurfaces        uintptr
	InputSurface        *comObject
	FutureSurfaces      uintptr
	PastSurfacesRight   uintptr
	InputSurfaceRight   uintptr
	FutureSurfacesRight uintptr
}

type rect struct{ Left, Top, Right, Bottom int32 }

// rtxVideo upscales frames of one size to another with RTX Video Super
// Resolution. It's created for the first frame and again when sizes
// change.
type rtxVideo struct {
	inW, inH, outW, outH int

	device, ctx, vdev, vctx   *comObject
	enum, proc                *comObject
	inStage, inTex, inView    *comObject
	outTex, outStage, outView *comObject
}

func newRTXVideo() (Upscaler, error) { return &rtxVideo{}, nil }

func (v *rtxVideo) Upscale(img *filters.Image, w, h int) (*filters.Image, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// NV12 halves the chroma, so the input is padded to even sides.
	inW, inH := img.W+img.W%2, img.H+img.H%2
	if v.device == nil || v.inW != inW || v.inH != inH || v.outW != w || v.outH != h {
		v.Close()
		if err := v.open(inW, inH, w, h, img.W, img.H); err != nil {
			v.Close()
			return nil, err
		}
	}
	if err := v.upload(img); err != nil {
		return nil, err
	}
	stream := videoProcessorStream{Enable: 1, InputSurface: v.inView}
	if hr := v.vctx.call(vtVCtxBlt, uintptr(unsafe.Pointer(v.proc)), uintptr(unsafe.Pointer(v.outView)), 0, 1, uintptr(unsafe.Pointer(&stream))); failed(hr) {
		return nil, hresult("VideoProcessorBlt", hr)
	}
	return v.download(img)
}

// open creates the device, textures and video processor for inW×inH
// frames (the picture itself srcW×srcH) made outW×outH.
func (v *rtxVideo) open(inW, inH, outW, outH, srcW, srcH int) error {
	v.inW, v.inH, v.outW, v.outH = inW, inH, outW, outH
	adapter, err := nvidiaAdapter()
	if err != nil {
		return err
	}
	defer adapter.release()
	hr, _, _ := procD3D11CreateDevice.Call(uintptr(unsafe.Pointer(adapter)), 0, 0, createVideoSupport|createBGRASupport,
		0, 0, 7, uintptr(unsafe.Pointer(&v.device)), 0, uintptr(unsafe.Pointer(&v.ctx)))
	if failed(hr) {
		return hresult("D3D11CreateDevice", hr)
	}
	if hr := v.device.call(vtQueryInterface, uintptr(unsafe.Pointer(&iidID3D11VideoDevice)), uintptr(unsafe.Pointer(&v.vdev))); failed(hr) {
		return errors.New(tr.T("this graphics driver has no video processor"))
	}
	if hr := v.ctx.call(vtQueryInterface, uintptr(unsafe.Pointer(&iidID3D11VideoCtx)), uintptr(unsafe.Pointer(&v.vctx))); failed(hr) {
		return errors.New(tr.T("this graphics driver has no video processor"))
	}

	content := videoProcessorContentDesc{
		InputRateNum: 30, InputRateD: 1, InputWidth: uint32(inW), InputHeight: uint32(inH),
		OutRateNum: 30, OutRateD: 1, OutputWidth: uint32(outW), OutputHeight: uint32(outH),
	}
	if hr := v.vdev.call(vtVDevCreateVideoProcessorEnumerator, uintptr(unsafe.Pointer(&content)), uintptr(unsafe.Pointer(&v.enum))); failed(hr) {
		return hresult("CreateVideoProcessorEnumerator", hr)
	}
	if hr := v.vdev.call(vtVDevCreateVideoProcessor, uintptr(unsafe.Pointer(v.enum)), 0, uintptr(unsafe.Pointer(&v.proc))); failed(hr) {
		return hresult("CreateVideoProcessor", hr)
	}

	// Input: an NV12 staging texture TouchUp writes, copied into one the
	// processor reads. Drivers differ in the bind flags they take for a
	// processor's input, so a few are tried.
	if err := v.texture(texture2DDesc{Width: uint32(inW), Height: uint32(inH), Format: formatNV12,
		Usage: usageStaging, CPUAccessFlags: cpuWrite}, &v.inStage); err != nil {
		return err
	}
	inDesc := [4]uint32{0, 1, 0, 0} // FourCC, TEXTURE2D, mip, array slice
	var lastErr error
	for _, bind := range []uint32{bindDecoder, bindShaderResource, bindRenderTarget, 0} {
		v.inTex.release()
		v.inTex = nil
		if lastErr = v.texture(texture2DDesc{Width: uint32(inW), Height: uint32(inH), Format: formatNV12,
			Usage: usageDefault, BindFlags: bind}, &v.inTex); lastErr != nil {
			continue
		}
		hr := v.vdev.call(vtVDevCreateVideoProcessorInputView, uintptr(unsafe.Pointer(v.inTex)), uintptr(unsafe.Pointer(v.enum)),
			uintptr(unsafe.Pointer(&inDesc)), uintptr(unsafe.Pointer(&v.inView)))
		if !failed(hr) {
			lastErr = nil
			break
		}
		lastErr = hresult("CreateVideoProcessorInputView", hr)
	}
	if lastErr != nil {
		return lastErr
	}

	// Output: a BGRA render target, copied to a staging texture to read.
	if err := v.texture(texture2DDesc{Width: uint32(outW), Height: uint32(outH), Format: formatBGRA8,
		Usage: usageDefault, BindFlags: bindRenderTarget}, &v.outTex); err != nil {
		return err
	}
	if err := v.texture(texture2DDesc{Width: uint32(outW), Height: uint32(outH), Format: formatBGRA8,
		Usage: usageStaging, CPUAccessFlags: cpuRead}, &v.outStage); err != nil {
		return err
	}
	outDesc := [4]uint32{1, 0, 0, 0} // TEXTURE2D, mip
	if hr := v.vdev.call(vtVDevCreateVideoProcessorOutputView, uintptr(unsafe.Pointer(v.outTex)), uintptr(unsafe.Pointer(v.enum)),
		uintptr(unsafe.Pointer(&outDesc)), uintptr(unsafe.Pointer(&v.outView))); failed(hr) {
		return hresult("CreateVideoProcessorOutputView", hr)
	}

	// BT.709 video levels in, full-range RGB out: the color space
	// bitfields are Usage, RGB_Range, YCbCr_Matrix, xvYCC, then a 2-bit
	// Nominal_Range (1 is 16–235).
	streamSpace := uint32(1<<2 | 1<<4)
	outSpace := uint32(0)
	p := uintptr(unsafe.Pointer(v.proc))
	v.vctx.call(vtVCtxSetStreamFrameFormat, p, 0, 0) // progressive
	v.vctx.call(vtVCtxSetStreamColorSpace, p, 0, uintptr(unsafe.Pointer(&streamSpace)))
	v.vctx.call(vtVCtxSetOutputColorSpace, p, uintptr(unsafe.Pointer(&outSpace)))
	src := rect{0, 0, int32(srcW), int32(srcH)}
	dst := rect{0, 0, int32(outW), int32(outH)}
	v.vctx.call(vtVCtxSetStreamSourceRect, p, 0, 1, uintptr(unsafe.Pointer(&src)))
	v.vctx.call(vtVCtxSetStreamDestRect, p, 0, 1, uintptr(unsafe.Pointer(&dst)))
	v.vctx.call(vtVCtxSetOutputTargetRect, p, 1, uintptr(unsafe.Pointer(&dst)))

	// Version 1, method 2 (super resolution), on.
	ext := [3]uint32{1, 2, 1}
	if hr := v.vctx.call(vtVCtxSetStreamExtension, p, 0, uintptr(unsafe.Pointer(&guidNvidiaPPE)), unsafe.Sizeof(ext), uintptr(unsafe.Pointer(&ext))); failed(hr) {
		return fmt.Errorf("%s (HRESULT 0x%08X)", tr.T("the driver turned down RTX Video Super Resolution: it needs a GeForce RTX GPU and a recent NVIDIA driver"), uint32(hr))
	}
	return nil
}

func (v *rtxVideo) texture(desc texture2DDesc, out **comObject) error {
	desc.MipLevels, desc.ArraySize, desc.SampleCount = 1, 1, 1
	if hr := v.device.call(vtDeviceCreateTexture2D, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(out))); failed(hr) {
		return hresult("CreateTexture2D", hr)
	}
	return nil
}

// upload converts img to NV12 (BT.709, video levels) into the input.
func (v *rtxVideo) upload(img *filters.Image) error {
	var m mappedSubresource
	if hr := v.ctx.call(vtCtxMap, uintptr(unsafe.Pointer(v.inStage)), 0, mapWrite, 0, uintptr(unsafe.Pointer(&m))); failed(hr) {
		return hresult("Map", hr)
	}
	pitch := int(m.RowPitch)
	data := unsafe.Slice((*byte)(m.Data), pitch*v.inH*3/2)
	at := func(x, y int) (float32, float32, float32) {
		x, y = min(x, img.W-1), min(y, img.H-1)
		i := (y*img.W + x) * 3
		return img.Pix[i], img.Pix[i+1], img.Pix[i+2]
	}
	for y := 0; y < v.inH; y++ {
		row := data[y*pitch:]
		for x := 0; x < v.inW; x++ {
			r, g, b := at(x, y)
			row[x] = byte(16 + 219*(0.2126*r+0.7152*g+0.0722*b) + 0.5)
		}
	}
	uv := data[pitch*v.inH:]
	for y := 0; y < v.inH/2; y++ {
		row := uv[y*pitch:]
		for x := 0; x < v.inW/2; x++ {
			var r, g, b float32
			for _, d := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				pr, pg, pb := at(2*x+d[0], 2*y+d[1])
				r, g, b = r+pr/4, g+pg/4, b+pb/4
			}
			yy := 0.2126*r + 0.7152*g + 0.0722*b
			row[2*x] = byte(clamp8(128 + 224*(b-yy)/1.8556))
			row[2*x+1] = byte(clamp8(128 + 224*(r-yy)/1.5748))
		}
	}
	v.ctx.call(vtCtxUnmap, uintptr(unsafe.Pointer(v.inStage)), 0)
	v.ctx.call(vtCtxCopyResource, uintptr(unsafe.Pointer(v.inTex)), uintptr(unsafe.Pointer(v.inStage)))
	return nil
}

func clamp8(v float32) float32 { return max(0, min(255, v+0.5)) }

// download reads the result, with img's alpha scaled along.
func (v *rtxVideo) download(img *filters.Image) (*filters.Image, error) {
	v.ctx.call(vtCtxCopyResource, uintptr(unsafe.Pointer(v.outStage)), uintptr(unsafe.Pointer(v.outTex)))
	var m mappedSubresource
	if hr := v.ctx.call(vtCtxMap, uintptr(unsafe.Pointer(v.outStage)), 0, mapRead, 0, uintptr(unsafe.Pointer(&m))); failed(hr) {
		return nil, hresult("Map", hr)
	}
	defer v.ctx.call(vtCtxUnmap, uintptr(unsafe.Pointer(v.outStage)), 0)
	pitch := int(m.RowPitch)
	data := unsafe.Slice((*byte)(m.Data), pitch*v.outH)
	out := filters.NewImage(v.outW, v.outH)
	for y := 0; y < v.outH; y++ {
		row := data[y*pitch:]
		for x := 0; x < v.outW; x++ {
			i := (y*v.outW + x) * 3
			out.Pix[i] = float32(row[4*x+2]) / 255
			out.Pix[i+1] = float32(row[4*x+1]) / 255
			out.Pix[i+2] = float32(row[4*x]) / 255
		}
	}
	if img.Alpha != nil {
		out.Alpha = filters.Resize(img, v.outW, v.outH, filters.Bicubic).Alpha
	}
	return out, nil
}

func (v *rtxVideo) Close() {
	for _, o := range []**comObject{&v.inView, &v.outView, &v.inTex, &v.inStage, &v.outTex, &v.outStage,
		&v.proc, &v.enum, &v.vctx, &v.vdev, &v.ctx, &v.device} {
		(*o).release()
		*o = nil
	}
}

// nvidiaAdapter finds the first NVIDIA GPU.
func nvidiaAdapter() (*comObject, error) {
	var factory *comObject
	if hr, _, _ := procCreateDXGIFactory1.Call(uintptr(unsafe.Pointer(&iidIDXGIFactory1)), uintptr(unsafe.Pointer(&factory))); failed(hr) {
		return nil, hresult("CreateDXGIFactory1", hr)
	}
	defer factory.release()
	for i := uintptr(0); ; i++ {
		var adapter *comObject
		hr := factory.call(vtFactoryEnumAdapters1, i, uintptr(unsafe.Pointer(&adapter)))
		if uint32(hr) == dxgiErrorNotFound {
			return nil, errors.New(tr.T("RTX Video Super Resolution needs an NVIDIA GeForce RTX GPU, and none was found"))
		}
		if failed(hr) {
			return nil, hresult("EnumAdapters1", hr)
		}
		var desc adapterDesc1
		if !failed(adapter.call(vtAdapterGetDesc1, uintptr(unsafe.Pointer(&desc)))) && desc.VendorID == nvidiaVendor {
			return adapter, nil
		}
		adapter.release()
	}
}
