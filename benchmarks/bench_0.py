import time
import sys
sys.setrecursionlimit(200000)

def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)


t1 = time.perf_counter()
res = fib(35)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
