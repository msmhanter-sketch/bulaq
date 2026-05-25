import time
import sys
sys.setrecursionlimit(200000)

def harmonic_sum(limit):
    s = 0.0
    i = 1
    while i < limit:
        s += 1.0 / i
        i += 1
    return s


t1 = time.perf_counter()
res = harmonic_sum(2000000)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
