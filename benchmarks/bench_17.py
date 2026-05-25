import time
import sys
sys.setrecursionlimit(200000)

def golden_ratio(limit):
    a = 1.0
    b = 1.0
    i = 0
    while i < limit:
        t = b
        b = a + b
        a = t
        i += 1
    return b / a


t1 = time.perf_counter()
res = golden_ratio(50)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
