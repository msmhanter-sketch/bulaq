package main

import (
	"fmt"
	"time"
)

func bitwise_ops() int64 {
    c := int64(0)
    i := int64(1)
    for i < 2000000 {
        c += (i << 1)
        c += (i >> 1)
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := bitwise_ops()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
