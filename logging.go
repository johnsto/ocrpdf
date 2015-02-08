package main

import "fmt"

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
