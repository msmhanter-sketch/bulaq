import time
import sys
sys.setrecursionlimit(200000)

def loop_10m():
    x = 0
    while x < 10000000:
        x += 1
    return x


t1 = time.perf_counter()
res = loop_10m()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
