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

func (d *Document) AddPage(imagename string, image Image, words []Word, format string) error {
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
	reader, err := image.Reader(format)
	if err != nil {
		log.Fatalln(err)
	}
	pdf.RegisterImageReader(imagename, format, reader)
	pdf.Image(imagename, 0, 0, w, h, false, format, 0, "")
	pdf.EndLayer()

	return nil
}

func main() {

	tessData := flag.String("tess-data", "/usr/share/tesseract-ocr/tessdata",
		"Tesseract data directory")
	tessLang := flag.String("tess-lang", "eng",
		"Tesseract language")
	docSize := flag.String("size", "a4",
		"document size, e.g. A4 or 210x297mm")
	force := flag.Bool("force", false, "overwrite output file if necessary")
	imgContrast := flag.Float64("contrast", 0.5,
		"automatic contrast amount (0: none, 1: max)")
	imgFormat := flag.String("format", "jpg",
		"format to use when storing images in PDF (jpg|png)")

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

	openFlags := os.O_RDWR | os.O_CREATE
	if *force {
		openFlags |= os.O_TRUNC
	} else {
		openFlags |= os.O_EXCL
	}

	outfile, err := os.OpenFile(outfn, openFlags, 0666)

	if os.IsExist(err) {
		fmt.Printf("File '%s' already exists. Use -force to overwrite.")
		os.Exit(1)
	} else {
		fmt.Printf("Couldn't open '%s': %s", outfn, err)
		os.Exit(1)
	}

	for _, fn := range files {
		img := NewImageFromFile(fn)
		img = img.Adjust(float32(*imgContrast))
		tess.SetImagePix(img.cPIX)
		words := tess.Words()
		doc.AddPage(fn, *img, words, *imgFormat)
	}

	doc.pdf.OutputAndClose(outfile)
}
