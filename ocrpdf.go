package main

/*
#cgo LDFLAGS: -llept
#include "leptonica/allheaders.h"
#include <stdlib.h>
*/
import "C"
import (
	"bytes"
	"log"
	"os"
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

func NewImageFromFile(filename string) *Image {
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	// create new PIX
	cPIX := C.pixRead(cFilename)
	if cPIX == nil {
		log.Fatalln("could not create PIX from given filename")
	}

	return &Image{
		cPIX: cPIX,
	}
}

type Image struct {
	cPIX *C.PIX
	buf  *bytes.Buffer
}

// Adjust improves the clarity and contrast of the image, generally reducing
// scanning artifacts.
func (i *Image) Adjust(threshold float32) Image {
	result := C.pixContrastTRC(i.cPIX, i.cPIX, C.l_float32(threshold))
	return Image{
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

func (i *Image) Read(p []byte) (n int, err error) {
	if i.buf == nil {
		log.Println("Creating image...")
		var data *C.l_uint8
		sz := C.size_t(0)
		C.pixWriteMem(&data, &sz, i.cPIX, C.IFF_PNG)
		buf := C.GoBytes(unsafe.Pointer(&data), C.int(sz))
		i.buf = bytes.NewBuffer(buf)
		log.Println("Created!")
	}
	return i.buf.Read(p)
}

func (i Image) Reader() *bytes.Buffer {
	log.Println("Creating image...")
	var data *C.l_uint8
	sz := C.size_t(0)
	C.pixWriteMem(&data, &sz, i.cPIX, C.IFF_PNG)
	buf := C.GoBytes(unsafe.Pointer(&data), C.int(sz))
	log.Println(len(buf))
	log.Println(buf[0:10])
	return bytes.NewBuffer(buf)
}

func main() {
	infile := "test.jpg"

	path := "/usr/share/tesseract-ocr/tessdata"
	t, err := NewTess(path, "eng")
	if err != nil {
		log.Fatalln(err)
	}

	img := NewImageFromFile(infile)
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
	pdf.RegisterImageReader("img", "png", img.Reader())
	//pdf.Image("img", 0, 0, float64(w)/10, float64(h)/10, false, "png", 0, "")
	pdf.EndLayer()

	log.Println("Saving...")
	outfile, _ := os.Create("test.pdf")
	pdf.OutputAndClose(outfile)
}
