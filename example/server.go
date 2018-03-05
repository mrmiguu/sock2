package main

import (
	"time"

	"github.com/mrmiguu/sock2"
)

func main() {
	start := make(chan time.Time)
	paddle := make(chan int)
	err := make(chan error)

	sock2.Add(start)
	sock2.Add(paddle, "paddle", "1")
	sock2.Add(err)

	defer close(start)
	defer close(paddle)
	defer close(err)

	t := <-start
	println("started!")

	var i int
	for range [100]int{} {
		i = 1 + <-paddle
		paddle <- i
	}
	d := time.Since(t)
	println(d.Seconds(), "seconds")

	println("i:", i) // should be 100
	if e := <-err; e != nil {
		panic("i != 100; " + e.Error())
	}
	println("i == 100")

	select {}
}
