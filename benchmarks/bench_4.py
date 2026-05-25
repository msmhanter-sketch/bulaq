import time
import sys
sys.setrecursionlimit(200000)

def ackermann(m, n):
    if m == 0:
        return n + 1
    if m > 0 and n == 0:
        return ackermann(m - 1, 1)
    return ackermann(m - 1, ackermann(m, n - 1))


t1 = time.perf_counter()
res = ackermann(3, 7)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
