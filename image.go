package ocrpdf

// Based in part on code from https://github.com/GeertJohan/go.leptonica

// #cgo LDFLAGS: -llept
// #include "leptonica/allheaders.h"
// #include <stdlib.h>
import "C"
import (
	"bytes"
	"fmt"
	"runtime"
	"unsafe"
)

const DefaultJPEGCompression int = 75

// NewImageFromFile creates and returns a new image loaded from the given
// file path.
func NewImageFromFile(filename string) (*Image, error) {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	// create new PIX
	cPIX := C.pixRead(cFilename)
	if cPIX == nil {
		return nil, fmt.Errorf("could not read image from '%s'", filename)
	}

	img := &Image{
		cPIX:      cPIX,
		pixFormat: C.getImpliedFileFormat(cFilename),
	}

	runtime.SetFinalizer(img, (*Image).delete)

	return img, nil
}

type Image struct {
	cPIX      *C.PIX
	buf       *bytes.Buffer
	pixFormat C.l_int32
}

func (i *Image) delete() {
	if i.cPIX != nil {
		C.pixDestroy(&i.cPIX)
		C.free(unsafe.Pointer(i.cPIX))
		i.cPIX = nil
	}
}

func (i *Image) CPIX() *C.PIX {
	return i.cPIX
}

// Adjust improves the clarity and contrast of the image, generally reducing
// scanning artifacts.
func (i *Image) Adjust(threshold float32) *Image {
	depth := C.pixGetDepth(i.cPIX)
	if depth == 1 {
		// Can't improve contrast on 1BPP images!
		return i
	}
	result := C.pixContrastTRC(i.cPIX, i.cPIX, C.l_float32(threshold))
	return &Image{
		cPIX: result,
	}
}

// Dimensions calculates the width, height and colour depth of the image.
func (i Image) Dimensions() (int32, int32, int32) {
	var w, h, d int32

	cW := C.l_int32(w)
	cH := C.l_int32(h)
	cD := C.l_int32(d)

	C.pixGetDimensions(i.cPIX, &cW, &cH, &cD)

	w = int32(cW)
	h = int32(cH)
	d = int32(cD)

	return w, h, d
}

// Scale resizes the image to the specified dimensions.
func (i *Image) Scale(w, h int32) *Image {
	result := C.pixScaleToSize(i.cPIX, C.l_int32(w), C.l_int32(h))
	return &Image{
		cPIX: result,
	}
}

// ScaleDown scales down the image to the specified dimensions, returning
// the original image if it is already smaller (in terms of pixel count)
func (i *Image) ScaleDown(w, h int32) *Image {
	cw, ch, _ := i.Dimensions()
	if int64(w)*int64(h) < int64(cw)*int64(ch) {
		return i.Scale(w, h)
	}
	// No scaling necessary
	return i
}

// FormatString returns the image format as a string, e.g. 'jpg'
func (i Image) FormatString() string {
	return map[C.l_int32]string{
		C.IFF_JFIF_JPEG: "jpg",
		C.IFF_PNG:       "png",
	}[i.pixFormat]
}

// ReaderJPEG returns an io.Reader for the image data, returning a compressed
// JPEG of the specified quality (0-100).
func (i Image) ReaderJPEG(quality int, progressive bool) (*bytes.Buffer, error) {
	if quality < 0 || quality > 100 {
		return nil, fmt.Errorf("quality %d exeeds range 0-100", quality)
	}

	var data *C.l_uint8
	var length C.size_t
	size := int(unsafe.Sizeof(*data))

	q := C.l_int32(quality)
	p := C.l_int32(0)
	if progressive {
		p = C.l_int32(1)
	}

	C.pixWriteMemJpeg(&data, &length, i.cPIX, q, p)
	defer C.free(unsafe.Pointer(data))
	buf := C.GoBytes(unsafe.Pointer(data), C.int(size*int(length)))

	return bytes.NewBuffer(buf), nil
}

// ReaderPNG returns an io.Reader for the image data, in PNG format.
func (i Image) ReaderPNG(gamma float32) (*bytes.Buffer, error) {
	var data *C.l_uint8
	var length C.size_t
	size := int(unsafe.Sizeof(*data))

	g := C.l_float32(gamma)
	C.pixWriteMemPng(&data, &length, i.cPIX, g)
	defer C.free(unsafe.Pointer(data))
	buf := C.GoBytes(unsafe.Pointer(data), C.int(size*int(length)))

	return bytes.NewBuffer(buf), nil
}

// Reader returns an io.Reader for the image data. If format is not specified,
// the reader will produce image data in the original image format. Otherwise,
// `format` must be either "auto", "jpg" or "png"
func (i Image) Reader(format string) (*bytes.Buffer, string, error) {
	pixFormat := i.pixFormat
	if format == "auto" {
		pixFormat = C.IFF_PNG
	}

	switch pixFormat {
	case C.IFF_PNG:
		buf, err := i.ReaderPNG(0.0)
		return buf, "png", err
	case C.IFF_JFIF_JPEG:
		buf, err := i.ReaderJPEG(DefaultJPEGCompression, false)
		return buf, "jpg", err
	default:
		return nil, "", fmt.Errorf("unsupported image format %d", pixFormat)
	}
}
