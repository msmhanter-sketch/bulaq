package main

import (
	"fmt"
	"time"
)

func pi_approx(n int64) float64 {
    s := 0.0
    i := int64(0)
    for i < n {
        div := float64(2*i + 1)
        rem := (i/2)*2 == i
        if rem {
            s += 4.0 / div
        } else {
            s -= 4.0 / div
        }
        i += 1
    }
    return s
}

func main() {
    t1 := time.Now()
    res := pi_approx(1000000)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
