import time
import sys
sys.setrecursionlimit(200000)

def mandelbrot_mock(limit):
    c = 0
    x_i = 0
    while x_i < limit:
        y_i = 0
        while y_i < limit:
            zr = 0.0
            zi = 0.0
            j = 0
            while j < 15:
                zr2 = zr * zr - zi * zi + x_i
                zi = 2.0 * zr * zi + y_i
                zr = zr2
                j += 1
            c += 1
            y_i += 1
        x_i += 1
    return c


t1 = time.perf_counter()
res = mandelbrot_mock(100)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
