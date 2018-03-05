package main

import (
	"time"

	"github.com/mrmiguu/sock2"
)

func main() {
	start := make(chan time.Time)
	paddle := make(chan int)
	sock2.Add(start)
	sock2.Add(paddle, "paddle", "1")

	t := time.Now()
	start <- t
	close(start)
	println("started!")

	i := 0
	for range [100]int{} {
		paddle <- i
		i = <-paddle
	}
	d := time.Since(t)
	println(d.Seconds(), "seconds")
	println("i:", i) // should be 100

	select {}
}
