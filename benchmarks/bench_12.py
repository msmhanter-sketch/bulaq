import time
import sys
sys.setrecursionlimit(200000)

def type_cast():
    c = 0
    i = 0
    while i < 2000000:
        c += int(1.5)
        i += 1
    return c


t1 = time.perf_counter()
res = type_cast()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
