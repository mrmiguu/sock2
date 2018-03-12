package main

import (
	"errors"
	"time"

	"github.com/mrmiguu/sock2"
)

func main() {
	start := make(chan time.Time)
	sock2.Add(start)
	defer close(start)

	t := <-start
	println("started!")

	paddle := make(chan int)
	sock2.Add(paddle, "paddle", "1")
	defer close(paddle)

	var i int
	for range [100]int{} {
		i = 1 + <-paddle
		paddle <- i
	}
	d := time.Since(t)
	println(d.Seconds(), "seconds")

	err := make(chan error)
	sock2.Add(err)
	defer close(err)

	var e error
	if i != 100 {
		e = errors.New("i is not 100")
	}
	err <- e

	select {}
}
