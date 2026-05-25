package main

import (
	"fmt"
	"time"
)

func collatz(limit int64) int64 {
    total := int64(0)
    i := int64(1)
    for i < limit {
        n := i
        for n != 1 {
            div := n / 2
            rem := n - (div * 2)
            if rem == 0 {
                n = div
            } else {
                n = n*3 + 1
            }
            total += 1
        }
        i += 1
    }
    return total
}

func main() {
    t1 := time.Now()
    res := collatz(50000)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
