package main

import (
	"fmt"
	"time"
)

func harmonic_sum(limit int64) float64 {
    s := 0.0
    i := int64(1)
    for i < limit {
        s += 1.0 / float64(i)
        i += 1
    }
    return s
}

func main() {
    t1 := time.Now()
    res := harmonic_sum(2000000)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
