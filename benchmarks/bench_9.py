import time
import sys
sys.setrecursionlimit(200000)

def gcd(a, b):
    while b != 0:
        div = a // b
        rem = a - (div * b)
        a = b
        b = rem
    return a

def gcd_loop():
    i = 0
    c = 0
    while i < 2000000:
        c += gcd(12345, i)
        i += 1
    return c


t1 = time.perf_counter()
res = gcd_loop()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
