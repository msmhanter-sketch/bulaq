import time
import sys
sys.setrecursionlimit(200000)

def bitwise_ops():
    c = 0
    i = 1
    while i < 2000000:
        c += (i << 1)
        c += (i >> 1)
        i += 1
    return c


t1 = time.perf_counter()
res = bitwise_ops()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
