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

	auth := <-login
	if len(auth[0]) == 0 {
		e := errors.New("empty user")
		err <- e
		panic(e)
	}
	if len(auth[1]) == 0 {
		e := errors.New("empty pass")
		err <- e
		panic(e)
	}
	err <- nil

	t := time.Now()
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

	println("i:", i) // should be 100
	if e := <-err; e != nil {
		panic("i != 100; " + e.Error())
	}
	println("i == 100")

	select {}
}
