package main

import (
	"fmt"
	"time"
)

func mandelbrot_mock(limit int64) int64 {
    c := int64(0)
    x_i := int64(0)
    for x_i < limit {
        y_i := int64(0)
        for y_i < limit {
            zr := 0.0
            zi := 0.0
            j := int64(0)
            for j < 15 {
                zr2 := zr*zr - zi*zi + float64(x_i)
                zi = 2.0*zr*zi + float64(y_i)
                zr = zr2
                j += 1
            }
            c += 1
            y_i += 1
        }
        x_i += 1
    }
    return c
}

func main() {
    t1 := time.Now()
    res := mandelbrot_mock(100)
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\n", t2.Sub(t1).Seconds())
}
