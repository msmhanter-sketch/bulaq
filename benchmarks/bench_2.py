import time
import sys
sys.setrecursionlimit(200000)

def nested_loop():
    c = 0
    i = 0
    while i < 1000:
        j = 0
        while j < 1000:
            c += 1
            j += 1
        i += 1
    return c


t1 = time.perf_counter()
res = nested_loop()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
