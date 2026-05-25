package main

import (
	"fmt"
	"time"
)

func boolean_logic() int64 {
    i := int64(0)
    c := int64(0)
    for i < 2000000 {
        b1 := (i / 2) == 0
        b2 := (i / 3) == 0
        if b1 && !b2 {
            c += 1
        }
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := boolean_logic()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
