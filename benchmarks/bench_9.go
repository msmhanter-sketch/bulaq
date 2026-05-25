package main

import (
	"fmt"
	"time"
)

func gcd(a, b int64) int64 {
    for b != 0 {
        div := a / b
        rem := a - (div * b)
        a = b
        b = rem
    }
    return a
}
func gcd_loop() int64 {
    i := int64(0)
    c := int64(0)
    for i < 2000000 {
        c += gcd(12345, i)
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := gcd_loop()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
