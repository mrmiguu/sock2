sock2: Go channel sockets (ver. 2)

# Client
```go
c := make(chan time.Time)
sock2.Add(c)

c <- time.Now()

t := <-c
fmt.Printf("server time sent %v", t)
```
# Server
```go
c := make(chan time.Time)
sock2.Add(c)

t := <-c
fmt.Printf("client time sent %v", t)

c <- time.Now()
```