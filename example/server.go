package main

import (
	"errors"
	"time"

	"github.com/mrmiguu/sock2"
)

type auth struct {
	User string
	Pass string
}

func main() {
	login := make(chan auth)
	err := make(chan error)
	sock2.Add(login)
	sock2.Add(err)
	defer close(login)
	defer close(err)

	auth := <-login
	if len(auth.User) == 0 {
		e := errors.New("empty user")
		err <- e
		panic(e)
	}
	if len(auth.Pass) == 0 {
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
