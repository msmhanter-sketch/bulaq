package main

import (
	"fmt"
	"time"
)

func string_compare() int64 {
    i := int64(0)
    c := int64(0)
    s1 := "hello_world_test"
    s2 := "hello_world_test"
    for i < 2000000 {
        if s1 == s2 {
            c += 1
        }
        i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := string_compare()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
