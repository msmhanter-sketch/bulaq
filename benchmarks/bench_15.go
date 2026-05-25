package main

import (
	"fmt"
	"time"
)

func is_prime(n int64) int64 {
    if n < 2 {
        return 0
    }
    i := int64(2)
    for i*i <= n {
        div := n / i
        rem := n - (div * i)
        if rem == 0 {
            return 0
        }
        i += 1
    }
    return 1
}
func prime_count(limit int64) int64 {
    c := int64(0)
    i := int64(2)
    for i < limit {
        if is_prime(i) == 1 {
            c += 1
        }
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := prime_count(10000)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
