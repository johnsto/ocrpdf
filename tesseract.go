package main

// #cgo LDFLAGS: -L /usr/local/lib -ltesseract
// #include "tesseract/capi.h"
// #include <stdlib.h>
import "C"
import (
	"errors"
	"runtime"
	"unsafe"
)

type Tess struct {
	api *C.TessBaseAPI
}

type Word struct {
	Text   string
	Left   int
	Right  int
	Top    int
	Bottom int
}

func NewTess(datapath string, language string) (*Tess, error) {
	api := C.TessBaseAPICreate()

	cDatapath := C.CString(datapath)
	defer C.free(unsafe.Pointer(cDatapath))

	cLanguage := C.CString(language)
	defer C.free(unsafe.Pointer(cLanguage))

	res := C.TessBaseAPIInit3(api, cDatapath, cLanguage)
	if res != 0 {
		return nil, errors.New("could not initiate new Tess instance")
	}

	tess := &Tess{
		api: api,
	}

	runtime.SetFinalizer(tess, (*Tess).delete)

	return tess, nil
}

func (t *Tess) delete() {
	if t.api != nil {
		C.TessBaseAPIEnd(t.api)
		C.TessBaseAPIDelete(t.api)
	}
}

func (t *Tess) SetImagePix(pix *C.struct_Pix) {
	C.TessBaseAPISetImage2(t.api, pix)
}

func (t *Tess) Words() []Word {
	var words []Word

	C.TessBaseAPIRecognize(t.api, nil)
	ri := C.TessBaseAPIGetIterator(t.api)
	defer C.TessResultIteratorDelete(ri)

	pi := C.TessResultIteratorGetPageIterator(ri)

	if ri != nil {
		for {
			cWord := C.TessResultIteratorGetUTF8Text(ri, C.RIL_WORD)
			var cLeft, cTop, cRight, cBottom C.int
			C.TessPageIteratorBoundingBox(pi, C.RIL_WORD,
				&cLeft, &cTop, &cRight, &cBottom)
			word := Word{
				Text:   C.GoString(cWord),
				Left:   int(cLeft),
				Right:  int(cRight),
				Top:    int(cTop),
				Bottom: int(cBottom),
			}
			words = append(words, word)
			if C.TessPageIteratorNext(pi, C.RIL_WORD) == C.int(0) {
				break
			}
		}
	}

	return words
}
