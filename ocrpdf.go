package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"code.google.com/p/gofpdf"
)

type Document struct {
	pdf         *gofpdf.Fpdf
	ocrLayerId  int
	scanLayerId int
}

func NewDocument(size string) *Document {
	pdf := gofpdf.New("P", "mm", size, "")
	pdf.SetAutoPageBreak(false, 0)
	ocrLayerId := pdf.AddLayer("OCR", true)
	scanLayerId := pdf.AddLayer("Scan", true)
	return &Document{
		pdf:         pdf,
		ocrLayerId:  ocrLayerId,
		scanLayerId: scanLayerId,
	}
}

func (d *Document) AddPage(imagename string, image Image, words []Word) error {
	pdf := d.pdf

	pdf.AddPage()

	imageWidth, imageHeight, _ := image.Dimensions()
	iw, ih := float64(imageWidth), float64(imageHeight)
	w, h := pdf.GetPageSize()
	mx, my := 1.0, 1.0

	if iw*h < ih*w {
		w = h * iw / ih
	} else {
		h = w * ih / iw
	}
	mx = w / iw
	my = h / ih

	pdf.SetFont("Arial", "B", 10)
	pdf.Write(8, "This line belongs to layer 1.\n")

	pdf.BeginLayer(d.ocrLayerId)
	pdf.SetFont("Arial", "B", 10)
	for _, word := range words {
		ww := float64(word.Right-word.Left) * mx
		wh := float64(word.Bottom-word.Top) * my
		_, _ = ww, wh
		pdf.SetXY(float64(word.Left)*mx, float64(word.Top)*my)
		pdf.Cell(ww, wh, word.Text)
	}
	pdf.EndLayer()

	pdf.BeginLayer(d.scanLayerId)
	reader, err := image.Reader("jpg")
	if err != nil {
		log.Fatalln(err)
	}
	pdf.RegisterImageReader(imagename, "jpg", reader)
	pdf.Image(imagename, 0, 0, w, h, false, "jpg", 0, "")
	pdf.EndLayer()

	return nil
}

func (d *Document) AddPageFromFile(tess *Tess, filename string) {
	img := NewImageFromFile(filename)
	img = img.Adjust(0.9)
	tess.SetImagePix(img.cPIX)

	words := tess.Words()

	d.AddPage(filename, *img, words)
}

func main() {

	tessData := flag.String("tess-data", "/usr/share/tesseract-ocr/tessdata",
		"Tesseract data directory")
	tessLang := flag.String("tess-lang", "eng", "Tesseract language")
	docSize := flag.String("size", "a4", "document size, e.g. A4 or 210x297mm")

	flag.Parse()

	tess, err := NewTess(*tessData, *tessLang)
	if err != nil {
		fmt.Printf("Could not initialise Tesseract: %s\n", err)
		os.Exit(1)
	}

	doc := NewDocument(*docSize)

	files := flag.Args()

	if len(files) == 0 {
		fmt.Printf("No file(s) specified!\n")
		flag.Usage()
		os.Exit(1)
	}

	outfn := files[0]
	if len(files) == 1 {
		ext := filepath.Ext(outfn)
		outfn = strings.TrimRight(outfn, ext) + ".pdf"
	}

	for _, fn := range files {
		doc.AddPageFromFile(tess, fn)
	}

	outfile, _ := os.Create(outfn)
	doc.pdf.OutputAndClose(outfile)

}
