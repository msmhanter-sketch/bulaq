import time
import sys
sys.setrecursionlimit(200000)

def math_float():
    x = 1.0
    i = 0
    while i < 5000000:
        x *= 1.00001
        x /= 1.000001
        i += 1
    return x


t1 = time.perf_counter()
res = math_float()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
