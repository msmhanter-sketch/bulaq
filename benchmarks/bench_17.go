package main

import (
	"fmt"
	"time"
)

func golden_ratio(limit int64) float64 {
    a := 1.0
    b := 1.0
    i := int64(0)
    for i < limit {
        t := b
        b = a + b
        a = t
        i += 1
    }
    return b / a
}

func main() {
    t1 := time.Now()
    res := golden_ratio(50)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
