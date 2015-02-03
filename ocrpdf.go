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
	pdf         *gofpdf.Fpdf
	ocrLayerId  int
	scanLayerId int
}

func NewDocument(size string) *Document {
	pdf := gofpdf.New("P", "mm", size, "")
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetFont("Arial", "B", 10)
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

	if iw*h > ih*w {
		w = h * iw / ih
		mx = w / iw
	} else {
		h = w * ih / iw
		my = h / ih
	}

	pdf.BeginLayer(d.ocrLayerId)
	for _, word := range words {
		width := float64(word.Right-word.Left) * mx
		height := float64(word.Bottom-word.Top) * my
		pdf.SetXY(float64(word.Left)*mx, float64(word.Top)*my)
		pdf.Cell(width, height, word.Text)
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

	log.Println("Recognising...")
	words := tess.Words()

	doc := NewDocument("A4")

	doc.AddPage(filename, *img, words)
}

func main() {
	path := "/usr/share/tesseract-ocr/tessdata"
	t, err := NewTess(path, "eng")
	if err != nil {
		log.Fatalln(err)
	}

	doc := NewDocument("A4")
	doc.AddPageFromFile(t, "test_page1.jpg")
	//doc.AddPageFromFile(t, "test_page2.jpg")

	log.Println("Saving...")
	outfile, _ := os.Create("test.pdf")
	doc.pdf.OutputAndClose(outfile)
}
