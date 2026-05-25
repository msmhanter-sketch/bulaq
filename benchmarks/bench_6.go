package main

import (
	"fmt"
	"time"
)

func string_concat() int64 {
    s := ""
    i := int64(0)
    for i < 2000 {
        s += "a"
        i += 1
    }
    return int64(len(s))
}

func main() {
    t1 := time.Now()
    res := string_concat()
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
