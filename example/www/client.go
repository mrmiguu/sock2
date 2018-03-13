package main

import (
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

	login <- auth{"user", "pass"}
	if err := <-err; err != nil {
		panic(err)
	}

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
