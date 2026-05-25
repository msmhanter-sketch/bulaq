package main

import (
	"fmt"
	"time"
)

func ackermann(m, n int64) int64 {
    if m == 0 {
        return n + 1
    }
    if m > 0 && n == 0 {
        return ackermann(m-1, 1)
    }
    return ackermann(m-1, ackermann(m, n-1))
}

func main() {
    t1 := time.Now()
    res := ackermann(3, 7)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
