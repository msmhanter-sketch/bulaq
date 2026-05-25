import time
import sys
sys.setrecursionlimit(200000)

def collatz(limit):
    total = 0
    i = 1
    while i < limit:
        n = i
        while n != 1:
            div = n // 2
            rem = n - (div * 2)
            if rem == 0:
                n = div
            else:
                n = n * 3 + 1
            total += 1
        i += 1
    return total


t1 = time.perf_counter()
res = collatz(50000)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
