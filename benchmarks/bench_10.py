import time
import sys
sys.setrecursionlimit(200000)

def factorial_loop():
    c = 0
    i = 0
    while i < 1000000:
        f = 1
        j = 1
        while j < 10:
            f *= j
            j += 1
        c += f
        i += 1
    return c


t1 = time.perf_counter()
res = factorial_loop()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
