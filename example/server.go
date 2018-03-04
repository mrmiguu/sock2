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
	println("started!")

	var i, i2 int
	for range [100]int{} {
		i2 = <-paddle
		if i != i2 {
			i = i2 // proves we didn't just read what we wrote
		}
		paddle <- i + 1
	}
	d := time.Since(t)
	println(d.Seconds(), "seconds")
	println("i:", i) // should be 100

	select {}
}