package main

import (
	"log"
	"os"

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

type Document struct {
	pdf *gofpdf.Fpdf
}

func NewDocument() *Document {
	pdf := gofpdf.New("P", "mm", "A4", "")
	return &Document{
		pdf: pdf,
	}
}

func (d *Document) AddPage(i Image, size string, fit string) error {
	return nil
}

func main() {
	infile := "test.jpg"

	path := "/usr/share/tesseract-ocr/tessdata"
	t, err := NewTess(path, "eng")
	if err != nil {
		log.Fatalln(err)
	}

	img := NewImageFromFile(infile)
	img = img.Adjust(0.9)
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
