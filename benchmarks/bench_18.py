import time
import sys
sys.setrecursionlimit(200000)

def power_loop(limit):
    s = 0.0
    i = 0
    while i < limit:
        p = 1.0
        j = 0
        while j < 10:
            p *= 1.1
            j += 1
        s += p
        i += 1
    return s


t1 = time.perf_counter()
res = power_loop(5000)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
