import time
import sys
sys.setrecursionlimit(200000)

def pi_approx(n):
    s = 0.0
    i = 0
    while i < n:
        div = float(2 * i + 1)
        rem = (i // 2) * 2 == i
        if rem:
            s += 4.0 / div
        else:
            s -= 4.0 / div
        i += 1
    return s


t1 = time.perf_counter()
res = pi_approx(1000000)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
