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

	t := <-start
	println("started!")

	for range [100]int{} {
		<-paddle
		paddle <- 0
	}
	println(time.Since(t).Seconds(), "seconds")

	select {}
}
