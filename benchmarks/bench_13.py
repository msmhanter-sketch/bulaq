import time
import sys
sys.setrecursionlimit(200000)

def sum_of_squares(limit):
    c = 0
    i = 0
    while i < limit:
        c += (i * i)
        i += 1
    return c


t1 = time.perf_counter()
res = sum_of_squares(500000)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
