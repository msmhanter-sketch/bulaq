package main

import (
	"fmt"
	"time"
)

func power_loop(limit int64) float64 {
    s := 0.0
    i := int64(0)
    for i < limit {
        p := 1.0
        j := int64(0)
        for j < 10 {
            p *= 1.1
            j += 1
        }
        s += p
        i += 1
    }
    return s
}

func main() {
    t1 := time.Now()
    res := power_loop(5000)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
