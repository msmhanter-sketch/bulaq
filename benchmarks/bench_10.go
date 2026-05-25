package main

import (
	"fmt"
	"time"
)

func factorial_loop() int64 {
    c := int64(0)
    i := int64(0)
    for i < 1000000 {
        f := int64(1)
        j := int64(1)
        for j < 10 {
            f *= j
            j += 1
        }
        c += f
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := factorial_loop()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
