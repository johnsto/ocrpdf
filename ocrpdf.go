package main

/*
#cgo LDFLAGS: -llept
#include "leptonica/allheaders.h"
#include <stdlib.h>
*/
import "C"
import (
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

func main() {
	infile := "test.jpg"

	path := "/usr/share/tesseract-ocr/tessdata"
	t, err := NewTess(path, "eng")
	if err != nil {
		log.Fatalln(err)
	}

	cFilename := C.CString(infile)
	defer C.free(unsafe.Pointer(cFilename))

	// create new PIX
	cPIX := C.pixRead(cFilename)
	if cPIX == nil {
		log.Fatalln("could not create PIX from given filename")
	}

	grey := C.pixConvertRGBToGrayFast(cPIX)
	//tophat := C.pixTophat(grey, C.l_int32(15), C.l_int32(15), C.L_TOPHAT_BLACK)
	result := C.pixContrastTRC(cPIX, cPIX, C.l_float32(0.5))
	C.pixWritePng(C.CString("test.png"), result, C.l_float32(2.2))

	t.SetImagePix(grey)

	words := t.Words()

	var imgWidth, imgHeight, imgDepth int32
	cW := C.l_int32(imgWidth)
	cH := C.l_int32(imgHeight)
	cD := C.l_int32(imgDepth)
	C.pixGetDimensions(cPIX, &cW, &cH, &cD)
	imgWidth = int32(cW)
	imgHeight = int32(cH)

	log.Println(imgWidth, imgHeight)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPageFormat("P", gofpdf.SizeType{
		Wd: float64(imgWidth) / 10,
		Ht: float64(imgHeight) / 10,
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
	pdf.Image(infile, 0, 0, float64(imgWidth)/10, float64(imgHeight)/10, false, "", 0, "")
	pdf.EndLayer()

	outfile, _ := os.Create("test.pdf")
	pdf.OutputAndClose(outfile)
}
