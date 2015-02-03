package main

/*
#cgo LDFLAGS: -llept
#include "leptonica/allheaders.h"
#include <stdlib.h>
*/
import "C"
import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"unsafe"

	"code.google.com/p/gofpdf"
)

var paperSizes map[string]PageSize

type PageSize struct {
	Width  float64
	Height float64
	Units  string
}

func init() {
	paperSizes = map[string]PageSize{
		"a4":     {210, 297, "mm"},
		"pa4":    {210, 280, "mm"},
		"a5":     {105, 149, "mm"},
		"letter": {216, 279, "mm"},
		"legal":  {216, 356, "mm"},
		"c4":     {229, 324, "mm"},
	}
}

type Options struct {
	Input  string
	Size   string
	Output string
}

func (o Options) InputFilenames() []string {
	return nil
}

// NewImageFromFile creates and returns a new image loaded from the given
// file path.
func NewImageFromFile(filename string) *Image {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	// create new PIX
	cPIX := C.pixRead(cFilename)
	if cPIX == nil {
		log.Fatalln("could not create PIX from given filename")
	}

	return &Image{
		cPIX:      cPIX,
		pixFormat: C.getImpliedFileFormat(cFilename),
	}
}

type Image struct {
	cPIX      *C.PIX
	buf       *bytes.Buffer
	pixFormat C.l_int32
}

func (i *Image) Close() {
	C.pixDestroy(&i.cPIX)
	C.free(unsafe.Pointer(i.cPIX))
	i.cPIX = nil
}

// Adjust improves the clarity and contrast of the image, generally reducing
// scanning artifacts.
func (i *Image) Adjust(threshold float32) *Image {
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
// `format` must be either "jpg" or "png"
func (i Image) Reader(format string) (*bytes.Buffer, error) {
	pixFormat := i.pixFormat
	if format != "" {
		// Determine pix format
		var ok bool
		pixFormat, ok = map[string]C.l_int32{
			"jpg":  C.IFF_JFIF_JPEG,
			"jpeg": C.IFF_JFIF_JPEG,
			"png":  C.IFF_PNG,
		}[strings.ToLower(format)]
		if !ok {
			return nil, fmt.Errorf("Unknown or unsupported format '%s'", format)
		}
	}

	var data *C.l_uint8
	var length C.size_t
	size := int(unsafe.Sizeof(*data))

	C.pixWriteMem(&data, &length, i.cPIX, pixFormat)
	buf := C.GoBytes(unsafe.Pointer(data), C.int(size*int(length)))

	C.free(unsafe.Pointer(data))

	return bytes.NewBuffer(buf), nil
}

func main() {
	infile := "test.jpg"

	path := "/usr/share/tesseract-ocr/tessdata"
	t, err := NewTess(path, "eng")
	if err != nil {
		log.Fatalln(err)
	}

	img := NewImageFromFile(infile)
	img = img.Adjust(0.5)
	w, h, _ := img.Dimensions()
	t.SetImagePix(img.cPIX)

	log.Println("Recognising...")
	words := t.Words()

	log.Println("Creating page...")
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPageFormat("P", gofpdf.SizeType{
		Wd: float64(w) / 10,
		Ht: float64(h) / 10,
	})

	pdf.SetAutoPageBreak(false, 0)

	ocrLayer := pdf.AddLayer("OCR", true)
	pdf.SetFont("Arial", "B", 10)
	pdf.BeginLayer(ocrLayer)
	for _, word := range words {
		width := float64(word.Right-word.Left) / 10
		height := float64(word.Bottom-word.Top) / 10
		pdf.SetXY(float64(word.Left)/10, float64(word.Top)/10)
		pdf.Cell(width, height, word.Text)
	}
	pdf.EndLayer()

	scanLayer := pdf.AddLayer("Scan", true)
	pdf.BeginLayer(scanLayer)
	reader, err := img.Reader("jpg")
	if err != nil {
		log.Fatalln(err)
	}
	pdf.RegisterImageReader("img", "jpg", reader)
	pdf.Image("img", 0, 0, float64(w)/10, float64(h)/10, false, "jpg", 0, "")
	pdf.EndLayer()

	log.Println("Saving...")
	outfile, _ := os.Create("test.pdf")
	pdf.OutputAndClose(outfile)
}
