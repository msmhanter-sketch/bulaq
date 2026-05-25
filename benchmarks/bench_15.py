import time
import sys
sys.setrecursionlimit(200000)

def is_prime(n):
    if n < 2:
        return 0
    i = 2
    while i * i <= n:
        div = n // i
        rem = n - (div * i)
        if rem == 0:
            return 0
        i += 1
    return 1

def prime_count(limit):
    c = 0
    i = 2
    while i < limit:
        if is_prime(i) == 1:
            c += 1
        i += 1
    return c


t1 = time.perf_counter()
res = prime_count(10000)
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
