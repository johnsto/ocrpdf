package main

import (
	"fmt"
	"os"
)

func logv(a ...interface{}) {
	if verbose {
		fmt.Println(a...)
	}
}
func logvf(format string, a ...interface{}) {
	if verbose {
		fmt.Printf(format, a...)
	}
}

func logd(a ...interface{}) {
	if debug {
		fmt.Println(a...)
	}
}

func logdf(format string, a ...interface{}) {
	if debug {
		fmt.Printf(format, a...)
	}
}

func loge(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
}

func logef(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}
