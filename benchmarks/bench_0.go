package main

import (
	"fmt"
	"time"
)

func fib(n int64) int64 {
    if n < 2 {
        return n
    }
    return fib(n-1) + fib(n-2)
}

func main() {
    t1 := time.Now()
    res := fib(35)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
