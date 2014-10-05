// +build debug

package main

import "log"

func debug(v ...interface{}) {
	log.Println(v...)
}
