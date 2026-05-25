package main

import (
	"fmt"
	"time"
)

func type_cast() int64 {
    c := int64(0)
    i := int64(0)
    val := 1.5
    for i < 2000000 {
        c += int64(val)
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := type_cast()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
