# ocrpdf

*A tool to convert scanned documents into searchable PDFs*

`ocrpdf` recognises and extracts text from scanned documents and combines them to produce (generally) searchable PDFs that look like the original document. It uses the open source [Tesseract](https://tesseract-ocr.googlecode.com) library for OCR, [Leptonica](http://leptonica.com) for image manipiulation, and [gofpdf](code.google.com/p/gofpdf) for document generation.

## I demand a GUI!

You're probably better off with [`gscan2pdf`](http://gscan2pdf.sourceforge.net/), which was the original inspiration for this tool (obligatory Sourceforge warning!)

## Installation

This software requires that you have the Tesseract 3.03.03 library and data files installed.

On Debian systems, use `apt-get install libtesseract3 tesseract-ocr` to install the library and all language data files.

On Windows, erm...

## Usage

Converting a scanned image is as simple as:

`ocrpdf scan.jpg`

By default, `ocrpdf` will take the filename name of the first input scan as the output document name, in this case, `scan.pdf`.

You can also specify a document size, output filename, title, enable compression and specify multiple pages:

`ocrpdf -s letter -o taxes.pdf -t "2015 Taxes" --compress taxes1.jpg taxes2.jpg taxes3.jpg`

Automatic contrast enhancement to improve the legibility of the text is performed by default, you can disable this with the `--contrast=0` flag.

## PDF Structure

Pages in the output PDF contain two layers, one with the recognised text, and one with the scanned image. The image is positioned and arranged on top of the text.

In PDF viewers like `evince`, this arrangement lets you search and select text as if it were an invisible layer on top of the image.

