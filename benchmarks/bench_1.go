package main

import (
	"fmt"
	"time"
)

func loop_10m() int64 {
    x := int64(0)
    for x < 10000000 {
        x += 1
    }
    return x
}

func main() {
    t1 := time.Now()
    res := loop_10m()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
