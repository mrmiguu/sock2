package main

import (
	"time"

	"github.com/mrmiguu/sock2"
)

func main() {
	start := make(chan time.Time)
	sock2.Add(start)
	defer close(start)

	t := time.Now()
	start <- t
	println("started!")

	paddle := make(chan int)
	sock2.Add(paddle, "paddle", "1")
	defer close(paddle)

	i := 0
	for range [100]int{} {
		paddle <- i
		i = <-paddle
	}
	d := time.Since(t)
	println(d.Seconds(), "seconds")

	err := make(chan error)
	sock2.Add(err)
	defer close(err)

	println("i:", i) // should be 100
	if e := <-err; e != nil {
		panic("i != 100; " + e.Error())
	}
	println("i == 100")

	select {}
}
