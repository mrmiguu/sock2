package main

import (
	"errors"
	"time"

	"github.com/mrmiguu/sock2"
)

func main() {
	login := make(chan []string)
	err := make(chan error)
	sock2.Add(login)
	sock2.Add(err)
	defer close(login)
	defer close(err)

	login <- []string{"user", "pass"}
	if err := <-err; err != nil {
		panic(err)
	}

	t := time.Now()
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

	var e error
	if i != 100 {
		e = errors.New("i is not 100")
	}
	err <- e

	select {}
}
