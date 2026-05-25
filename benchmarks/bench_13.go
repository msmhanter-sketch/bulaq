package main

import (
	"fmt"
	"time"
)

func sum_of_squares(limit int64) int64 {
    c := int64(0)
    i := int64(0)
    for i < limit {
        c += (i * i)
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := sum_of_squares(500000)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
