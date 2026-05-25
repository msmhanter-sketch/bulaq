package main

import (
	"fmt"
	"time"
)

func math_float() float64 {
    x := 1.0
    i := int64(0)
    for i < 5000000 {
        x *= 1.00001
        x /= 1.000001
        i += 1
    }
    return x
}

func main() {
    t1 := time.Now()
    res := math_float()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
