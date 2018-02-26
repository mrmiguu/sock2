package main

import (
	"time"

	"github.com/mrmiguu/sock2"
)

func main() {
	start := make(chan time.Time)
	paddle := make(chan byte)
	sock2.Add(start)
	sock2.Add(paddle)

	t := time.Now()
	start <- t
	println("started!")

	for range [100]int{} {
		paddle <- 0
		<-paddle
	}
	d := time.Since(t)
	println(d.Seconds(), "seconds")

	select {}
}
