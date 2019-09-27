package main

import (
	"log"
	"os"
)

func arging() {
	if os.Args[1] == "--client" || os.Args[1] == "-c" {
		log.Println("yes")

	} else if os.Args[1] == "-s" || os.Args[1] == "--send" {

		SendMQ()

	} else {
		log.Println("no")
	}
}
