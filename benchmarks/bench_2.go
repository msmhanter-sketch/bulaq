package main

import (
	"fmt"
	"time"
)

func nested_loop() int64 {
    c := int64(0)
    i := int64(0)
    for i < 1000 {
        j := int64(0)
        for j < 1000 {
            c += 1
            j += 1
        }
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := nested_loop()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
