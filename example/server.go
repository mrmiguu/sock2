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

	t := <-start
	close(start)
	println("started!")

	var i int
	for range [100]int{} {
		i = 1 + <-paddle
		paddle <- i
	}
	d := time.Since(t)
	println(d.Seconds(), "seconds")
	println("i:", i) // should be 100

	select {}
}
