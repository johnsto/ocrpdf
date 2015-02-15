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

// FormatString returns the image format as a string, e.g. 'jpg'
func (i Image) FormatString() string {
	return map[C.l_int32]string{
		C.IFF_JFIF_JPEG: "jpg",
		C.IFF_PNG:       "png",
	}[i.pixFormat]
}

// Reader returns an io.Reader for the image data. If format is not specified,
// the reader will produce image data in the original image format. Otherwise,
// `format` must be either "auto", "jpg" or "png"
func (i Image) Reader(format string) (*bytes.Buffer, string, error) {
	pixFormat := i.pixFormat
	switch format {
	case "png":
		pixFormat = C.IFF_PNG
	case "jpg":
		pixFormat = C.IFF_JFIF_JPEG
	case "auto":
		// Determine pix format
		if pixFormat == C.IFF_PNG {
			format = "png"
		} else if pixFormat == C.IFF_JFIF_JPEG {
			format = "jpg"
		} else {
			// Choose a better format... for now we'll always use PNG
			pixFormat = C.IFF_PNG
			format = "png"
		}
	}

	var data *C.l_uint8
	var length C.size_t
	size := int(unsafe.Sizeof(*data))

	C.pixWriteMem(&data, &length, i.cPIX, pixFormat)
	defer C.free(unsafe.Pointer(data))
	buf := C.GoBytes(unsafe.Pointer(data), C.int(size*int(length)))

	return bytes.NewBuffer(buf), format, nil
}
